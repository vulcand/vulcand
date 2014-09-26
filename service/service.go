package service

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/go-etcd/etcd"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/manners"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/scroll"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/metrics"
	"github.com/mailgun/vulcand/api"
	"github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/backend/etcdbackend"
	"github.com/mailgun/vulcand/plugin"
	"github.com/mailgun/vulcand/secret"
	"github.com/mailgun/vulcand/server"
	"github.com/mailgun/vulcand/supervisor"
)

func Run(registry *plugin.Registry) error {
	options, err := ParseCommandLine()
	if err != nil {
		return fmt.Errorf("failed to parse command line: %s", err)
	}
	service := NewService(options, registry)
	if err := service.Start(); err != nil {
		return fmt.Errorf("service start failure: %s", err)
	} else {
		log.Infof("Service exited gracefully")
	}
	return nil
}

type Service struct {
	client        *etcd.Client
	options       Options
	registry      *plugin.Registry
	apiApp        *scroll.App
	errorC        chan error
	sigC          chan os.Signal
	supervisor    *supervisor.Supervisor
	metricsClient metrics.Client
	apiServer     *manners.GracefulServer
}

func NewService(options Options, registry *plugin.Registry) *Service {
	return &Service{
		registry: registry,
		options:  options,
		errorC:   make(chan error),
		// Channel receiving signals has to be non blocking, otherwise the service can miss a signal.
		sigC: make(chan os.Signal, 1024),
	}
}

func (s *Service) Start() error {
	log.Init([]*log.LogConfig{&log.LogConfig{Name: s.options.Log}})

	if s.options.PidPath != "" {
		ioutil.WriteFile(s.options.PidPath, []byte(fmt.Sprint(os.Getpid())), 0644)
	}

	if s.options.StatsdAddr != "" {
		var err error
		s.metricsClient, err = metrics.NewStatsd(s.options.StatsdAddr, s.options.StatsdPrefix)
		if err != nil {
			return err
		}
	}

	apiFile, muxFiles, err := s.getFiles()
	if err != nil {
		return err
	}

	s.supervisor = supervisor.NewSupervisorWithOptions(
		s.newServer, s.newBackend, s.errorC, supervisor.Options{Files: muxFiles})

	// Tells configurator to perform initial proxy configuration and start watching changes
	if err := s.supervisor.Start(); err != nil {
		return err
	}

	if err := s.initApi(); err != nil {
		return err
	}

	go func() {
		s.errorC <- s.startApi(apiFile)
	}()

	if s.metricsClient != nil {
		go s.reportSystemMetrics()
	}
	signal.Notify(s.sigC, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGUSR2)

	// Block until a signal is received or we got an error
	for {
		select {
		case signal := <-s.sigC:
			switch signal {
			case syscall.SIGTERM, syscall.SIGINT:
				log.Infof("Got signal '%s', shutting down gracefully", signal)
				s.supervisor.Stop(true)
				log.Infof("All servers stopped")
				return nil
			case syscall.SIGKILL:
				log.Infof("Got signal '%s', exiting now without waiting", signal)
				s.supervisor.Stop(false)
				return nil
			case syscall.SIGUSR2:
				log.Infof("Got signal '%s', forking a new self", signal)
				if err := s.startChild(); err != nil {
					log.Infof("Failed to start self: %s", err)
				} else {
					log.Infof("Successfully started self")
				}
			default:
				log.Infof("Ignoring '%s'", signal)
			}
		case err := <-s.errorC:
			log.Infof("Got request to shutdown with error: %s", err)
			return err
		}
	}

	return nil
}

func (s *Service) getFiles() (*server.FileDescriptor, []*server.FileDescriptor, error) {
	// These files may be passed in by the parent process
	filesString := os.Getenv(vulcandFilesKey)
	if filesString == "" {
		return nil, nil, nil
	}

	files, err := filesFromString(filesString)
	if err != nil {
		return nil, nil, fmt.Errorf("child failed to start: failed to read files from string, error %s", err)
	}

	if len(files) != 0 {
		log.Infof("I am a child that has been passed files: %s", files)
	}

	return s.splitFiles(files)
}

func (s *Service) splitFiles(files []*server.FileDescriptor) (*server.FileDescriptor, []*server.FileDescriptor, error) {
	apiAddr := fmt.Sprintf("%s:%d", s.options.ApiInterface, s.options.ApiPort)
	for i, f := range files {
		if f.Address.Address == apiAddr {
			return files[i], append(files[:i], files[i+1:]...), nil
		}
	}
	return nil, nil, fmt.Errorf("API address %s not found in %s", files)
}

func (s *Service) startChild() error {
	log.Infof("Starting child")
	path, err := execPath()
	if err != nil {
		return err
	}

	wd, err := os.Getwd()
	if nil != err {
		return err
	}

	// Get socket files currently in use by the underlying http server controlled by supervisor
	extraFiles, err := s.supervisor.GetFiles()
	if err != nil {
		return err
	}

	apiFile, err := s.GetAPIFile()
	if err != nil {
		return err
	}

	extraFiles = append(extraFiles, apiFile)

	// These files will be passed to the child process
	files := []*os.File{os.Stdin, os.Stdout, os.Stderr}
	for _, f := range extraFiles {
		files = append(files, f.File)
	}

	// Serialize files to JSON string representation
	vals, err := filesToString(extraFiles)
	if err != nil {
		return err
	}

	log.Infof("Passing %s to child", vals)
	os.Setenv(vulcandFilesKey, vals)

	p, err := os.StartProcess(path, os.Args, &os.ProcAttr{
		Dir:   wd,
		Env:   os.Environ(),
		Files: files,
		Sys:   &syscall.SysProcAttr{},
	})

	if err != nil {
		return err
	}

	log.Infof("Started new child pid=%d binary=%s", p.Pid, path)
	return nil
}

func (s *Service) GetAPIFile() (*server.FileDescriptor, error) {
	file, err := s.apiServer.GetFile()
	if err != nil {
		return nil, err
	}
	a := backend.Address{
		Network: "tcp",
		Address: fmt.Sprintf("%s:%d", s.options.ApiInterface, s.options.ApiPort),
	}
	return &server.FileDescriptor{File: file, Address: a}, nil
}

func (s *Service) newBox() (*secret.Box, error) {
	if s.options.SealKey == "" {
		return nil, nil
	}
	key, err := secret.KeyFromString(s.options.SealKey)
	if err != nil {
		return nil, err
	}
	return secret.NewBox(key)
}

func (s *Service) newBackend() (backend.Backend, error) {
	box, err := s.newBox()
	if err != nil {
		return nil, err
	}
	return etcdbackend.NewEtcdBackendWithOptions(
		s.registry, s.options.EtcdNodes, s.options.EtcdKey,
		etcdbackend.Options{
			EtcdConsistency: s.options.EtcdConsistency,
			Box:             box,
		})
}

func (s *Service) reportSystemMetrics() {
	defer func() {
		if r := recover(); r != nil {
			log.Infof("Recovered in reportSystemMetrics", r)
		}
	}()
	for {
		s.metricsClient.ReportRuntimeMetrics("sys", 1.0)
		// we have 256 time buckets for gc stats, GC is being executed every 4ms on average
		// so we have 256 * 4 = 1024 around one second to report it. To play safe, let's report every 300ms
		time.Sleep(300 * time.Millisecond)
	}
}

func (s *Service) newServer(id int) (server.Server, error) {
	return server.NewMuxServerWithOptions(id, server.Options{
		MetricsClient:  s.metricsClient,
		DialTimeout:    s.options.EndpointDialTimeout,
		ReadTimeout:    s.options.ServerReadTimeout,
		WriteTimeout:   s.options.ServerWriteTimeout,
		MaxHeaderBytes: s.options.ServerMaxHeaderBytes,
		DefaultListener: &backend.Listener{
			Id:       "DefaultListener",
			Protocol: "http",
			Address: backend.Address{
				Network: "tcp",
				Address: fmt.Sprintf("%s:%d", s.options.Interface, s.options.Port),
			},
		},
	})
}

func (s *Service) initApi() error {
	s.apiApp = scroll.NewApp()
	b, err := s.newBackend()
	if err != nil {
		return err
	}
	api.InitProxyController(b, s.supervisor, s.apiApp)
	return nil
}

func (s *Service) startApi(file *server.FileDescriptor) error {
	addr := fmt.Sprintf("%s:%d", s.options.ApiInterface, s.options.ApiPort)

	server := &http.Server{
		Addr:           addr,
		Handler:        s.apiApp.GetHandler(),
		ReadTimeout:    s.options.ServerReadTimeout,
		WriteTimeout:   s.options.ServerWriteTimeout,
		MaxHeaderBytes: 1 << 20,
	}

	var listener net.Listener
	if file != nil {
		var err error
		listener, err = file.ToListener()
		if err != nil {
			return err
		}
	}

	s.apiServer = manners.NewWithOptions(manners.Options{Server: server, Listener: listener})
	return s.apiServer.ListenAndServe()
}

func execPath() (string, error) {
	name, err := exec.LookPath(os.Args[0])
	if err != nil {
		return "", err
	}
	if _, err = os.Stat(name); nil != err {
		return "", err
	}
	return name, err
}

type fileDescriptor struct {
	Address  backend.Address
	FileFD   int
	FileName string
}

// filesToString serializes file descriptors as well as accompanying information (like socket host and port)
func filesToString(files []*server.FileDescriptor) (string, error) {
	out := make([]fileDescriptor, len(files))
	for i, f := range files {
		out[i] = fileDescriptor{
			// Once files will be passed to the child process and their FDs will change.
			// The first three passed files are stdin, stdout and stderr, every next file will have the index + 3
			// That's why we rearrange the FDs for child processes to get the correct file descriptors.
			FileFD:   i + 3,
			FileName: f.File.Name(),
			Address:  f.Address,
		}
	}
	bytes, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// filesFromString de-serializes the file descriptors and turns them in the os.Files
func filesFromString(in string) ([]*server.FileDescriptor, error) {
	var out []fileDescriptor
	if err := json.Unmarshal([]byte(in), &out); err != nil {
		return nil, err
	}
	files := make([]*server.FileDescriptor, len(out))
	for i, o := range out {
		files[i] = &server.FileDescriptor{
			File:    os.NewFile(uintptr(o.FileFD), o.FileName),
			Address: o.Address,
		}
	}
	return files, nil
}

const vulcandFilesKey = "VULCAND_FILES_KEY"

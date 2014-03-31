package service

import (
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	log "github.com/mailgun/gotools-log"
	runtime "github.com/mailgun/gotools-runtime"
	"github.com/mailgun/vulcan"
	"github.com/mailgun/vulcan/endpoint"
	"github.com/mailgun/vulcan/loadbalance/roundrobin"
	"github.com/mailgun/vulcan/location/httploc"
	"github.com/mailgun/vulcan/route/hostroute"
	"github.com/mailgun/vulcan/route/pathroute"
	"net/http"
	"strings"
	"time"
)

type Service struct {
	client  *etcd.Client
	proxy   *vulcan.Proxy
	options Options
	router  *hostroute.HostRouter
}

func NewService(options Options) *Service {
	return &Service{
		options: options,
		client:  etcd.NewClient(options.EtcdNodes),
	}
}

func (s *Service) Start() error {
	// Init logging
	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})

	if err := s.client.SetConsistency(s.options.EtcdConsistency); err != nil {
		return nil
	}

	if s.options.PidPath != "" {
		if err := runtime.WritePid(s.options.PidPath); err != nil {
			return fmt.Errorf("Failed to write PID file: %v\n", err)
		}
	}

	if err := s.createProxy(); err != nil {
		return err
	}

	if err := s.configureProxy(); err != nil {
		return err
	}

	return s.startProxy()
}

func (s *Service) createProxy() error {
	s.router = hostroute.NewHostRouter()
	proxy, err := vulcan.NewProxy(s.router)
	if err != nil {
		return err
	}
	s.proxy = proxy
	return nil
}

func (s *Service) configureProxy() error {
	for _, server := range s.getDirs(s.options.EtcdKey, "servers") {
		log.Infof("Configuring server: %s", server)
		if err := s.configureServerLocations(server); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) configureServerLocations(serverPath string) error {
	router := pathroute.NewPathRouter()
	locationPaths := s.getDirs(serverPath, "locations")
	log.Infof("Server(%s) locations: %s", serverPath, locationPaths)
	for _, locationPath := range locationPaths {
		path, ok := s.getVal(locationPath, "path")
		if !ok {
			log.Errorf("Missing location path: %s", path)
			continue
		}
		log.Infof("Configuring Server(%s) Path(%s)", key(serverPath), path)
		rr, err := roundrobin.NewRoundRobin()
		if err != nil {
			return err
		}
		location, err := httploc.NewLocation(rr)
		if err != nil {
			return err
		}
		if err := router.AddLocation(path, location); err != nil {
			return err
		}
		s.configureLocationEndpoints(locationPath, rr)
	}

	// Connect the root router and the hostname specific router
	return s.router.SetRouter(key(serverPath), router)
}

func (s *Service) configureLocationEndpoints(locationPath string, rr *roundrobin.RoundRobin) error {
	endpointUrls := s.getVals(locationPath, "endpoints")
	log.Infof("Location(%s) endpoints(%s)", locationPath, endpointUrls)

	for _, endpointUrl := range endpointUrls {
		endpoint, err := endpoint.ParseUrl(endpointUrl)
		if err != nil {
			fmt.Printf("Ignoring endpoint: failed to parse url: %s", endpoint)
		}
		if err := rr.AddEndpoint(endpoint); err != nil {
			return err
		}
		log.Infof("Added endpoint(%s) to location(%s)", endpointUrl, locationPath)
	}
	return nil
}

func (s *Service) getVal(keys ...string) (string, bool) {
	response, err := s.client.Get(strings.Join(keys, "/"), false, false)
	if notFound(err) {
		return "", false
	}
	if isDir(response.Node) {
		return "", false
	}
	return response.Node.Value, true
}

func (s *Service) getDirs(keys ...string) []string {
	var out []string
	response, err := s.client.Get(strings.Join(keys, "/"), true, true)
	if notFound(err) {
		return out
	}

	if !isDir(response.Node) {
		return out
	}

	for _, srvNode := range response.Node.Nodes {
		if isDir(&srvNode) {
			out = append(out, srvNode.Key)
		}
	}
	return out
}

func (s *Service) getVals(keys ...string) []string {
	var out []string
	response, err := s.client.Get(strings.Join(keys, "/"), true, true)
	if notFound(err) {
		return out
	}

	if !isDir(response.Node) {
		return out
	}

	for _, srvNode := range response.Node.Nodes {
		if !isDir(&srvNode) {
			out = append(out, srvNode.Value)
		}
	}
	return out
}

func key(key string) string {
	vals := strings.Split(key, "/")
	return vals[len(vals)-1]
}

func notFound(err error) bool {
	if err == nil {
		return false
	}
	eErr, ok := err.(*etcd.EtcdError)
	return ok && eErr.ErrorCode == 100
}

func isDir(n *etcd.Node) bool {
	return n != nil && n.Dir == true
}

func (s *Service) startProxy() error {
	addr := fmt.Sprintf("%s:%d", s.options.Interface, s.options.Port)
	server := &http.Server{
		Addr:           addr,
		Handler:        s.proxy,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	return server.ListenAndServe()
}

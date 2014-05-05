package etcdbackend

import (
	"github.com/mailgun/go-etcd/etcd"
	log "github.com/mailgun/gotools-log"
	. "github.com/mailgun/vulcand/backend"
	. "launchpad.net/gocheck"
	"os"
	"strings"
	"testing"
	"time"
)

func TestConfigure(t *testing.T) { TestingT(t) }

type EtcdBackendSuite struct {
	backend     *EtcdBackend
	nodes       []string
	etcdPrefix  string
	consistency string
	client      *etcd.Client
	changesC    chan interface{}
}

var _ = Suite(&EtcdBackendSuite{etcdPrefix: "vulcandtest", consistency: etcd.STRONG_CONSISTENCY})

func (s *EtcdBackendSuite) SetUpSuite(c *C) {
	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})

	nodes_string := os.Getenv("VULCAND_ETCD_NODES")
	if nodes_string == "" {
		// Skips the entire suite
		c.Skip("This test requires etcd, provide comma separated nodes in VULCAND_TEST_ETCD_NODES environment variable")
		return
	}

	s.nodes = strings.Split(nodes_string, ",")
}

func (s *EtcdBackendSuite) SetUpTest(c *C) {
	// Initiate a backend
	backend, err := NewEtcdBackend(s.nodes, s.etcdPrefix, s.consistency)
	c.Assert(err, IsNil)
	s.backend = backend
	s.client = s.backend.client

	// Delete all values under the given prefix
	_, err = s.client.Get(s.etcdPrefix, false, false)
	if err != nil {
		// There's no key like this
		if notFound(err) {
			return
		}
		// We haven't expected this error, oops
		c.Assert(err, IsNil)
	}

	_, err = s.backend.client.Delete(s.etcdPrefix, true)
	c.Assert(err, IsNil)

	s.changesC = make(chan interface{})
	go s.backend.WatchChanges(s.changesC, false)
}

func (s *EtcdBackendSuite) TearDownTest(c *C) {
	// Make sure we've recognized the change
	s.backend.StopWatching()
}

func (s *EtcdBackendSuite) collectChanges(c *C, expected int) []interface{} {
	changes := make([]interface{}, expected)
	for i, _ := range changes {
		select {
		case changes[i] = <-s.changesC:
			//
		case <-time.After(time.Second):
			c.Fatalf("Timeout occured")
		}
	}
	return changes
}

func (s *EtcdBackendSuite) expectChanges(c *C, expected ...interface{}) {
	changes := s.collectChanges(c, len(expected))
	for i, ch := range changes {
		c.Assert(ch, DeepEquals, expected[i])
	}
}

/*
func (s *EtcdBackendSuite) TestAddBadHost(c *C) {
	// Add the host with empty hostname won't work
	err := s.backend.AddHost("")
	c.Assert(err, NotNil)
}*/

func (s *EtcdBackendSuite) TestAddHost(c *C) {
	// Add the host
	err := s.backend.AddHost("localhost")
	c.Assert(err, IsNil)

	s.expectChanges(c, &HostAdded{
		Host: &Host{
			Name:      "localhost",
			EtcdKey:   s.backend.path("hosts", "localhost"),
			Locations: []*Location{}}})
}

func (s *EtcdBackendSuite) TestAddTwice(c *C) {
	// Add the host twice
	err := s.backend.AddHost("localhost")
	c.Assert(err, IsNil)

	err = s.backend.AddHost("localhost")
	c.Assert(err, NotNil)
}

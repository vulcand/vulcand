package backend

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/vulcand/vulcand/engine"
	"github.com/vulcand/vulcand/proxy"
	. "gopkg.in/check.v1"
)

var _ = Suite(&BackendSuite{})

type BackendSuite struct {
}

func (s *BackendSuite) TestNew(c *C) {
	beCfg, err := engine.NewHTTPBackend("foo", engine.HTTPBackendSettings{})
	c.Assert(err, IsNil)
	be, err := New(*beCfg, proxy.Options{}, nil)
	c.Assert(err, IsNil)

	// When
	_, srvCfgs := be.Snapshot()

	// Then
	c.Assert(srvCfgs, IsNil)
}

func (s *BackendSuite) TestNewFailure(c *C) {
	// When
	beCfg, err := engine.NewHTTPBackend("foo", engine.HTTPBackendSettings{
		Timeouts: engine.HTTPBackendTimeouts{
			Dial: "bar",
		},
	})

	// Then
	c.Assert(err.Error(), Equals, "invalid dial timeout: time: invalid duration bar")
	c.Assert(beCfg, IsNil)
}

// Returned server config is immutable, in the sense that it is not affected
// by changes made to the backend after the snapshot that produced it was taken.
func (s *BackendSuite) TestCopyOnRead(c *C) {
	beCfg, err := engine.NewHTTPBackend("foo", engine.HTTPBackendSettings{})
	c.Assert(err, IsNil)
	be, err := New(*beCfg, proxy.Options{}, []Srv{newBeSrv("1"), newBeSrv("3")})
	c.Assert(err, IsNil)

	// When
	be.UpsertServer(engine.Server{Id: "2"})
	_, srvCfgs1 := be.Snapshot()

	be.UpsertServer(engine.Server{Id: "3"}) // Duplicate
	be.UpsertServer(engine.Server{Id: "4"})
	_, srvCfgs2 := be.Snapshot()

	be.DeleteServer(engine.ServerKey{Id: "5"}) // Missing
	be.DeleteServer(engine.ServerKey{Id: "1"})
	be.UpsertServer(engine.Server{Id: "5"})
	be.DeleteServer(engine.ServerKey{Id: "2"})
	be.UpsertServer(engine.Server{Id: "1"})
	_, srvCfgs3 := be.Snapshot()

	// Then
	c.Assert(srvCfgs1, DeepEquals, []engine.Server{{Id: "1"}, {Id: "3"}, {Id: "2"}})
	c.Assert(srvCfgs2, DeepEquals, []engine.Server{{Id: "1"}, {Id: "3"}, {Id: "2"}, {Id: "4"}})
	c.Assert(srvCfgs3, DeepEquals, []engine.Server{{Id: "3"}, {Id: "4"}, {Id: "5"}, {Id: "1"}})
}

// Server Upsert/Delete functions report whether servers has actually been
// updated.
func (s *BackendSuite) TestServerUpdates(c *C) {
	beCfg, err := engine.NewHTTPBackend("foo", engine.HTTPBackendSettings{})
	c.Assert(err, IsNil)
	be, err := New(*beCfg, proxy.Options{}, []Srv{newBeSrv("1"), newBeSrv("3")})
	c.Assert(err, IsNil)

	for i, tc := range []struct {
		id        string
		operation string
		mutated   bool
	}{
		{id: "2", operation: "ups", mutated: true},
		{id: "3", operation: "ups", mutated: false},
		{id: "5", operation: "del", mutated: false},
		{id: "1", operation: "del", mutated: true},
		{id: "5", operation: "ups", mutated: true},
		{id: "2", operation: "del", mutated: true},
		{id: "1", operation: "ups", mutated: true},
		{id: "1", operation: "ups", mutated: false},
		{id: "3", operation: "ups", mutated: false},
		{id: "5", operation: "del", mutated: true},
	} {
		fmt.Printf("Test case #%d", i)
		var err error
		var mutated bool

		// When
		switch tc.operation {
		case "ups":
			mutated, err = be.UpsertServer(engine.Server{Id: tc.id})
			c.Assert(err, IsNil)
		case "del":
			mutated = be.DeleteServer(engine.ServerKey{Id: tc.id})
		}
		// Then
		c.Assert(mutated, Equals, tc.mutated)
	}

	_, srvCfgs := be.Snapshot()
	c.Assert(srvCfgs, DeepEquals, []engine.Server{{Id: "3"}, {Id: "4"}, {Id: "1"}})
}

func (s *BackendSuite) TestUpdate(c *C) {
	beCfg, err := engine.NewHTTPBackend("foo", engine.HTTPBackendSettings{
		Timeouts: engine.HTTPBackendTimeouts{
			Read:         "3s",
			TLSHandshake: "15s",
		},
	})
	c.Assert(err, IsNil)
	be, err := New(*beCfg, proxy.Options{}, []Srv{newBeSrv("1"), newBeSrv("3")})
	c.Assert(err, IsNil)

	beCfg2 := engine.Backend{
		Id: "foo",
		Settings: engine.HTTPBackendSettings{
			Timeouts: engine.HTTPBackendTimeouts{
				Read:         "7s",
				TLSHandshake: "19s",
			},
		},
	}

	// When
	mutated, err := be.Update(beCfg2, proxy.Options{})

	// Then
	c.Assert(err, Equals, nil)
	c.Assert(mutated, Equals, true)
	tp, _ := be.Snapshot()
	c.Assert(tp.ResponseHeaderTimeout, Equals, 7*time.Second)
	c.Assert(tp.TLSHandshakeTimeout, Equals, 19*time.Second)
}

// If the new config is essentially the same then Update returns mutated=false.
func (s *BackendSuite) TestUpdateSame(c *C) {
	beCfg, err := engine.NewHTTPBackend("foo", engine.HTTPBackendSettings{
		Timeouts: engine.HTTPBackendTimeouts{
			Read:         "3s",
			TLSHandshake: "15s",
		},
	})
	c.Assert(err, IsNil)
	be, err := New(*beCfg, proxy.Options{}, []Srv{newBeSrv("1"), newBeSrv("3")})
	c.Assert(err, IsNil)

	beCfg2 := engine.Backend{
		Id: "foo",
		Settings: engine.HTTPBackendSettings{
			Timeouts: engine.HTTPBackendTimeouts{
				Read:         "3s",
				TLSHandshake: "15s",
			},
		},
	}

	// When
	mutated, err := be.Update(beCfg2, proxy.Options{})

	// Then
	c.Assert(err, Equals, nil)
	c.Assert(mutated, Equals, false)
	tp, _ := be.Snapshot()
	c.Assert(tp.ResponseHeaderTimeout, Equals, 3*time.Second)
	c.Assert(tp.TLSHandshakeTimeout, Equals, 15*time.Second)
}

// Bad config is ignored entirely and does not result in partial update.
func (s *BackendSuite) TestUpdateBadConfig(c *C) {
	beCfg, err := engine.NewHTTPBackend("foo", engine.HTTPBackendSettings{
		Timeouts: engine.HTTPBackendTimeouts{
			Read:         "3s",
			TLSHandshake: "15s",
		},
	})
	c.Assert(err, IsNil)
	be, err := New(*beCfg, proxy.Options{}, []Srv{newBeSrv("1"), newBeSrv("3")})
	c.Assert(err, IsNil)

	beCfg2 := engine.Backend{
		Id: "foo",
		Settings: engine.HTTPBackendSettings{
			Timeouts: engine.HTTPBackendTimeouts{
				Read:         "5s",
				TLSHandshake: "bar",
			},
		},
	}

	// When
	mutated, err := be.Update(beCfg2, proxy.Options{})

	// Then
	c.Assert(err.Error(), Equals, "bad config: invalid HTTP cfg: invalid tls handshake timeout: time: invalid duration bar")
	c.Assert(mutated, Equals, false)
	tp, _ := be.Snapshot()
	c.Assert(tp.ResponseHeaderTimeout, Equals, 3*time.Second)
	c.Assert(tp.TLSHandshakeTimeout, Equals, 15*time.Second)
}

func newBeSrv(id string) Srv {
	beSrv, err := NewServer(engine.Server{
		Id:  id,
		URL: fmt.Sprintf("http://localhost/%s", id),
	})
	if err != nil {
		panic(errors.Wrap(err, "must not fail"))
	}
	return beSrv
}

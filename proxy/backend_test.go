package proxy

import (
	"time"

	"github.com/vulcand/vulcand/engine"
	. "gopkg.in/check.v1"
)

var _ = Suite(&BackendSuite{})

type BackendSuite struct {
}

func (s *BackendSuite) TestNew(c *C) {
	beCfg, err := engine.NewHTTPBackend("foo", engine.HTTPBackendSettings{})
	c.Assert(err, IsNil)
	be, err := newBackend(*beCfg, Options{}, nil)
	c.Assert(err, IsNil)

	// When
	_, srvCfgs := be.snapshot()

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
	be, err := newBackend(*beCfg, Options{}, []engine.Server{{Id: "1"}, {Id: "3"}})
	c.Assert(err, IsNil)

	// When
	be.upsertServer(engine.Server{Id: "2"})
	_, srvCfgs1 := be.snapshot()

	be.upsertServer(engine.Server{Id: "3"}) // Duplicate
	be.upsertServer(engine.Server{Id: "4"})
	_, srvCfgs2 := be.snapshot()

	be.deleteServer(engine.ServerKey{Id: "5"}) // Missing
	be.deleteServer(engine.ServerKey{Id: "1"})
	be.upsertServer(engine.Server{Id: "5"})
	be.deleteServer(engine.ServerKey{Id: "2"})
	be.upsertServer(engine.Server{Id: "1"})
	_, srvCfgs3 := be.snapshot()

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
	be, err := newBackend(*beCfg, Options{}, []engine.Server{{Id: "1"}, {Id: "3"}})
	c.Assert(err, IsNil)

	// When/Then
	c.Assert(be.upsertServer(engine.Server{Id: "2"}), Equals, true)
	c.Assert(be.upsertServer(engine.Server{Id: "3"}), Equals, false)
	c.Assert(be.upsertServer(engine.Server{Id: "4"}), Equals, true)
	c.Assert(be.deleteServer(engine.ServerKey{Id: "5"}), Equals, false)
	c.Assert(be.deleteServer(engine.ServerKey{Id: "1"}), Equals, true)
	c.Assert(be.upsertServer(engine.Server{Id: "5"}), Equals, true)
	c.Assert(be.deleteServer(engine.ServerKey{Id: "2"}), Equals, true)
	c.Assert(be.upsertServer(engine.Server{Id: "1"}), Equals, true)
	c.Assert(be.upsertServer(engine.Server{Id: "1"}), Equals, false)
	c.Assert(be.upsertServer(engine.Server{Id: "3"}), Equals, false)
	c.Assert(be.deleteServer(engine.ServerKey{Id: "5"}), Equals, true)

	_, srvCfgs := be.snapshot()
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
	be, err := newBackend(*beCfg, Options{}, []engine.Server{{Id: "1"}, {Id: "3"}})
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
	mutated, err := be.update(beCfg2, Options{})

	// Then
	c.Assert(err, Equals, nil)
	c.Assert(mutated, Equals, true)
	tp, _ := be.snapshot()
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
	be, err := newBackend(*beCfg, Options{}, []engine.Server{{Id: "1"}, {Id: "3"}})
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
	mutated, err := be.update(beCfg2, Options{})

	// Then
	c.Assert(err, Equals, nil)
	c.Assert(mutated, Equals, false)
	tp, _ := be.snapshot()
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
	be, err := newBackend(*beCfg, Options{}, []engine.Server{{Id: "1"}, {Id: "3"}})
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
	mutated, err := be.update(beCfg2, Options{})

	// Then
	c.Assert(err.Error(), Equals, "bad config: invalid HTTP cfg: invalid tls handshake timeout: time: invalid duration bar")
	c.Assert(mutated, Equals, false)
	tp, _ := be.snapshot()
	c.Assert(tp.ResponseHeaderTimeout, Equals, 3*time.Second)
	c.Assert(tp.TLSHandshakeTimeout, Equals, 15*time.Second)
}

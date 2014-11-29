package testutils

import (
	"fmt"
	"sync/atomic"

	"github.com/mailgun/vulcand/engine"
	"github.com/mailgun/vulcand/plugin/ratelimit"
)

var lastId int64

func UID(prefix string) string {
	return fmt.Sprintf("%s%d", prefix, atomic.AddInt64(&lastId, 1))
}

type Batch struct {
	Route    string
	Addr     string
	URL      string
	Protocol string
	Host     string
	KeyPair  *engine.KeyPair
}

type BatchVal struct {
	H engine.Host

	L  engine.Listener
	LK engine.ListenerKey

	F  engine.Frontend
	FK engine.FrontendKey

	B  engine.Backend
	BK engine.BackendKey

	S  engine.Server
	SK engine.ServerKey
}

func MakeURL(l engine.Listener, path string) string {
	return fmt.Sprintf("%s://%s%s", l.Protocol, l.Address.Address, path)
}

func (b BatchVal) FrontendURL(path string) string {
	return MakeURL(b.L, path)
}

func MakeBatch(b Batch) BatchVal {
	if b.Host == "" {
		b.Host = "localhost"
	}
	if b.Protocol == "" {
		b.Protocol = engine.HTTP
	}
	h := MakeHost(b.Host, b.KeyPair)
	bk := MakeBackend()
	l := MakeListener(b.Addr, b.Protocol)
	f := MakeFrontend(b.Route, bk.Id)
	s := MakeServer(b.URL)
	return BatchVal{
		H: h,

		L:  l,
		LK: engine.ListenerKey{Id: l.Id},

		F:  f,
		FK: engine.FrontendKey{Id: f.Id},

		B:  bk,
		BK: engine.BackendKey{Id: bk.Id},

		S:  s,
		SK: engine.ServerKey{BackendKey: engine.BackendKey{Id: bk.Id}, Id: s.Id},
	}
}

func MakeHost(name string, keyPair *engine.KeyPair) engine.Host {
	return engine.Host{
		Name:     name,
		Settings: engine.HostSettings{KeyPair: keyPair},
	}
}

func MakeListener(addr string, protocol string) engine.Listener {
	l, err := engine.NewListener(UID("listener"), protocol, engine.TCP, addr)
	if err != nil {
		panic(err)
	}
	return *l
}

func MakeFrontend(route string, backendId string) engine.Frontend {
	f, err := engine.NewHTTPFrontend(UID("frontend"), backendId, route, engine.HTTPFrontendSettings{})
	if err != nil {
		panic(err)
	}
	return *f
}

func MakeBackend() engine.Backend {
	b, err := engine.NewHTTPBackend(UID("backend"), engine.HTTPBackendSettings{})
	if err != nil {
		panic(err)
	}
	return *b
}

func MakeServer(url string) engine.Server {
	s, err := engine.NewServer(UID("server"), url)
	if err != nil {
		panic(err)
	}
	return *s
}

func MakeRateLimit(id string, rate int64, variable string, burst int64, periodSeconds int64) engine.Middleware {
	rl, err := ratelimit.FromOther(ratelimit.RateLimit{
		PeriodSeconds: periodSeconds,
		Requests:      rate,
		Burst:         burst,
		Variable:      variable})
	if err != nil {
		panic(err)
	}
	return engine.Middleware{
		Type:       "ratelimit",
		Id:         id,
		Middleware: rl,
	}
}

func NewTestKeyPair() *engine.KeyPair {
	return &engine.KeyPair{
		Key:  LocalhostKey,
		Cert: LocalhostCert,
	}
}

var LocalhostCert = []byte(`-----BEGIN CERTIFICATE-----
MIIBdzCCASOgAwIBAgIBADALBgkqhkiG9w0BAQUwEjEQMA4GA1UEChMHQWNtZSBD
bzAeFw03MDAxMDEwMDAwMDBaFw00OTEyMzEyMzU5NTlaMBIxEDAOBgNVBAoTB0Fj
bWUgQ28wWjALBgkqhkiG9w0BAQEDSwAwSAJBAN55NcYKZeInyTuhcCwFMhDHCmwa
IUSdtXdcbItRB/yfXGBhiex00IaLXQnSU+QZPRZWYqeTEbFSgihqi1PUDy8CAwEA
AaNoMGYwDgYDVR0PAQH/BAQDAgCkMBMGA1UdJQQMMAoGCCsGAQUFBwMBMA8GA1Ud
EwEB/wQFMAMBAf8wLgYDVR0RBCcwJYILZXhhbXBsZS5jb22HBH8AAAGHEAAAAAAA
AAAAAAAAAAAAAAEwCwYJKoZIhvcNAQEFA0EAAoQn/ytgqpiLcZu9XKbCJsJcvkgk
Se6AbGXgSlq+ZCEVo0qIwSgeBqmsJxUu7NCSOwVJLYNEBO2DtIxoYVk+MA==
-----END CERTIFICATE-----`)

// localhostKey is the private key for localhostCert.
var LocalhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIBPAIBAAJBAN55NcYKZeInyTuhcCwFMhDHCmwaIUSdtXdcbItRB/yfXGBhiex0
0IaLXQnSU+QZPRZWYqeTEbFSgihqi1PUDy8CAwEAAQJBAQdUx66rfh8sYsgfdcvV
NoafYpnEcB5s4m/vSVe6SU7dCK6eYec9f9wpT353ljhDUHq3EbmE4foNzJngh35d
AekCIQDhRQG5Li0Wj8TM4obOnnXUXf1jRv0UkzE9AHWLG5q3AwIhAPzSjpYUDjVW
MCUXgckTpKCuGwbJk7424Nb8bLzf3kllAiA5mUBgjfr/WtFSJdWcPQ4Zt9KTMNKD
EUO0ukpTwEIl6wIhAMbGqZK3zAAFdq8DD2jPx+UJXnh0rnOkZBzDtJ6/iN69AiEA
1Aq8MJgTaYsDQWyU/hDq5YkDJc9e9DSCvUIzqxQWMQE=
-----END RSA PRIVATE KEY-----`)

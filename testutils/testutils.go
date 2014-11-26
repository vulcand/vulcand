package testutils

import (
	"fmt"
	"sync/atomic"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/loadbalance/roundrobin"
	"github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/plugin/ratelimit"
)

var lastId int64

type LocOpts struct {
	UpId     string
	Hostname string
	Addr     string
	URL      string
	LocId    string
}

func nextId(prefix string) string {
	return fmt.Sprintf("%s%d", prefix, atomic.AddInt64(&lastId, 1))
}

func MakeLocation(o LocOpts) (*backend.Location, *backend.Host) {
	o = setDefaults(o)
	host := &backend.Host{
		Name: o.Hostname,
		Listeners: []*backend.Listener{
			&backend.Listener{
				Protocol: backend.HTTP,
				Address:  backend.Address{Network: "tcp", Address: o.Addr},
			},
		},
	}
	upstream := &backend.Upstream{
		Id: o.UpId,
		Endpoints: []*backend.Endpoint{
			{
				UpstreamId: o.UpId,
				Id:         o.URL,
				Url:        o.URL,
			},
		},
	}
	location := &backend.Location{
		Hostname: host.Name,
		Path:     fmt.Sprintf("/%s", o.LocId),
		Id:       o.LocId,
		Upstream: upstream,
	}
	return location, host
}

func setDefaults(o LocOpts) LocOpts {
	if o.UpId == "" {
		o.UpId = nextId("up")
	}
	if o.LocId == "" {
		o.LocId = nextId("loc")
	}
	return o
}

func MakeRateLimit(id string, rate int64, variable string, burst int64, periodSeconds int64, loc *backend.Location) *backend.MiddlewareInstance {
	rl, err := ratelimit.FromOther(ratelimit.RateLimit{
		PeriodSeconds: periodSeconds,
		Requests: rate,
		Burst: burst,
		Variable: variable})
	if err != nil {
		panic(err)
	}
	return &backend.MiddlewareInstance{
		Type:       "ratelimit",
		Id:         id,
		Middleware: rl,
	}
}

func MakeURL(loc *backend.Location, l *backend.Listener) string {
	return fmt.Sprintf("%s://%s%s", l.Protocol, l.Address.Address, loc.Path)
}

func EndpointsEq(a []*roundrobin.WeightedEndpoint, b []*backend.Endpoint) bool {
	x, y := map[string]bool{}, map[string]bool{}
	for _, e := range a {
		x[e.GetUrl().String()] = true
	}

	for _, e := range b {
		y[e.Url] = true
	}

	if len(x) != len(y) {
		return false
	}

	for k, _ := range x {
		_, ok := y[k]
		if !ok {
			return false
		}
	}
	return true
}

func NewTestKeyPair() *backend.KeyPair {
	return &backend.KeyPair{
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

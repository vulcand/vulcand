package testutils

import (
	"fmt"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/loadbalance/roundrobin"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
	. "github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/plugin/ratelimit"
)

func MakeLocation(hostname, listenerAddress, endpointURL string) (*Location, *Host) {
	host := &Host{
		Name: hostname,
		Listeners: []*Listener{
			&Listener{Protocol: HTTP, Address: Address{"tcp", listenerAddress}}},
	}

	upstream := &Upstream{
		Id: "up1",
		Endpoints: []*Endpoint{
			{
				UpstreamId: "up1",
				Id:         endpointURL,
				Url:        endpointURL,
			},
		},
	}
	location := &Location{
		Hostname: host.Name,
		Path:     "/loc1",
		Id:       "loc1",
		Upstream: upstream,
	}
	return location, host
}

func MakeRateLimit(id string, rate int, variable string, burst int64, periodSeconds int, loc *Location) *MiddlewareInstance {
	rl, err := ratelimit.NewRateLimit(rate, variable, burst, periodSeconds)
	if err != nil {
		panic(err)
	}
	return &MiddlewareInstance{
		Type:       "ratelimit",
		Id:         id,
		Middleware: rl,
	}
}

func MakeURL(loc *Location, l *Listener) string {
	return fmt.Sprintf("%s://%s%s", l.Protocol, l.Address.Address, loc.Path)
}

func AssertSameEndpoints(c *C, a []*roundrobin.WeightedEndpoint, b []*Endpoint) {
	x, y := map[string]bool{}, map[string]bool{}
	for _, e := range a {
		x[e.GetUrl().String()] = true
	}

	for _, e := range b {
		y[e.Url] = true
	}
	c.Assert(x, DeepEquals, y)
}

func NewTestKeyPair() *KeyPair {
	return &KeyPair{
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

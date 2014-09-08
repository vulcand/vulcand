package server

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/testutils"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
	. "github.com/mailgun/vulcand/backend"
	"testing"
)

func TestServer(t *testing.T) { TestingT(t) }

var _ = Suite(&ServerSuite{})

type ServerSuite struct {
	mux    *MuxServer
	lastId int
}

func (s *ServerSuite) SetUpTest(c *C) {
	m, err := NewMuxServerWithOptions(s.lastId, Options{})
	c.Assert(err, IsNil)
	s.mux = m
}

func (s *ServerSuite) TearDownTest(c *C) {
	s.mux.Stop(true)
}

func (s *ServerSuite) TestStartStop(c *C) {
	c.Assert(s.mux.Start(), IsNil)
}

func (s *ServerSuite) TestServerCRUD(c *C) {
	e := NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi, I'm endpoint"))
	})
	defer e.Close()

	l, h := makeLocation("localhost", "localhost:31000", e.URL)

	c.Assert(s.mux.UpsertHost(h), IsNil)
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	c.Assert(s.mux.Start(), IsNil)

	response, bodyBytes, err := GET(makeURL(l, h.Listeners[0]), "")
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(bodyBytes), Equals, "Hi, I'm endpoint")

	c.Assert(s.mux.DeleteHost(h.Name), IsNil)

	_, _, err = GET(makeURL(l, h.Listeners[0]), "")
	c.Assert(err, NotNil)
}

// Test case when you have two hosts on the same domain
func (s *ServerSuite) TestTwoHosts(c *C) {
	e := NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi, I'm endpoint 1"))
	})
	defer e.Close()

	e2 := NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi, I'm endpoint 2"))
	})
	defer e2.Close()

	c.Assert(s.mux.Start(), IsNil)

	l, h := makeLocation("localhost", "localhost:31000", e.URL)
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	l2, h2 := makeLocation("otherhost", "localhost:31000", e2.URL)
	c.Assert(s.mux.UpsertLocation(h2, l2), IsNil)

	response, bodyBytes, err := GET(makeURL(l, h.Listeners[0]), "")
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(bodyBytes), Equals, "Hi, I'm endpoint 1")

	response, bodyBytes, err = GET(makeURL(l, h2.Listeners[0]), "otherhost")
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(bodyBytes), Equals, "Hi, I'm endpoint 2")
}

func (s *ServerSuite) TestServerListenerCRUD(c *C) {
	e := NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi, I'm endpoint"))
	})
	defer e.Close()

	c.Assert(s.mux.Start(), IsNil)

	l, h := makeLocation("localhost", "localhost:31000", e.URL)

	c.Assert(s.mux.UpsertHost(h), IsNil)
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	h.Listeners = append(h.Listeners, &Listener{Id: "l2", Protocol: HTTP, Address: Address{"tcp", "localhost:31001"}})

	s.mux.AddHostListener(h, h.Listeners[1])

	response, bodyBytes, err := GET(makeURL(l, h.Listeners[1]), "")
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(bodyBytes), Equals, "Hi, I'm endpoint")

	c.Assert(s.mux.DeleteHostListener(h, h.Listeners[1].Id), IsNil)

	_, _, err = GET(makeURL(l, h.Listeners[1]), "")
	c.Assert(err, NotNil)
}

func (s *ServerSuite) TestServerHTTPSCRUD(c *C) {
	e := NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi, I'm endpoint"))
	})
	defer e.Close()

	l, h := makeLocation("localhost", "localhost:31000", e.URL)
	h.Cert = &Certificate{Key: localhostKey, Cert: localhostCert}
	h.Listeners[0].Protocol = HTTPS

	c.Assert(s.mux.UpsertHost(h), IsNil)
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	c.Assert(s.mux.Start(), IsNil)

	response, bodyBytes, err := GET(makeURL(l, h.Listeners[0]), "")
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(bodyBytes), Equals, "Hi, I'm endpoint")

	c.Assert(s.mux.DeleteHost(h.Name), IsNil)

	_, _, err = GET(makeURL(l, h.Listeners[0]), "")
	c.Assert(err, NotNil)
}

func (s *ServerSuite) TestLiveCertUpdate(c *C) {
	e := NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi, I'm endpoint"))
	})
	defer e.Close()
	c.Assert(s.mux.Start(), IsNil)

	l, h := makeLocation("localhost", "localhost:31000", e.URL)
	h.Cert = &Certificate{Key: localhostKey, Cert: localhostCert}
	h.Listeners[0].Protocol = HTTPS

	c.Assert(s.mux.UpsertHost(h), IsNil)
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	response, bodyBytes, err := GET(makeURL(l, h.Listeners[0]), "")
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(bodyBytes), Equals, "Hi, I'm endpoint")

	h.Cert = &Certificate{Key: localhostKey2, Cert: localhostCert2}
	c.Assert(s.mux.UpdateHostCert(h.Name, h.Cert), IsNil)

	response, bodyBytes, err = GET(makeURL(l, h.Listeners[0]), "")
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(bodyBytes), Equals, "Hi, I'm endpoint")
}

func (s *ServerSuite) TestSNI(c *C) {
	e := NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi, I'm endpoint 1"))
	})
	defer e.Close()

	e2 := NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi, I'm endpoint 2"))
	})
	defer e2.Close()

	c.Assert(s.mux.Start(), IsNil)
	l, h := makeLocation("localhost", "localhost:31000", e.URL)
	h.Cert = &Certificate{Key: localhostKey, Cert: localhostCert}
	h.Listeners[0].Protocol = HTTPS

	l2, h2 := makeLocation("otherhost", "localhost:31000", e2.URL)
	h2.Cert = &Certificate{Key: localhostKey2, Cert: localhostCert2}
	h2.Listeners[0].Protocol = HTTPS
	h2.Options.Default = true

	c.Assert(s.mux.UpsertLocation(h, l), IsNil)
	c.Assert(s.mux.UpsertLocation(h2, l2), IsNil)

	response, bodyBytes, err := GET(makeURL(l, h.Listeners[0]), "")
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(bodyBytes), Equals, "Hi, I'm endpoint 1")

	response, bodyBytes, err = GET(makeURL(l, h2.Listeners[0]), "otherhost")
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(bodyBytes), Equals, "Hi, I'm endpoint 2")

	s.mux.DeleteHost(h2.Name)

	response, bodyBytes, err = GET(makeURL(l, h.Listeners[0]), "")
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(bodyBytes), Equals, "Hi, I'm endpoint 1")

	response, _, err = GET(makeURL(l, h2.Listeners[0]), "otherhost")
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Not(Equals), http.StatusOK)
}

func (s *ServerSuite) TestHijacking(c *C) {
	e := NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi, I'm endpoint 1"))
	})
	defer e.Close()

	c.Assert(s.mux.Start(), IsNil)

	l, h := makeLocation("localhost", "localhost:31000", e.URL)
	h.Cert = &Certificate{Key: localhostKey, Cert: localhostCert}
	h.Listeners[0].Protocol = HTTPS

	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	response, bodyBytes, err := GET(makeURL(l, h.Listeners[0]), "")
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(bodyBytes), Equals, "Hi, I'm endpoint 1")

	mux2, err := NewMuxServerWithOptions(s.lastId, Options{})
	c.Assert(err, IsNil)

	e2 := NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi, I'm endpoint 2"))
	})
	defer e2.Close()

	l2, h2 := makeLocation("localhost", "localhost:31000", e2.URL)
	h2.Cert = &Certificate{Key: localhostKey2, Cert: localhostCert2}
	h2.Listeners[0].Protocol = HTTPS

	c.Assert(mux2.UpsertLocation(h2, l2), IsNil)
	c.Assert(mux2.HijackListenersFrom(s.mux), IsNil)

	c.Assert(mux2.Start(), IsNil)
	s.mux.Stop(true)
	defer mux2.Stop(true)

	response, bodyBytes, err = GET(makeURL(l2, h2.Listeners[0]), "")
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(bodyBytes), Equals, "Hi, I'm endpoint 2")
}

func makeURL(loc *Location, l *Listener) string {
	return fmt.Sprintf("%s://%s%s", l.Protocol, l.Address.Address, loc.Path)
}

func makeLocation(hostname, listenerAddress, endpointURL string) (*Location, *Host) {
	host := &Host{
		Name:      hostname,
		Listeners: []*Listener{&Listener{Protocol: HTTP, Address: Address{"tcp", listenerAddress}}}}

	upstream := &Upstream{
		Id: "up1",
		Endpoints: []*Endpoint{
			{
				Url: endpointURL,
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

func GET(url string, host string) (*http.Response, []byte, error) {
	request, _ := http.NewRequest("GET", url, strings.NewReader(""))
	if len(host) != 0 {
		request.Host = host
	}

	var tr *http.Transport
	if strings.HasPrefix(url, "https") {
		tr = &http.Transport{
			DisableKeepAlives: true,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		}
	} else {
		tr = &http.Transport{
			DisableKeepAlives: true,
		}
	}

	client := &http.Client{Transport: tr}
	response, err := client.Do(request)
	if err == nil {
		bodyBytes, err := ioutil.ReadAll(response.Body)
		return response, bodyBytes, err
	}
	return response, nil, err
}

// localhostCert is a PEM-encoded TLS cert with SAN IPs
// "127.0.0.1" and "[::1]", expiring at the last second of 2049 (the end
// of ASN.1 time).
// generated from src/pkg/crypto/tls:
// go run generate_cert.go  --rsa-bits 512 --host 127.0.0.1,::1,example.com --ca --start-date "Jan 1 00:00:00 1970" --duration=1000000h
var localhostCert = []byte(`-----BEGIN CERTIFICATE-----
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
var localhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIBPAIBAAJBAN55NcYKZeInyTuhcCwFMhDHCmwaIUSdtXdcbItRB/yfXGBhiex0
0IaLXQnSU+QZPRZWYqeTEbFSgihqi1PUDy8CAwEAAQJBAQdUx66rfh8sYsgfdcvV
NoafYpnEcB5s4m/vSVe6SU7dCK6eYec9f9wpT353ljhDUHq3EbmE4foNzJngh35d
AekCIQDhRQG5Li0Wj8TM4obOnnXUXf1jRv0UkzE9AHWLG5q3AwIhAPzSjpYUDjVW
MCUXgckTpKCuGwbJk7424Nb8bLzf3kllAiA5mUBgjfr/WtFSJdWcPQ4Zt9KTMNKD
EUO0ukpTwEIl6wIhAMbGqZK3zAAFdq8DD2jPx+UJXnh0rnOkZBzDtJ6/iN69AiEA
1Aq8MJgTaYsDQWyU/hDq5YkDJc9e9DSCvUIzqxQWMQE=
-----END RSA PRIVATE KEY-----`)

var localhostCert2 = []byte(`-----BEGIN CERTIFICATE-----
MIIBizCCATegAwIBAgIRAL3EdJdBpGqcIy7kqCul6qIwCwYJKoZIhvcNAQELMBIx
EDAOBgNVBAoTB0FjbWUgQ28wIBcNNzAwMTAxMDAwMDAwWhgPMjA4NDAxMjkxNjAw
MDBaMBIxEDAOBgNVBAoTB0FjbWUgQ28wXDANBgkqhkiG9w0BAQEFAANLADBIAkEA
zAy3eIgjhro/wksSVgN+tZMxNbETDPgndYpIVSMMGHRXid71Zit8R5jJg8GZhWOs
2GXAZVZIJy634mODg5Xs8QIDAQABo2gwZjAOBgNVHQ8BAf8EBAMCAKQwEwYDVR0l
BAwwCgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUwAwEB/zAuBgNVHREEJzAlggtleGFt
cGxlLmNvbYcEfwAAAYcQAAAAAAAAAAAAAAAAAAAAATALBgkqhkiG9w0BAQsDQQA2
NW/PChPgBPt4q4ATTDDmoLoWjY8Vrp++6Wtue1YQBfEyvGWTFibNLD7FFodIPg/a
5LgeVKZTukSJX31lVCBm
-----END CERTIFICATE-----`)

var localhostKey2 = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIBOwIBAAJBAMwMt3iII4a6P8JLElYDfrWTMTWxEwz4J3WKSFUjDBh0V4ne9WYr
fEeYyYPBmYVjrNhlwGVWSCcut+Jjg4OV7PECAwEAAQJAYHjOsZzj9wnNpUWrCKGk
YaKSzIjIsgQNW+QiKKZmTJS0rCJnUXUz8nSyTnS5rYd+CqOlFDXzpDbcouKGLOn5
BQIhAOtwl7+oebSLYHvznksQg66yvRxULfQTJS7aIKHNpDTPAiEA3d5gllV7EuGq
oqcbLwrFrGJ4WflasfeLpcDXuOR7sj8CIQC34IejuADVcMU6CVpnZc5yckYgCd6Z
8RnpLZKuy9yjIQIgYsykNk3agI39bnD7qfciD6HJ9kcUHCwgA6/cYHlenAECIQDZ
H4E4GFiDetx8ZOdWq4P7YRdIeepSvzPeOEv2sfsItg==
-----END RSA PRIVATE KEY-----`)

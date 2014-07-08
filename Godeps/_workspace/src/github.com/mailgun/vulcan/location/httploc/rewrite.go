package httploc

import (
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/headers"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/netutils"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
	"net"
	"net/http"
	"strings"
)

// Rewrites incom
type Rewriter struct {
	TrustForwardHeader bool
	Hostname           string
}

func (rw *Rewriter) ProcessRequest(r Request) (*http.Response, error) {
	req := r.GetHttpRequest()

	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		if rw.TrustForwardHeader {
			if prior, ok := req.Header[headers.XForwardedFor]; ok {
				clientIP = strings.Join(prior, ", ") + ", " + clientIP
			}
		}
		req.Header.Set(headers.XForwardedFor, clientIP)
	}

	if xfp := req.Header.Get(headers.XForwardedProto); xfp != "" && rw.TrustForwardHeader {
		req.Header.Set(headers.XForwardedProto, xfp)
	} else if req.TLS != nil {
		req.Header.Set(headers.XForwardedProto, "https")
	} else {
		req.Header.Set(headers.XForwardedProto, "http")
	}

	if req.Host != "" {
		req.Header.Set(headers.XForwardedHost, req.Host)
	}
	req.Header.Set(headers.XForwardedServer, rw.Hostname)

	// Remove hop-by-hop headers to the backend.  Especially important is "Connection" because we want a persistent
	// connection, regardless of what the client sent to us.
	netutils.RemoveHeaders(headers.HopHeaders, req.Header)

	return nil, nil
}

func (tl *Rewriter) ProcessResponse(r Request, a Attempt) {
}

package httploc

import (
	"net"
	"net/http"
	"strings"

	"github.com/mailgun/vulcan/headers"
	"github.com/mailgun/vulcan/netutils"
	"github.com/mailgun/vulcan/request"
)

// Rewriter is responsible for removing hop-by-hop headers, fixing encodings and content-length
type Rewriter struct {
	TrustForwardHeader bool
	Hostname           string
}

func (rw *Rewriter) ProcessRequest(r request.Request) (*http.Response, error) {
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

	// We need to set ContentLength based on known request size. The incoming request may have been
	// set without content length or using chunked TransferEncoding
	totalSize, err := r.GetBody().TotalSize()
	if err != nil {
		return nil, err
	}
	req.ContentLength = totalSize
	// Remove TransferEncoding that could have been previously set
	req.TransferEncoding = []string{}

	return nil, nil
}

func (tl *Rewriter) ProcessResponse(r request.Request, a request.Attempt) {
}

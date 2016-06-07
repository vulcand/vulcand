/*
package stream provides http.Handler middleware that solves several problems when dealing with http requests:

Reads the entire request and response into buffer, optionally buffering it to disk for large requests.
Checks the limits for the requests and responses, rejecting in case if the limit was exceeded.
Changes request content-transfer-encoding from chunked and provides total size to the handlers.

Examples of a streaming middleware:

  // sample HTTP handler
  handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
    w.Write([]byte("hello"))
  })

  // Stream will read the body in buffer before passing the request to the handler
  // calculate total size of the request and transform it from chunked encoding
  // before passing to the server
  stream.New(handler)

  // This version will buffer up to 2MB in memory and will serialize any extra
  // to a temporary file, if the request size exceeds 10MB it will reject the request
  stream.New(handler,
    stream.MemRequestBodyBytes(2 * 1024 * 1024),
    stream.MaxRequestBodyBytes(10 * 1024 * 1024))

  // Will do the same as above, but with responses
  stream.New(handler,
    stream.MemResponseBodyBytes(2 * 1024 * 1024),
    stream.MaxResponseBodyBytes(10 * 1024 * 1024))

  // Stream will replay the request if the handler returns error at least 3 times
  // before returning the response
  stream.New(handler, stream.Retry(`IsNetworkError() && Attempts() <= 2`))

*/
package stream

import (
	"net/http"

	"github.com/vulcand/oxy/utils"
)

const (
	// No limit by default
	DefaultMaxBodyBytes = -1
)

// Stream is responsible for buffering requests and responses
// It buffers large reqeuests and responses to disk,
type Stream struct {
	maxRequestBodyBytes int64

	maxResponseBodyBytes int64

	retryPredicate hpredicate

	next       http.Handler
	errHandler utils.ErrorHandler
	log        utils.Logger
}

// New returns a new streamer middleware. New() function supports optional functional arguments
func New(next http.Handler, setters ...optSetter) (*Stream, error) {
	strm := &Stream{
		next: next,

		maxRequestBodyBytes: DefaultMaxBodyBytes,

		maxResponseBodyBytes: DefaultMaxBodyBytes,
	}
	for _, s := range setters {
		if err := s(strm); err != nil {
			return nil, err
		}
	}
	if strm.log == nil {
		strm.log = utils.NullLogger
	}

	return strm, nil
}

type optSetter func(s *Stream) error

// Logger sets the logger that will be used by this middleware.
func Logger(l utils.Logger) optSetter {
	return func(s *Stream) error {
		s.log = l
		return nil
	}
}

// Wrap sets the next handler to be called by stream handler.
func (s *Stream) Wrap(next http.Handler) error {
	s.next = next
	return nil
}

func (s *Stream) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	s.next.ServeHTTP(w, req)
}

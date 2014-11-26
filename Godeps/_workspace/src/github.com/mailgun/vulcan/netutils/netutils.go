// Network related utilities
package netutils

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Provides update safe copy by avoiding
// shallow copying certain fields (like user data)
func CopyUrl(in *url.URL) *url.URL {
	out := new(url.URL)
	*out = *in
	if in.User != nil {
		*out.User = *in.User
	}
	return out
}

// RawPath returns escaped url path section
func RawPath(in string) (string, error) {
	u, err := url.ParseRequestURI(in)
	if err != nil {
		return "", err
	}
	path := ""
	if u.Opaque != "" {
		path = u.Opaque
	} else if u.Host == "" {
		path = in
	} else {
		vals := strings.SplitN(in, u.Host, 2)
		if len(vals) != 2 {
			return "", fmt.Errorf("failed to parse url")
		}
		path = vals[1]
	}
	idx := strings.IndexRune(path, '?')
	if idx == -1 {
		return path, nil
	}
	return path[:idx], nil
}

// RawURL returns URL built out of the provided request's Request-URI, to avoid un-escaping.
// Note: it assumes that scheme and host for the provided request's URL are defined.
func RawURL(request *http.Request) string {
	return strings.Join([]string{request.URL.Scheme, "://", request.URL.Host, request.RequestURI}, "")
}

// Copies http headers from source to destination
// does not overide, but adds multiple headers
func CopyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// Determines whether any of the header names is present
// in the http headers
func HasHeaders(names []string, headers http.Header) bool {
	for _, h := range names {
		if headers.Get(h) != "" {
			return true
		}
	}
	return false
}

// Removes the header with the given names from the headers map
func RemoveHeaders(names []string, headers http.Header) {
	for _, h := range names {
		headers.Del(h)
	}
}

func MustParseUrl(inUrl string) *url.URL {
	u, err := ParseUrl(inUrl)
	if err != nil {
		panic(err)
	}
	return u
}

// Standard parse url is very generous,
// parseUrl wrapper makes it more strict
// and demands scheme and host to be set
func ParseUrl(inUrl string) (*url.URL, error) {
	parsedUrl, err := url.Parse(inUrl)
	if err != nil {
		return nil, err
	}

	if parsedUrl.Host == "" || parsedUrl.Scheme == "" {
		return nil, fmt.Errorf("Empty Url is not allowed")
	}
	return parsedUrl, nil
}

type BasicAuth struct {
	Username string
	Password string
}

func (ba *BasicAuth) String() string {
	encoded := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", ba.Username, ba.Password)))
	return fmt.Sprintf("Basic %s", encoded)
}

func ParseAuthHeader(header string) (*BasicAuth, error) {

	values := strings.Fields(header)
	if len(values) != 2 {
		return nil, fmt.Errorf(fmt.Sprintf("Failed to parse header '%s'", header))
	}

	auth_type := strings.ToLower(values[0])
	if auth_type != "basic" {
		return nil, fmt.Errorf("Expected basic auth type, got '%s'", auth_type)
	}

	encoded_string := values[1]
	decoded_string, err := base64.StdEncoding.DecodeString(encoded_string)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse header '%s', base64 failed: %s", header, err)
	}

	values = strings.SplitN(string(decoded_string), ":", 2)
	if len(values) != 2 {
		return nil, fmt.Errorf("Failed to parse header '%s', expected separator ':'", header)
	}
	return &BasicAuth{Username: values[0], Password: values[1]}, nil
}

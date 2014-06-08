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
		return nil, fmt.Errorf(
			fmt.Sprintf("Failed to parse header '%s'", header))
	}

	auth_type := strings.ToLower(values[0])
	if auth_type != "basic" {
		return nil, fmt.Errorf("Expected basic auth type, got '%s'", auth_type)
	}

	encoded_string := values[1]
	decoded_string, err := base64.StdEncoding.DecodeString(encoded_string)
	if err != nil {
		err = fmt.Errorf(
			"Failed to parse header '%s', base64 failed: %s", header, err)
		return nil, err
	}

	values = strings.SplitN(string(decoded_string), ":", 2)
	if len(values) != 2 {
		err = fmt.Errorf(
			"Failed to parse header '%s', expected separator ':'", header, err)
		return nil, err
	}
	return &BasicAuth{Username: values[0], Password: values[1]}, nil
}

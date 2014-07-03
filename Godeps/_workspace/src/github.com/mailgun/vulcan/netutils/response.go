package netutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

func NewHttpResponse(request *http.Request, statusCode int, body []byte, contentType string) *http.Response {
	resp := &http.Response{
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		StatusCode: statusCode,
		Proto:      "HTTP/1.0",
		ProtoMajor: 1,
		ProtoMinor: 0,
		Header:     make(http.Header),
	}
	resp.Header.Add("Content-Type", contentType)
	resp.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	resp.ContentLength = int64(len(body))
	resp.Request = request
	return resp
}

func NewTextResponse(request *http.Request, statusCode int, body string) *http.Response {
	return NewHttpResponse(request, statusCode, []byte(body), "text/plain")
}

func NewJsonResponse(request *http.Request, statusCode int, message interface{}) *http.Response {
	bytes, err := json.Marshal(message)
	if err != nil {
		bytes = []byte("{}")
	}
	return NewHttpResponse(request, statusCode, bytes, "application/json")
}

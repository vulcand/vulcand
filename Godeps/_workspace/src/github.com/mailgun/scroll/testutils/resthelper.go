package testutils

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/scroll"
)

type RestHelper struct{}

// T is an interface common to testing.T, testing.B, and *check.C.
type T interface {
	Fatal(args ...interface{})
}

func (h *RestHelper) Get(t T, url string) scroll.Response {
	response, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	return h.parseResponse(t, response)
}

func (h *RestHelper) Post(t T, url string, data url.Values) scroll.Response {
	response, err := http.PostForm(url, data)
	if err != nil {
		t.Fatal(err)
	}
	return h.parseResponse(t, response)
}

func (h *RestHelper) PostJSON(t T, url, data string) scroll.Response {
	request, err := http.NewRequest("POST", url, strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return h.parseResponse(t, response)
}

func (h *RestHelper) Delete(t T, url string) scroll.Response {
	request, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return h.parseResponse(t, response)
}

func (h *RestHelper) parseResponse(t T, response *http.Response) scroll.Response {
	defer response.Body.Close()

	parsedResponse := scroll.Response{}
	err := json.NewDecoder(response.Body).Decode(&parsedResponse)
	if err != nil {
		t.Fatal(err)
	}

	return parsedResponse
}

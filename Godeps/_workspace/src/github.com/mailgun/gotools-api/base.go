package api

import (
	"encoding/json"
	"fmt"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/gorilla/mux"
	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"
	"io/ioutil"
	"net/http"
	"time"
)

type Response map[string]interface{}

const (
	DefaultLimit int = 100
	MaxLimit     int = 10000
	MaxBatchSize int = 1000
)

func MakeHandler(fn func(http.ResponseWriter, *http.Request, map[string]string) (interface{}, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// we'll measure time the request took
		start := time.Now()

		// set content type appropriate for JSON responses
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		// parse form for POST requests so handler functions don't have to
		if r.Method == "POST" {
			r.ParseMultipartForm(0)
		} else {
			r.ParseForm()
		}

		// pass parsed URL params into a handler function
		response, err := fn(w, r, mux.Vars(r))

		// if an error returned, return an appropriate status code
		if err != nil {
			ReplyError(w, err)
			log.Errorf("Request failed (%v): %v %v %v - took %v", err, r.Method, r.URL, r.Form, time.Since(start))
		} else {
			ReplySuccess(w, response)
			log.Infof("Request completed: %v %v %v - took %v", r.Method, r.URL, r.Form, time.Since(start))
		}
	}
}

func MakeRawHandler(fn func(http.ResponseWriter, *http.Request, map[string]string, []byte) (interface{}, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// we'll measure time the request took
		start := time.Now()

		// set content type appropriate for JSON responses
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			ReplyError(w, fmt.Errorf("Failed to read request body: %s", err))
			return
		}
		response, err := fn(w, r, mux.Vars(r), body)

		// if an error returned, return an appropriate status code
		if err != nil {
			ReplyError(w, err)
			log.Errorf("Request failed (%v): %v %v %v - took %v", err, r.Method, r.URL, r.Form, time.Since(start))
		} else {
			ReplySuccess(w, response)
			log.Infof("Request completed: %v %v %v - took %v", r.Method, r.URL, r.Form, time.Since(start))
		}
	}
}

func ReplyError(w http.ResponseWriter, err error) {
	var response Response
	var statusCode int

	switch err.(type) {
	case GenericAPIError, MissingFieldError, InvalidFormatError, InvalidParameterError:
		response = Response{"message": err.Error()}
		statusCode = http.StatusBadRequest
	case NotFoundError:
		response = Response{"message": err.Error()}
		statusCode = http.StatusNotFound
	case ConflictError:
		response = Response{"message": err.Error()}
		statusCode = http.StatusConflict
	default:
		response = Response{"message": "Internal Server Error"}
		statusCode = http.StatusInternalServerError
	}

	marshalled, err := json.Marshal(response)
	if err != nil {
		log.Errorf("Failed to marshal response: %v %v", response, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(statusCode)
	fmt.Fprintln(w, string(marshalled))
}

func ReplySuccess(w http.ResponseWriter, response interface{}) {
	marshalled, err := json.Marshal(response)
	if err != nil {
		log.Errorf("Failed to marshal response: %v %v", response, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// write JSON response
	fmt.Fprintln(w, string(marshalled))
}

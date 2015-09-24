package router
import "net/http"

type Router interface {
	SetNotFound(http.Handler) error
	GetNotFound() http.Handler
	Handle(string, http.Handler) error
	Remove(string) error
	ServeHTTP(http.ResponseWriter, *http.Request)
}
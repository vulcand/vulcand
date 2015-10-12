package proxy

import (
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/oxy/roundrobin"
	"golang.org/x/net/websocket"
	"io"
	"net/http"
	"strings"
)

// WebsocketUpgrader is an HTTP middleware that detects websocket upgrade requests
// and establishes an HTTP connection via a chosen backend server
type WebsocketUpgrader struct {
	next     http.Handler
	rr       *roundrobin.RoundRobin
	f        *frontend
	wsServer *websocket.Server
}

// create the upgrader via a roundrobin and the expected next handler (if not websocket)
// also make sure a websocket server exists
func newWebsocketUpgrader(rr *roundrobin.RoundRobin, next http.Handler, f *frontend) *WebsocketUpgrader {
	wsServer := &websocket.Server{}
	wu := WebsocketUpgrader{
		next:     next,
		rr:       rr,
		f:        f,
		wsServer: wsServer,
	}
	wu.wsServer.Handler = websocket.Handler(wu.proxyWS)
	return &wu
}

// ServeHTTP waits for a websocket upgrade request and creates a TCP connection between
// the backend server and the frontend
func (u *WebsocketUpgrader) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// If request is websocket, serve with golang websocket server to do protocol handshake
	if strings.Join(req.Header["Upgrade"], "") == "websocket" {
		log.Infof("Websocket connected!")
		u.wsServer.ServeHTTP(w, req)
		//return
	}
	log.Infof("Websocket not connected!")

	u.next.ServeHTTP(w, req)
}

func (u *WebsocketUpgrader) proxyWS(ws *websocket.Conn) {
	url, err := u.rr.NextServer()
	if err != nil {
		log.Errorf("Can't round robin")
		return
	}
	rurl := url.String()
	if strings.HasPrefix(rurl, "http") {
		rurl = strings.Replace(rurl, "http", "ws", 1)
	}
	if strings.HasPrefix(rurl, "https") {
		rurl = strings.Replace(rurl, "https", "wss", 1)
	}
	path := rurl + ws.Request().URL.String()
	ws2, err := websocket.Dial(path, "", url.Host)
	if err != nil {
		log.Errorf("Couldn't connect to backend server: %v", err)
		return
	}
	defer ws2.Close()
	done := make(chan bool)
	go func() {
		io.Copy(ws, ws2)
		ws2.Close()
		done <- true
	}()
	go func() {
		io.Copy(ws2, ws)
		ws2.Close()
		done <- true
	}()
	<-done
	<-done
}

package sseserver

import (
	"net/http"
	"time"

	"github.com/mroth/sseserver/internal/debug"
)

const (
	DefaultConnMsgBufferSize = 256 // default per-connection message buffer size
)

type connection struct {
	r         *http.Request       // HTTP Request that spawned the connection
	w         http.ResponseWriter // HTTP ResponseWriter for connection
	created   time.Time           // Timestamp for when connection was opened
	send      chan []byte         // Buffered channel of outbound messages
	namespace string              // Conceptual "channel" SSE client is requesting
	msgsSent  uint64              // Msgs the connection has sent (all time)
}

func newConnection(w http.ResponseWriter, r *http.Request, namespace string, bufCount uint) *connection {
	return &connection{
		send:      make(chan []byte, bufCount),
		w:         w,
		r:         r,
		created:   time.Now(),
		namespace: namespace,
	}
}

// ConnectionStatus is snapshot of metadata describing the status of a connection.
type ConnectionStatus struct {
	Path       string `json:"request_path"`
	Namespace  string `json:"namespace"`
	Created    int64  `json:"created_at"`
	RemoteAddr string `json:"remote_addr"`
	UserAgent  string `json:"user_agent"`
	MsgsSent   uint64 `json:"msgs_sent"`
}

// Status returns a snaphot of status metadata for the connection.
//
// Primarily intended for logging and reporting.
func (c *connection) Status() ConnectionStatus {
	return ConnectionStatus{
		Path:       c.r.URL.Path,
		Namespace:  c.namespace,
		Created:    c.created.Unix(),
		RemoteAddr: c.r.RemoteAddr,
		UserAgent:  c.r.UserAgent(),
		MsgsSent:   c.msgsSent,
	}
}

// writer is the event loop that attempts to send all messages on the active
// http connection.  it will detect if the http connection is closed and autoexit.
// it will also exit if the connection's send channel is closed (indicating a shutdown)
func (c *connection) writer() {
	// set up a keepalive tickle to prevent connections from being closed by a timeout
	// any SSE line beginning with the colon will be ignored, so use that.
	// https://www.w3.org/TR/eventsource/#event-stream-interpretation
	keepaliveTickler := time.NewTicker(15 * time.Second)
	keepaliveMsg := []byte(":keepalive\n")
	defer keepaliveTickler.Stop()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok { // send chan was closed (hub told us we have nothing left to do)
				debug.Debug("hub told us to shut down")
				return
			}
			// otherwise write message out to client
			_, err := c.w.Write(msg)
			if err != nil {
				debug.Debug("error writing msg to client, closing")
				return
			}
			if f, ok := c.w.(http.Flusher); ok {
				f.Flush()
				c.msgsSent++
			}

		case <-keepaliveTickler.C:
			_, err := c.w.Write(keepaliveMsg)
			if err != nil {
				debug.Debug("error writing keepalive to client, closing")
				return
			}
			if f, ok := c.w.(http.Flusher); ok {
				f.Flush()
			}

		case <-c.r.Context().Done():
			debug.Debug("closer fired for conn")
			return
		}
	}
}

// connectionHandler returns a http.HandlerFunc that sets up a connections to
// register with the server hub.
func connectionHandler(s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// write headers
		headers := w.Header()
		headers.Set("Content-Type", "text/event-stream; charset=utf-8")
		headers.Set("Cache-Control", "no-cache")
		headers.Set("Connection", "keep-alive")
		if origin := s.conf.CORSAllowOrigin; origin != "" {
			headers.Set("Access-Control-Allow-Origin", origin)
		}

		// get namespace from URL path, init connection & register with hub
		namespace := r.URL.Path
		c := newConnection(w, r, namespace, s.conf.ConnMsgBufSize)
		s.hub.Register(c)
		defer func() {
			s.hub.Unregister(c)
		}()

		// start the connection's main broadcasting event loop
		c.writer()
	})
}

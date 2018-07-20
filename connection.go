package sseserver

import (
	"net/http"
	"time"

	"github.com/azer/debug"
)

const connBufSize = 256

type connection struct {
	r         *http.Request       // The HTTP request
	w         http.ResponseWriter // The HTTP response
	created   time.Time           // Timestamp for when connection was opened
	send      chan []byte         // Buffered channel of outbound messages
	namespace string              // Conceptual "channel" SSE client is requesting
	msgsSent  uint64              // Msgs the connection has sent (all time)
}

func newConnection(w http.ResponseWriter, r *http.Request, namespace string) *connection {
	return &connection{
		send:      make(chan []byte, connBufSize),
		w:         w,
		r:         r,
		created:   time.Now(),
		namespace: namespace,
	}
}

type connectionStatus struct {
	Path      string `json:"request_path"`
	Namespace string `json:"namespace"`
	Created   int64  `json:"created_at"`
	ClientIP  string `json:"client_ip"`
	UserAgent string `json:"user_agent"`
	MsgsSent  uint64 `json:"msgs_sent"`
}

func (c *connection) Status() connectionStatus {
	return connectionStatus{
		Path:      c.r.URL.Path,
		Namespace: c.namespace,
		Created:   c.created.Unix(),
		ClientIP:  c.r.RemoteAddr,
		UserAgent: c.r.UserAgent(),
		MsgsSent:  c.msgsSent,
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
			if !ok { // chan was closed
				// ...our hub told us we have nothing left to do
				debug.Debug("hub told us to shut down")
				return
			}
			// otherwise write message out to client
			_, err := c.w.Write(msg)
			if err != nil {
				debug.Debug("Error writing msg to client, closing")
				return
			}
			if f, ok := c.w.(http.Flusher); ok {
				f.Flush()
				c.msgsSent++
			}

		case <-keepaliveTickler.C:
			_, err := c.w.Write(keepaliveMsg)
			if err != nil {
				debug.Debug("Error writing keepalive to client, closing")
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

func connectionHandler(h *hub) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// write headers
		headers := w.Header()
		headers.Set("Access-Control-Allow-Origin", "*") // TODO: make optional
		headers.Set("Content-Type", "text/event-stream; charset=utf-8")
		headers.Set("Cache-Control", "no-cache")
		headers.Set("Connection", "keep-alive")
		headers.Set("Server", "mroth/sseserver")

		// get namespace from URL path, init connection & register with hub
		namespace := r.URL.Path
		c := newConnection(w, r, namespace)
		h.register <- c
		defer func() {
			h.unregister <- c
		}()

		// start the connection's main broadcasting event loop
		c.writer()
	})
}

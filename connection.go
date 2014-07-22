package sseserver

import (
	. "github.com/azer/debug"
	"log"
	"net/http"
	"time"
)

type connection struct {
	r         *http.Request       // The HTTP request
	w         http.ResponseWriter // The HTTP response
	created   time.Time           // Timestamp for when connection was opened
	send      chan []byte         // Buffered channel of outbound messages
	namespace string              // Conceptual "channel" SSE client is requesting
	msgsSent  uint64              // Msgs the connection has sent (all time)
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

func (c *connection) writer() {
	cn := c.w.(http.CloseNotifier)
	closer := cn.CloseNotify()

	for {
		select {
		case msg := <-c.send:
			_, err := c.w.Write(msg)
			if err != nil {
				break
			}
			if f, ok := c.w.(http.Flusher); ok {
				f.Flush()
				c.msgsSent++
			}
		case <-closer:
			Debug("closer fired for conn")
			return
		}
	}
}

func sseHandler(w http.ResponseWriter, r *http.Request, h *hub) {
	namespace := r.URL.Path[10:] // strip out the prepending "/subscribe"
	// TODO: we should do the above in a clever way so we work on any path

	// override RemoteAddr to trust proxy IP msgs if they exist
	// pattern taken from http://git.io/xDD3Mw
	ip := r.Header.Get("X-Real-IP")
	if ip == "" {
		ip = r.Header.Get("X-Forwarded-For")
	}
	if ip != "" {
		r.RemoteAddr = ip
	}

	log.Println("CONNECT\t", namespace, "\t", r.RemoteAddr)

	headers := w.Header()
	headers.Set("Access-Control-Allow-Origin", "*")
	headers.Set("Content-Type", "text/event-stream; charset=utf-8")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Connection", "keep-alive")
	headers.Set("Server", "emojitrack-gostreamer")

	c := &connection{
		send:      make(chan []byte, 256),
		w:         w,
		r:         r,
		created:   time.Now(),
		namespace: namespace,
	}
	h.register <- c

	defer func() {
		log.Println("DISCONNECT\t", namespace, "\t", r.RemoteAddr)
		h.unregister <- c
	}()

	c.writer()
}

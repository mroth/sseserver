package sseserver

import (
	"net/http"
	"sort"
	"time"
)

// Server is the primary interface to a SSE server.
//
// Exposes a receive-only chan `Broadcast`: any SSEMessage sent to this channel
// will be broadcast out to any connected clients subscribed to a namespace
// that matches the message.
//
// Server implements the http.Handler interface, and can be chained into
// existing HTTP routing muxes if desired.
type Server struct {
	Broadcast chan<- SSEMessage
	hub       *hub

	conf serverConfig
}

// serverConfig defines configurable options that can be customized for a Server.
type serverConfig struct {
	CORSAllowOrigin string // Access-Control-Allow-Origin header value (dont send header if blank)
	ConnBufSize     uint   // message buffer count for new connections (TODO: add option&tests)
}

// NewServer creates a new Server with optional ServerOptions for configuration.
func NewServer(opts ...ServerOption) (*Server, error) {
	hub := newHub()
	s := &Server{
		// re-export just the hub's broadcast chan to public(will be typecast to receive-only)
		Broadcast: hub.broadcast,
		hub:       hub,

		conf: serverConfig{
			ConnBufSize: defaultConnBufSize,
		},
	}

	// set configuration from provided options
	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, err
		}
	}

	// start up our actual internal connection hub
	// which we keep in the server struct as private
	s.hub.Start()

	return s, nil
}

// ServerOptions defines a set of high-level user options that can be customized
type ServerOption func(s *Server) error

// WithCORSAllowOrigin sets the Access-Control-Allow-Origin header value to origin.
// If set to the zero value (""), the header will not be sent.
//
// If you want to allow connections from browsers at any origin, set to "*".
//
// See https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Origin.
func WithCORSAllowOrigin(origin string) ServerOption {
	return func(s *Server) error {
		s.conf.CORSAllowOrigin = origin
		return nil
	}
	// TODO: update examples/docs to use this
}

// ServeHTTP implements the http.Handler interface
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mux := http.NewServeMux()
	mux.Handle(
		"/subscribe/",
		http.StripPrefix("/subscribe", connectionHandler(s)),
	)
	mux.ServeHTTP(w, r)
}

// CONSIDER: func (s *Server) Drain() for allowing active connections to drain. This
// will require some thinking since SSE connections can be *very* long lived.
// Handling this via k8s may work by default looking at active HTTP connections.

// Shutdown a server gracefully, closing active connections.
//
// Currently, this returns immediately, and does not wait for connections to be
// closed in the background.
func (s *Server) Shutdown() {
	s.hub.Shutdown()
}

// ServerStatus is snapshot of metadata describing the status of a Server.
type ServerStatus struct {
	Status      string             `json:"status"`
	Reported    int64              `json:"reported_at"`
	StartupTime int64              `json:"startup_time"`
	SentMsgs    uint64             `json:"msgs_broadcast"`
	Connections []ConnectionStatus `json:"connections"`
}

// Status returns a snaphot of status metadata for the Server.
//
// Primarily intended for logging and reporting.
func (s *Server) Status() ServerStatus {
	cl := make([]ConnectionStatus, 0, len(s.hub.connections))
	for k := range s.hub.connections {
		cl = append(cl, k.Status())
	}
	// sort by age of connection
	sort.Slice(cl, func(i, j int) bool {
		return cl[i].Created < cl[j].Created
	})

	return ServerStatus{
		Status:      "OK",
		Reported:    time.Now().Unix(),
		StartupTime: s.hub.startupTime.Unix(),
		SentMsgs:    s.hub.sentMsgs,
		Connections: cl,
	}
}

package sseserver

import (
	"net/http"
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
	Options   ServerOptions
	hub       *hub
}

// ServerOptions defines a set of high-level user options that can be customized
// for a Server.
type ServerOptions struct {
	DisableAdminEndpoints bool // DEPRECATED: admin endpoints no longer enabled by default
	// DisallowRootSubscribe bool // TODO: possibly consider this option?
}

// NewServer creates a new Server and returns a reference to it.
func NewServer() *Server {
	s := Server{
		hub: newHub(),
	}

	// start up our actual internal connection hub
	// which we keep in the server struct as private
	s.hub.Start()

	// then re-export just the hub's broadcast chan to public
	// (will be typecast to receive-only)
	s.Broadcast = s.hub.broadcast

	return &s
}

// ServeHTTP implements the http.Handler interface
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mux := http.NewServeMux()
	mux.Handle(
		"/subscribe/",
		http.StripPrefix("/subscribe", connectionHandler(s.hub)),
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

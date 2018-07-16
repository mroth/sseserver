package sseserver

import (
	"log"
	"net/http"

	"github.com/azer/debug"
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
	DisableAdminEndpoints bool // disables the "/admin" status endpoints
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
		http.StripPrefix("/subscribe", newConnectionHandler(s.hub)),
	)
	mux.Handle(
		"/admin/",
		adminHandler(s),
	)
	mux.ServeHTTP(w, r)
}

// Serve is a convenience method to begin serving connections on specified address.
//
// This method blocks forever, as it is basically a convenience wrapper around
// http.ListenAndServe(addr, self).
func (s *Server) Serve(addr string) {
	debug.Debug("Starting server on addr " + addr)
	if err := http.ListenAndServe(addr, s); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

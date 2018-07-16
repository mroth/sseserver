package sseserver

import (
	"log"
	"net/http"

	"github.com/azer/debug"
)

// Server is the interface to a SSE server.
//
// Exposes a send-only chan `broadcast`, any SSEMessage sent to this channel
// will be broadcast out to any connected clients subscribed to a namespace
// that matches the message.
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

	// set up the public interface
	var s = Server{}

	// start up our actual internal connection hub
	// which we keep in the server struct as private
	s.hub = newHub()
	s.hub.Start()

	// expose just the broadcast chanel to public
	// will be typecast to send-only
	s.Broadcast = s.hub.broadcast

	// return handle
	return &s
}

// Serve begins serving connections on specified address.
//
// This method blocks forever, as it is basically a setup wrapper around
// http.ListenAndServe()
func (s *Server) Serve(addr string) {

	// set up routes.
	http.Handle("/subscribe/", newConnectionHandler(s.hub))
	if !s.Options.DisableAdminEndpoints {
		http.HandleFunc("/admin", adminStatusHTMLHandler)
		http.HandleFunc("/admin/status.json", func(w http.ResponseWriter, r *http.Request) {
			adminStatusDataHandler(w, r, s)
		})
	}

	// actually start the HTTP server
	debug.Debug("Starting server on addr " + addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

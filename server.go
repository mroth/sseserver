package sseserver

import (
	. "github.com/azer/debug"
	"log"
	"net/http"
)

// Interface to a SSE server.
//
// Exposes a send-only chan `broadcast`, any SSEMessage sent to this channel
// will be broadcast out to any connected clients subscribed to a namespace
// that matches the message.
type Server struct {
	Broadcast chan<- SSEMessage
	hub       *hub
}

// Creates a new Server and returns a reference to it.
func NewServer() *Server {

	// set up the public interface
	var s = Server{}

	// start up our actual internal connection hub
	// which we keep in the server struct as private
	var h = newHub()
	s.hub = h
	go h.run()

	// expose just the broadcast chanel to public
	// will be typecast to send-only
	s.Broadcast = h.broadcast

	// return handle
	return &s
}

// Begin serving connections on specified address.
//
// This method blocks forever, as it is basically a setup wrapper around
// http.ListenAndServe()
func (s *Server) Serve(addr string) {

	// set up routes.
	// use an anonymous function for closure in order to pass value to handler
	// https://groups.google.com/forum/#!topic/golang-nuts/SGn1gd290zI
	http.HandleFunc("/subscribe/", func(w http.ResponseWriter, r *http.Request) {
		sseHandler(w, r, s.hub)
	})

	http.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "sseserver/views/admin.html")
	})

	http.HandleFunc("/admin/status.json", func(w http.ResponseWriter, r *http.Request) {
		adminStatusDataHandler(w, r, s.hub)
	})

	// actually start the HTTP server
	Debug("Starting server on addr " + addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

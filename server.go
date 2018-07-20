package sseserver

import (
	"log"
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
		http.StripPrefix("/subscribe", connectionHandler(s.hub)),
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
//
// It also implements basic request logging to STDOUT.
//
// If you want to do something more sophisticated, you should not use this method,
// but rather just build your own HTTP routing/middleware chain around Server which
// implements the standard http.Handler interface.
func (s *Server) Serve(addr string) {
	log.Println("Starting server on addr " + addr)
	handler := ProxyRemoteAddrHandler(requestLogger(s))
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

// ProxyRemoteAddrHandler is HTTP middleware to determine the actual RemoteAddr
// of a http.Request when your server sits behind a proxy or load balancer.
//
// When utilized, the value of RemoteAddr will be overridden based on the
// X-Real-IP or X-Forwarded-For HTTP header, which can be a comma separated list
// of IPs.
//
// See http://httpd.apache.org/docs/2.2/mod/mod_proxy.html#x-headers for
// details.
//
// Based on http://git.io/xDD3Mw
func ProxyRemoteAddrHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.Header.Get("X-Real-IP")
		if ip == "" {
			ip = r.Header.Get("X-Forwarded-For")
		}
		if ip != "" {
			r.RemoteAddr = ip
		}
		next.ServeHTTP(w, r)
	})
}

// requestLogger is a sample of integrating logging via HTTP middleware.
//
// Utilized in our Serve() convenience function. Note that due to the long
// connection time of SSE requests you likely want to log disconnection
// separately.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("CONNECT\t", r.URL.Path, "\t", r.RemoteAddr)
		next.ServeHTTP(w, r)
		log.Println("DISCONNECT\t", r.URL.Path, "\t", r.RemoteAddr)
	})
}

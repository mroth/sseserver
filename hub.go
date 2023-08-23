package sseserver

import (
	"fmt"
	"strings"
	"time"

	"github.com/mroth/sseserver/internal/debug"
)

// A hub keeps track of all the active client connections, and handles
// broadcasting messages out to those connections that match the appropriate
// namespace.
//
// The hub is effectively the "heart" of a Server, but is kept private to hide
// implementation detais.
type hub struct {
	broadcast  chan SSEMessage  // Inbound messages to propagate out
	register   chan *connection // Register requests from the connections
	unregister chan *connection // Unregister requests from connections
	shutdown   chan bool        // Internal chan to handle shutdown notification

	// INTERNAL STATE
	connections map[*connection]bool // Registered connections.

	// METADATA
	sentMsgs    uint64    // Msgs broadcast since startup
	startupTime time.Time // Time hub was created
}

func newHub() *hub {
	return &hub{
		broadcast:   make(chan SSEMessage),
		register:    make(chan *connection),
		unregister:  make(chan *connection),
		shutdown:    make(chan bool),
		connections: make(map[*connection]bool),
		startupTime: time.Now(),
	}
}

// Shutdown method for cancellation of hub run loop.
//
// right now we are only using this in tests, in the future we may want to be
// able to more gracefully shut down a Server in production as well, but...
func (h *hub) Shutdown() {
	h.shutdown <- true
}

// Start begins the main run loop for a hub in a background go func.
func (h *hub) Start() {
	// TODO: guard against being started multiple times?
	go h.run()
}

// run is the internal event loop for a hub.
//
// Developer note: the entire concurrency safety model for a hub is handled via
// select over control channels. It feels elegant as it means a mutex is not
// required, but please be certain to not call any of the helper methods outside
// of this event loop, otherwise safety is not guaranteed.
func (h *hub) run() {
	for {
		select {
		case <-h.shutdown:
			h._shutdownAllConnections()
			return
		case c := <-h.register:
			h._registerConn(c)
		case c := <-h.unregister:
			h._unregisterConn(c)
		case msg := <-h.broadcast:
			h._broadcastMessage(msg)
		}
	}
}

/*
README: INTERNAL EVENT LOOP METHODS

The following methods are safe only to be called from above (*hub).run() loop.

Potential future safety: if this codebase is ever maintained by multiple
developers, it may be safest to move hub into its own internal package, to make
it impossible to accidentally abuse these.
*/

// _registerConn add a connection to the hub.
//
// INTERNAL EVENT LOOP METHOD.
func (h *hub) _registerConn(c *connection) {
	debug.Debug("new connection being registered for " + c.namespace)
	h.connections[c] = true
}

// _unregisterConn removes a connection from the hub.
// Safe to call multiple times with the same connection.
//
// INTERNAL EVENT LOOP METHOD.
func (h *hub) _unregisterConn(c *connection) {
	debug.Debug("connection told us to unregister for " + c.namespace)
	delete(h.connections, c)
}

// _shutdownAllConnections calls _shutdownConn on all registered connections.
//
// INTERNAL EVENT LOOP METHOD.
func (h *hub) _shutdownAllConnections() {
	debug.Debug(fmt.Sprintf("hub shutdown requested, cancelling %d connections...", len(h.connections)))
	for c := range h.connections {
		h._shutdownConn(c)
	}
	debug.Debug("...All connections cancelled, shutting down now.")
}

// _shutdownConn first unregisters the connection from the hub and then tells it
// to shutdown by closing the connection's send channel.
//
// SAFETY: must only be called once for a given connection to avoid panic!
func (h *hub) _shutdownConn(c *connection) {
	// for maximum safety, ALWAYS unregister a connection from the hub prior to
	// shutting it down, as we want no possibility of a send on closed channel
	// panic.
	h._unregisterConn(c)

	// close the connection's send channel, which will cause it to exit its
	// event loop and return to the HTTP handler.
	close(c.send)
}

// _broadcastMessage to all connections subscribed to the msg namespace.
//
// If this fails due to any connection having a full message buffer, attempts to
// terminate that connection.
//
// INTERNAL EVENT LOOP METHOD.
func (h *hub) _broadcastMessage(msg SSEMessage) {
	h.sentMsgs++

	formattedMsg := msg.sseFormat()
	for c := range h.connections {
		if strings.HasPrefix(msg.Namespace, c.namespace) {
			select {
			case c.send <- formattedMsg:
			default:
				debug.Debug("cant pass to a connection send chan, buffer is full -- kill it with fire")
				h._shutdownConn(c)
				/*
					we are already closing the send channel, in *theory* shouldn't the
					connection clean up? I guess possible it doesnt if its deadlocked or
					something... is it?

					closing the send channel will result in our handleFunc exiting, which Go
					treats as meaning we are done with the connection... but what if it's wedged?

					TODO: investigate using panic in a http.Handler, to absolutely force

					we want to make sure to always close the HTTP connection though,
					so server can never fill up max num of open sockets.
				*/
			}
		}
	}
}

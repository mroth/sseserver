package sseserver

import (
	"fmt"
	"time"

	"github.com/azer/debug"
	"github.com/mroth/sseserver/router"
)

// A hub keeps track of all the active client connections, and handles
// broadcasting messages out to those connections that match the appropriate
// namespace.
//
// The hub is effectively the "heart" of a Server, but is kept private to hide
// implementation detais.
type hub struct {
	broadcast   chan SSEMessage      // Inbound messages to propagate out.
	connections map[*connection]bool // Registered connections.
	router      router.Node          // Possibly more efficient lookups!
	register    chan *connection     // Register requests from the connections.
	unregister  chan *connection     // Unregister requests from connections.
	shutdown    chan bool            // Internal chan to handle shutdown notification
	sentMsgs    uint64               // Msgs broadcast since startup
	startupTime time.Time            // Time hub was created
}

func newHub() *hub {
	return &hub{
		broadcast:   make(chan SSEMessage),
		connections: make(map[*connection]bool),
		router:      router.New(),
		register:    make(chan *connection),
		unregister:  make(chan *connection),
		shutdown:    make(chan bool),
		startupTime: time.Now(),
	}
}

// Shutdown method for cancellation of hub run loop.
//
// right now we are only using this in tests, in the future we may want to be
// able to more gracefully shut down a Server in production as well, but...
//
// TODO: need to do some thinking before exposing this interface more broadly.
func (h *hub) Shutdown() {
	h.shutdown <- true
}

// Start begins the main run loop for a hub in a background go func.
func (h *hub) Start() {
	go h.run()
}

func (h *hub) run() {
	for {
		select {
		case <-h.shutdown:
			debug.Debug(fmt.Sprintf("hub shutdown requested, cancelling %d connections...", len(h.connections)))
			for c := range h.connections {
				h._shutdownConn(c)
			}
			debug.Debug("...All connections cancelled, shutting down now.")
			return
		case c := <-h.register:
			h._registerConn(c)
		case c := <-h.unregister:
			h._unregisterConn(c)
		case msg := <-h.broadcast:
			h.sentMsgs++
			h._broadcastMessage(msg)
		}
	}
}

// internal method, adds client to the hub
func (h *hub) _registerConn(c *connection) {
	debug.Debug("new connection being registered for " + c.namespace)
	h.connections[c] = true
	h.router.InsertAt(router.NS(c.namespace), c)
}

// internal method, removes that client from the hub and tells it to shutdown
// _unregister is safe to call multiple times with the same connection
func (h *hub) _unregisterConn(c *connection) {
	debug.Debug("connection told us to unregister for " + c.namespace)
	delete(h.connections, c)

	routerNode, err := h.router.Find(router.NS(c.namespace))
	if err == nil {
		routerNode.Remove(c)
	}
}

// internal method, removes that client from the hub and tells it to shutdown
// must only be called once for a given connection to avoid panic!
func (h *hub) _shutdownConn(c *connection) {
	// for maximum safety, ALWAYS unregister a connection from the hub prior to
	// shutting it down, as we want no possibility of a send on closed channel
	// panic.
	h._unregisterConn(c)
	// close the connection's send channel, which will cause it to exit its
	// event loop and return to the HTTP handler.
	close(c.send)
}

// internal method, broadcast a message to all matching clients if this fails
// due to any client having a full send buffer,
func (h *hub) _broadcastMessage(msg SSEMessage) {
	formattedMsg := msg.sseFormat()
	ns := router.NS(msg.Namespace)
	matchedNode := h.router.FindOrCreate(ns)
	if matchedNode != nil {
		matchedNode.ForEachAscendingValue(func(v router.Value) {
			c := v.(*connection)
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
		})
	}

}

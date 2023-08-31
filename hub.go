package sseserver

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mroth/sseserver/internal/debug"
)

// A hub keeps track of all the active client connections, and handles
// broadcasting messages out to those connections that match the appropriate
// namespace.
//
// The hub is effectively the "heart" of a Server, but is kept private to hide
// implementation details.
//
// To create a hub, use newHub() to ensure it is initialized properly.
//
// A hub should always be cancelled via Shutdown() when it is no longer needed,
// in order to avoid leaking a goroutine.
type hub struct {
	connections map[*connection]struct{}
	mu          sync.Mutex
	sentMsgs    uint64    // Msgs broadcast since startup
	startupTime time.Time // Time hub was created
}

func newHub() *hub {
	return &hub{
		connections: make(map[*connection]struct{}),
		startupTime: time.Now(),
	}
}

// Broadcast a SSEMessage to all connections subscribed to the msg namespace.
//
// If this fails due to any connection having a full message buffer, attempts to
// terminate that connection.
func (h *hub) Broadcast(msg SSEMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()

	formattedMsg := msg.sseFormat()
	for c := range h.connections {
		if strings.HasPrefix(msg.Namespace, c.namespace) {
			select {
			case c.send <- formattedMsg:
			default:
				debug.Debug("cant pass to a connection send chan, buffer is full -- kill it with fire")
				h._shutdownConn(c)
				// we are already closing the send channel, in *theory* shouldn't the
				// connection clean up? I guess possible it doesnt if its deadlocked or
				// something... is it?
				//
				// closing the send channel will result in our handleFunc exiting, which Go
				// treats as meaning we are done with the connection... but what if it's wedged?
				//
				// TODO: investigate using panic in a http.Handler, to absolutely force
				//
				// we want to make sure to always close the HTTP connection though,
				// so server can never fill up max num of open sockets.
			}
		}
	}

	h.sentMsgs++
}

// Register a connection to receive broadcast messages
func (h *hub) Register(c *connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	debug.Debug("new connection being registered for " + c.namespace)
	h.connections[c] = struct{}{}
}

// Unregister a connection to receive broadcast messages
func (h *hub) Unregister(c *connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	debug.Debug("connection told us to unregister for " + c.namespace)
	h._unregister(c)
}

func (h *hub) _unregister(c *connection) {
	delete(h.connections, c)
}

// Shutdown method for cancellation of hub run loop.
//
// TODO: consider taking a ctx similar to http.Server
func (h *hub) Shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	debug.Debug(fmt.Sprintf("hub shutdown requested, cancelling %d connections...", len(h.connections)))
	for c := range h.connections {
		h._shutdownConn(c)
	}
	debug.Debug("...All connections cancelled, shutting down now.")

	// TODO: currently we exit after signalling all connections no more messages
	// to come, and eventloop exits, but do not wait for connections to complete
	// writing to clients or closing (e.g. connections still have active
	// background goroutines) -- consider also forcing connections closed after
	// a certain point? likely utilize passed ctx as deadline to kill with fire.
}

// _shutdownConn first unregisters the connection from the hub and then tells it
// to shutdown by closing the connection's send channel.
//
// SAFETY: must only be called once for a given connection to avoid panic!
func (h *hub) _shutdownConn(c *connection) {
	// for maximum safety, ALWAYS unregister a connection from the hub prior to
	// shutting it down, as we want no possibility of a send on closed channel
	// panic.
	h._unregister(c)

	// close the connection's send channel, which will cause it to exit its
	// event loop and return to the HTTP handler.
	close(c.send)
}

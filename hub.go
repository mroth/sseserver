package sseserver

import (
	"context"
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
	broadcastCh  chan SSEMessage    // Inbound messages to propagate out
	registerCh   chan *connection   // Register requests from the connections
	unregisterCh chan *connection   // Unregister requests from connections
	cancel       context.CancelFunc // CancelFunc to handle shutdown notification

	// INTERNAL STATE
	connections map[*connection]struct{} // Registered connections
	running     sync.WaitGroup           // Track eventloop goroutine run state

	// METADATA
	sentMsgs    uint64    // Msgs broadcast since startup
	startupTime time.Time // Time hub was created
}

// newHub initializes a new hub and starts its event loop running
func newHub() *hub {
	ctx, cancel := context.WithCancel(context.Background())
	h := &hub{
		broadcastCh:  make(chan SSEMessage),
		registerCh:   make(chan *connection),
		unregisterCh: make(chan *connection),
		cancel:       cancel,
		connections:  make(map[*connection]struct{}),
		startupTime:  time.Now(),
	}

	h.running.Add(1)
	go func() {
		defer h.running.Done()
		h.eventloop(ctx)
	}()

	return h
}

func (h *hub) Broadcast(msg SSEMessage) {
	h.broadcastCh <- msg
}

func (h *hub) Register(c *connection) {
	h.registerCh <- c
}

func (h *hub) Unregister(c *connection) {
	h.unregisterCh <- c
}

// Shutdown method for cancellation of hub run loop.
//
// TODO: consider taking a ctx similar to http.Server
func (h *hub) Shutdown() {
	h.cancel()
	h.running.Wait()
}

// internal event loop for a hub.
//
// Developer note: the entire concurrency safety model for a hub is handled via
// select over control channels. It feels elegant as it means a mutex is not
// required, but please be certain to not call any of the helper methods outside
// of this event loop, otherwise safety is not guaranteed.
func (h *hub) eventloop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h._shutdownAllConnections()
			return
		case c := <-h.registerCh:
			h._registerConn(c)
		case c := <-h.unregisterCh:
			h._unregisterConn(c)
		case msg := <-h.broadcastCh:
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
	h.connections[c] = struct{}{}
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

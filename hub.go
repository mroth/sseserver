package sseserver

import (
	"strings"
	"time"

	"github.com/azer/debug"
)

// A connection hub keeps track of all the active client connections, and
// handles broadcasting messages out to those connections that match the
// appropriate namespace.
type hub struct {
	broadcast   chan SSEMessage      // Inbound messages to propogate out.
	connections map[*connection]bool // Registered connections.
	register    chan *connection     // Register requests from the connections.
	unregister  chan *connection     // Unregister requests from connections.
	sentMsgs    uint64               // Msgs broadcast since startup
	startupTime time.Time            // Time hub was created
}

func newHub() *hub {
	return &hub{
		broadcast:   make(chan SSEMessage),
		connections: make(map[*connection]bool),
		register:    make(chan *connection),
		unregister:  make(chan *connection),
		startupTime: time.Now(),
	}
}

func (h *hub) run() {
	for {
		select {
		case c := <-h.register:
			debug.Debug("new connection being registered for " + c.namespace)
			h.connections[c] = true
		case c := <-h.unregister:
			debug.Debug("connection told us to unregister for " + c.namespace)
			delete(h.connections, c)
			close(c.send)
		case msg := <-h.broadcast:
			h.sentMsgs++
			formattedMsg := msg.sseFormat()
			for c := range h.connections {
				if strings.HasPrefix(msg.Namespace, c.namespace) {
					select {
					case c.send <- formattedMsg:
					default:
						debug.Debug("cant pass to a connection send chan, buffer is full -- kill it with fire")
						delete(h.connections, c)
						close(c.send)
						/* TODO: figure out what to do here...
						we are already closing the send channel, in *theory* shouldn't the
						connection clean up? I guess possible it doesnt if its deadlocked or
						something... is it?

						we want to make sure to always close the HTTP connection though,
						so server can never fill up max num of open sockets.
						*/
					}
				}
			}
		}
	}
}

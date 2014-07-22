package sseserver

import (
	"fmt"
)

// SSEMessage is a message suitable for sending over a Server-Sent Event stream.
//
// Note: Namespace is not part of the SSE spec, it is merely used internally to
// map a message to the appropriate HTTP virtual endpoint.
//
type SSEMessage struct {
	Event     string // event scope for the message [optional]
	Data      []byte // message payload
	Namespace string // namespace for msg, matches to client subscriptions
}

// sseFormat is the formatted bytestring for a SSE message, ready to be sent.
func (msg SSEMessage) sseFormat() []byte {
	if msg.Event != "" {
		return []byte(fmt.Sprintf("event:%s\ndata:%s\n\n", msg.Event, msg.Data))
	} else {
		return []byte(fmt.Sprintf("data:%s\n\n", msg.Data))
	}
}

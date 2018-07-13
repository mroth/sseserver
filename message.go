package sseserver

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
	// var b []byte but add initial capacity of length of keys, fields, and linebreaks.
	// will be +1 byte wasted capacity if in nonevented case, does that really matter?
	// cost of the comparison branch before allocation may outweight the 1 byte saving.
	b := make([]byte, 0, 6+5+len(msg.Event)+len(msg.Data)+3)
	if msg.Event != "" {
		b = append(b, "event:"...)
		b = append(b, msg.Event...)
		b = append(b, '\n')
	}
	b = append(b, "data:"...)
	b = append(b, msg.Data...)
	b = append(b, '\n', '\n')
	return b
}

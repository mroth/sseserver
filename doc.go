/*
Package ssserver implements a reference Server-Sent Events server, suitable
for streaming unidirectional messages over HTTP to web browsers.

This implementation also adds easy namespacing so that clients can subscribe to
only a specific subset of messages.


Server-Sent Events

For more information on the SSE format itself, check out this fairly
comprehensive article:
http://www.html5rocks.com/en/tutorials/eventsource/basics/

Note that the implementation of SSE in this server intentionally does not implement
message IDs.  If you want them, let me know.


Namespacing

The server opens a HTTP endpoint at /subscribe that will accept connections on
any suitable child path.  The remainder of the path is a virtual endpoint
represents the "namespace" for the client connection.  For example:

	HTTP GET /subscribe/new          	// client subscribed to "/status" namespace
	HTTP GET /subscribe/status/243a7 	// client subscribed to "/status/243a7" namespace

SSEMessages broadcast via the server are only delivered to clients subscribed
to the endpoint matching their namespace.  (Note that presently namespaces must
be exact matches, however if there is demand for wildcards let me know as it
would be fairly simple to implement.)

*/
package sseserver

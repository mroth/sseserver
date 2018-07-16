/*
Package sseserver implements a reference Server-Sent Events server, suitable for
streaming unidirectional messages over HTTP to web browsers.

This implementation also adds easy namespacing so that clients can subscribe to
only a specific subset of messages.


Server-Sent Events

For more information on the SSE format itself, check out this fairly
comprehensive article:
http://www.html5rocks.com/en/tutorials/eventsource/basics/

Note that the implementation of SSE in this server intentionally does not
implement message IDs.


Namespacing

The server opens a HTTP endpoint at /subscribe/:namespace that will accept
connections on any suitable child path.  The remainder of the path is a virtual
endpoint represents the "namespace" for the client connection.  For example:

    HTTP GET /subscribe/pets        // client subscribed to "/pets" namespace
    HTTP GET /subscribe/pets/cats   // client subscribed to "/pets/cats" namespace
    HTTP GET /subscribe/pets/dogs   // client subscribed to "/pets/dogs" namespace

SSEMessages broadcast via the server are only delivered to clients subscribed to
the endpoint matching their namespace. Namespaces are hierarchical, subscribing
to the "parent" endpoint would receive all messages broadcast to all child
namespaces as well. E.g. in the previous example, a subscription to "/pets"
would receive all messages broadcast to both the dogs and cats namespaces as
well.
*/
package sseserver

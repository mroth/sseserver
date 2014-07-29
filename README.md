package sseserver
=================

An encapsulated high-performance Server-Sent Events endpoint server for Go with 
advanced namespacing support.

Abstracts multiple namespaced HTTP endpoints so that clients can subscribe to
messages on on a specific topic.  Should be thread-safe, so you can run multiple
instances concurrently (for example, on different ports) if needed.

Designed for high throughput as primary performance consideration. In my
preliminary benchmarking this can handle ~100K/sec messages broadcast across
~1000 open HTTP connections on a 3.4GHz Intel Core i7 (using a single  core,
e.g. with `GOMAXPROCS=1`).  There still remains quite a bit of optimization to
be done so it should get faster if needed.

This currently powers the streaming service for
[Emojitracker](http://emojitracker.com) in production, where it has routinely
handled dispatching hundreds of messages per second to thousands of clients
simultaneously, on a single Heroku dyno. (The previous NodeJS solution required
dozens of dynos to handle the same load.)


Why SSE vs Websockets?
----------------------

Words will go here.  In the meantime, there is a [semi-decent discussion on StackOverflow](http://stackoverflow.com/questions/5195452/websockets-vs-server-sent-events-eventsource) about the topic.


API
---
See the [godocs](https://godoc.org/github.com/mroth/sseserver).


Admin Page
----------
By default, an admin status page is available for easy monitoring:

![screenshot](http://f.cl.ly/items/1v2X1k342K3p0K1O2x0B/ssestreamer-admin.png)


Example Usage
-------------
A simple Go program utilizing this package:

```go
package main

import (
    "github.com/mroth/sseserver"
    "time"
)

func main() {
    s := sseserver.NewServer() // create a server instance

    // broadcast the time every second to the "/time" namespace
    go func() {
        ticker := time.Tick(time.Duration(1 * time.Second))
        for {
            // wait for the ticker to fire
            t := <-ticker
            // create the message payload, can be any []byte value
            data := []byte(t.Format("3:04:05 pm (MST)"))
            // send a message without an event on the "/time" namespace
            s.Broadcast <- sseserver.SSEMessage{"", data, "/time"}
        }
    }()

    // simulate sending some scoped events on the "/pets" namespace
    go func() {
        time.Sleep(time.Duration(5 * time.Second))
        s.Broadcast <- sseserver.SSEMessage{"new-dog", []byte("Corgi"), "/pets"}
        s.Broadcast <- sseserver.SSEMessage{"new-cat", []byte("Persian"), "/pets"}
        time.Sleep(time.Duration(1 * time.Second))
        s.Broadcast <- sseserver.SSEMessage{"new-dog", []byte("Terrier"), "/pets"}
        s.Broadcast <- sseserver.SSEMessage{"new-dog", []byte("Dauchsand"), "/pets"}
        time.Sleep(time.Duration(2 * time.Second))
        s.Broadcast <- sseserver.SSEMessage{"new-cat", []byte("LOLcat"), "/pets"}
    }()

    s.Serve(":8001") // bind to port and beging serving connections
}

```

All these event namespaces are exposed via HTTP endpoint in the
`/subscribe/:namespace` route.

On the client, we can easily connect to those endpoints using built-in functions in JS:
```js
// connect to an event source endpoint and print results
es1 = new EventSource("http://localhost:8001/subscribe/time");
es1.onmessage = function(event) {
    console.log("TICK! The time is currently: " + event.data);
};

// connect to a different event source endpoint and register event handlers
es2 = new EventSource("http://localhost:8001/subscribe/pets")
es2.addEventListener("new-dog", function(event) {
    console.log("WOOF! Hello " + event.data);
}, false);
es2.addEventListener("new-cat", function(event) {
    console.log("MEOW! Hello " + event.data);
}, false);
```

Which when connecting to the server would yield results:

    TICK! The time is currently: 6:07:17 pm (EDT)
    TICK! The time is currently: 6:07:18 pm (EDT)
    TICK! The time is currently: 6:07:19 pm (EDT)
    TICK! The time is currently: 6:07:20 pm (EDT)
    WOOF! Hello Corgi
    MEOW! Hello Persian
    TICK! The time is currently: 6:07:21 pm (EDT)
    WOOF! Hello Terrier
    WOOF! Hello Dauchsand
    TICK! The time is currently: 6:07:22 pm (EDT)
    TICK! The time is currently: 6:07:23 pm (EDT)
    MEOW! Hello LOLcat
    TICK! The time is currently: 6:07:24 pm (EDT)  


Of course you could easily send JSON objects in the data payload instead, and
most likely will be doing this often.

Another advantage of the SSE protocol is that the wire-format is so simple.
Unlike WebSockets, we can connect with `curl` to an endpoint directly and just
read what's going on:

```bash
$ curl http://localhost:8001/subscribe/pets
event:new-dog
data:Corgi

event:new-cat
data:Persian

event:new-dog
data:Terrier

event:new-dog
data:Dauchsand

event:new-cat
data:LOLcat
```

Yep, it's that simple.

Namespace Nesting
-----------------
A client can subscribe to a parent namespace. E.g. a subscription
to `/pets` will receive messages broadcast to both `/pets/dogs` and
`/pets/cats`.

Acknowledgements
----------------
A lot of the initial ideas for handling the connection hub in idiomatic Go originally
came from cribbing from Gary Burd's [go-websocket-chat][1], but has now been
modified to work with SSE instead of Websockets and to be encapsulated in a
thread-safe way.

[1]: http://gary.burd.info/go-websocket-chat

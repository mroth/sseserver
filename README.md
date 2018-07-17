# sseserver :surfing_man:

[![Build Status](https://travis-ci.org/mroth/sseserver.svg?branch=master)](https://travis-ci.org/mroth/sseserver)
[![GoDoc](https://godoc.org/github.com/mroth/sseserver?status.svg)](https://godoc.org/github.com/mroth/sseserver)

> A high performance and thread-safe Server-Sent Events server for Go with
_hierarchical namespacing_ support.

This library has powered the streaming API endpoint for
:dizzy: [Emojitracker](https://emojitracker.com) in production since 2014, where
it routinely handles dispatching hundreds of messages per second to thousands of
simultaneous clients, on a single Heroku dyno.

## Introduction

### Hierarchical Namespaced Channels*

A client can subscribe to channels reflecting the content they are interested
in. For example, say you are broadcasting events to the namespaces `/pets/cats`
and `/pets/dogs`. A client could subscribe to the parent channel `/pets` in the
hierarchy and receive all messages for either.

In **sseserver**, channels have infinite depth and are automatically created on
the fly with zero setup -- just broadcast and it works.

<small>(*There's probably a more accurate term for this.  If you know it, let me
know.)</small>

### Performance

Designed for high throughput as primary performance consideration. In my
preliminary benchmarking (on `v1.0`, circa 2014) this can handle ~100K/sec
messages broadcast across ~1000 open HTTP connections on a 3.4GHz Intel Core i7
(using a single  core, e.g. with `GOMAXPROCS=1`).  There still remains quite a
bit of optimization to be done so it should be able to get faster if needed.

### SSE vs Websockets

SSE is the oft-overlooked *uni-directional* cousin of websockets. Being "just
HTTP" it has some benefits:

- Trvially easier to understand, plaintext format.
  <small>Can be debugged by a human with `curl`.</small>
- Supported in most major browsers for a long time now.
  <small>Everything except IE/Edge, but an easy polyfill!</small>
- Built-in standard support for automatic reconnection, event binding, etc.
- Works with HTTP/2.

See also Smashing Magazine: ["Using SSE Instead Of WebSockets For Unidirectional
Data Flow Over HTTP/2"][1].

[1]: https://www.smashingmagazine.com/2018/02/sse-websockets-data-flow-http2/

## Documentation

[![GoDoc](https://godoc.org/github.com/mroth/sseserver?status.svg)](https://godoc.org/github.com/mroth/sseserver)

### Namespaces are URLs

For clients, no need to think about protocol. To subscribe to one of the above
namespaces from the previous example, just connect to `http://$SERVER/pets/dogs`.
Done.

### Example Usage

A simple Go program utilizing this package:

```go
package main

import (
    "time"
    "github.com/mroth/sseserver"
)

func main() {
    s := sseserver.NewServer() // create a new server instance

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
        time.Sleep(5 * time.Second)
        s.Broadcast <- sseserver.SSEMessage{"new-dog", []byte("Corgi"), "/pets/dogs"}
        s.Broadcast <- sseserver.SSEMessage{"new-cat", []byte("Persian"), "/pets/cats"}
        time.Sleep(1 * time.Second)
        s.Broadcast <- sseserver.SSEMessage{"new-dog", []byte("Terrier"), "/pets/dogs"}
        s.Broadcast <- sseserver.SSEMessage{"new-dog", []byte("Dauchsand"), "/pets/cats"}
        time.Sleep(2 * time.Second)
        s.Broadcast <- sseserver.SSEMessage{"new-cat", []byte("LOLcat"), "/pets/cats"}
    }()

    s.Serve(":8001") // bind to port and begin serving connections
}
```

All these event namespaces are exposed via HTTP endpoint in the
`/subscribe/:namespace` route.

On the client, we can easily connect to those endpoints using built-in functions in JS:
```js
// connect to an event source endpoint and print results
var es1 = new EventSource("http://localhost:8001/subscribe/time");
es1.onmessage = function(event) {
    console.log("TICK! The time is currently: " + event.data);
};

// connect to a different event source endpoint and register event handlers
// note that by subscribing to the "parent" namespace, we get all events
// contained within the pets hierarchy.
var es2 = new EventSource("http://localhost:8001/subscribe/pets")
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

### Keep-Alives

All connections will send periodic `:keepalive` messages as recommended in the
WHATWG spec (by default, every 15 seconds). Any library adhering to the
EventSource standard should already automatically ignore and filter out these
messages for you.

### Admin Page
By default, an admin status page is available for easy monitoring.

![screenshot](http://f.cl.ly/items/1v2X1k342K3p0K1O2x0B/ssestreamer-admin.png)

It's powered by a simple JSON API endpoint, which you can also use to build your
own reporting.  These endpoints can be disabled in the settings (see `Server.Options`).

### HTTP Middleware

`sseserver.Server` implements the standard Go `http.Handler` interface, so you
can easily integrate it with most existing HTTP middleware, and easily plug it
into whatever you are using for your current routing, etc.

## Acknowledgements

A lot of the initial ideas for handling the connection hub in idiomatic Go
originally came from Gary Burd's [go-websocket-chat][1], but has since
undergone extensive modification.

[1]: http://gary.burd.info/go-websocket-chat

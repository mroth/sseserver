package main

import (
	"expvar"
	"net/http"
	"time"

	"github.com/mroth/sseserver"
	"github.com/mroth/sseserver/admin"
)

func main() {
	s := sseserver.NewServer()

	// broadcast the time every second to the "/time" namespace
	go func() {
		ticker := time.Tick(1 * time.Second)
		for {
			t := <-ticker
			data := []byte(t.Format("3:04:05 pm (MST)"))
			s.Broadcast <- sseserver.SSEMessage{Data: data, Namespace: "/time"}
		}
	}()

	http.Handle("/subscribe/", s)

	// legacy admin page, both HTML/JSON
	http.Handle("/admin/", admin.AdminHandler(s))

	// you can also expose the status json via the standard expvar package, once
	// published it will be available via /debug/vars along with memstats etc.
	// see expvar package documentation for more details.
	expvar.Publish("sseserver", expvar.Func(func() interface{} {
		return s.Status()
	}))

	http.ListenAndServe("127.0.0.1:8333", nil)
}

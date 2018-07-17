package main

import (
	"net/http"
	"time"

	"github.com/mroth/sseserver"
)

func main() {
	s := sseserver.NewServer()

	// broadcast the time every second to the "/time" namespace
	go func() {
		ticker := time.Tick(1 * time.Second)
		for {
			// wait for the ticker to fire
			t := <-ticker
			// create the message payload, can be any []byte value
			data := []byte(t.Format("3:04:05 pm (MST)"))
			// send a message without an event on the "/time" namespace
			s.Broadcast <- sseserver.SSEMessage{Data: data, Namespace: "/time"}
		}
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
	http.Handle("/subscribe/", s)

	// s.Serve(":8111") // bind to port and begin serving connections
	http.ListenAndServe(":8111", nil)
}

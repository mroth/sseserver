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

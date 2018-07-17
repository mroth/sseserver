/*
In this example we will still only send simple text messages, but one can just
as easily serialize objects to JSON and send them through the data field, and
deserialize back into Javascript objects on the client side.
*/
package main

import (
	"math/rand"
	"time"

	"github.com/mroth/sseserver"
)

var dogBreeds = []string{
	"Corgi", "Terrier", "Dauchsand",
}
var catBreeds = []string{
	"Persian", "Maine Coon", "LOLcat",
}

func randomBreed(breeds []string) []byte {
	return []byte(breeds[rand.Intn(len(breeds))])
}

func main() {
	s := sseserver.NewServer()

	// simulate sending some scoped events on the "/pets" namespace
	go func() {
		for {
			msg := sseserver.SSEMessage{}
			if rand.Intn(2) == 0 {
				msg.Event = "new-cat"
				msg.Data = randomBreed(catBreeds)
				msg.Namespace = "/pets/cats"
			} else {
				msg.Event = "new-dog"
				msg.Data = randomBreed(dogBreeds)
				msg.Namespace = "/pets/dogs"
			}
			s.Broadcast <- msg

			r := rand.Intn(5) + 1
			time.Sleep(time.Duration(r) * time.Second)
		}
	}()

	s.Serve(":8222") // bind to port and beging serving connections
}

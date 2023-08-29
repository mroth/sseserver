package sseserver

import (
	"testing"
)


func TestServer_Shutdown(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}

	// verify calling multiple times is safe and does not hang
	for i := 0; i < 5; i++ {
		s.Shutdown()
	}

	// TODO: shutdown should also start disallowing new connections
	// TODO: verify no goroutine leaks
}

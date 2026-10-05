package sseserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConnectionStatusDuringWrites(t *testing.T) {
	req, _ := http.NewRequest("GET", "/pets/cats", nil)
	rr := httptest.NewRecorder()
	c := newConnection(rr, req, "/pets/cats")
	done := make(chan struct{})
	go func() {
		c.writer()
		close(done)
	}()

	const messages = 1000
	payload := SSEMessage{Data: []byte("hello")}.sseFormat()
	var previous uint64
	for i := 0; i < messages; i++ {
		c.send <- payload
		status := c.Status()
		if status.MsgsSent < previous || status.MsgsSent > uint64(i+1) {
			t.Errorf("unexpected message count: got %d after %d sends, previous %d", status.MsgsSent, i+1, previous)
		}
		previous = status.MsgsSent
	}
	close(c.send)
	<-done
	if got := c.Status().MsgsSent; got != messages {
		t.Errorf("message count: got %d want %d", got, messages)
	}
	if got := rr.Body.Len(); got != messages*len(payload) {
		t.Errorf("response length: got %d want %d", got, messages*len(payload))
	}
}

func TestServerStatusDuringBroadcasts(t *testing.T) {
	s := NewServer()
	defer s.hub.Shutdown()

	const messages = 1000
	done := make(chan struct{})
	go func() {
		for i := 0; i < messages; i++ {
			req, _ := http.NewRequest("GET", "/pets/cats", nil)
			c := newConnection(httptest.NewRecorder(), req, "/pets/cats")
			s.hub.register <- c
			s.Broadcast <- SSEMessage{Data: []byte("hello"), Namespace: "/pets/cats"}
			s.hub.unregister <- c
		}
		close(done)
	}()

	var previous uint64
	for {
		status := s.Status()
		if status.SentMsgs < previous || status.SentMsgs > messages {
			t.Errorf("unexpected broadcast count: got %d, previous %d", status.SentMsgs, previous)
		}
		previous = status.SentMsgs
		for _, c := range status.Connections {
			if c.Path != "/pets/cats" || c.Namespace != "/pets/cats" {
				t.Errorf("unexpected connection status: %+v", c)
			}
		}
		select {
		case <-done:
			if got := s.Status().SentMsgs; got != messages {
				t.Errorf("broadcast count: got %d want %d", got, messages)
			}
			return
		default:
		}
	}
}

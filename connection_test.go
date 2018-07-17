package sseserver

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

/*
New connections should get...
 - HTTP status OK 200
 - content-type event-stream
 - check all headers match what we want
*/
func TestConnectionHandler(t *testing.T) {
	// need to have a running hub, otherwise conn blocks trying to register
	h := newHub()
	h.Start()
	defer h.Shutdown()

	// use http.Request with Timeout context, so it will close itself, this
	// is a hacky way to just disconnect after we have all the headers without
	// messing around with modifying the http.ResponseRecorder too much.
	//
	// See also: https://github.com/golang/go/issues/4436
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := connectionHandler(h)
	ctx := req.Context()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()
	handler.ServeHTTP(rr, req.WithContext(ctx))

	t.Run("status", func(t *testing.T) {
		expect := http.StatusOK
		if status := rr.Code; status != expect {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, expect)
		}
	})

	t.Run("headers", func(t *testing.T) {
		var headerTests = []struct {
			header, expect string
		}{
			{"Content-type", "text/event-stream; charset=utf-8"},
			{"Connection", "keep-alive"},
			{"Cache-Control", "no-cache"},
			{"Access-Control-Allow-Origin", "*"},
		}
		hd := rr.Header()
		for _, ht := range headerTests {
			if actual := hd.Get(ht.header); actual != ht.expect {
				t.Errorf("%s header does not match: got %v want %v",
					ht.header, actual, ht.expect)
			}
		}
	})

}

/*
Connection receives broadcast messages to its send channel.
*/
func TestConnectionSend(t *testing.T) {
	// useless mocks since we're testing internals
	// if able to move [r,w] out of connection wont need these...
	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	c := newConnection(rr, req, "butts")

	// async send a sse msg 2x then close
	msg := SSEMessage{Event: "foo", Data: []byte("bar")}
	payload := msg.sseFormat()
	go func() {
		c.send <- payload
		c.send <- payload
		close(c.send)
	}()

	c.writer() // blocks until send is closed
	var expected = append(payload, payload...)
	if actual := rr.Body.Bytes(); !bytes.Equal(actual, expected) {
		t.Errorf("body does not match:\n[got]\n%s[expected]\n%s",
			actual, expected)
	}
}

/*
A connection should close if it's send channel is closed. This happens when the
hub wants us to shutdown gracefully. This is mostly designed to deal with
stalled clients -- as the hub will tell us to close the connection once our
buffer fills up.

Add a test for this on the connection side (tested on hub side already).
*/

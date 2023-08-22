package sseserver

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
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
	var testcases = []struct {
		name          string
		opts          []ServerOption
		expectStatus  int
		expectHeaders http.Header
	}{
		{
			name:         "default",
			opts:         nil,
			expectStatus: http.StatusOK,
			expectHeaders: http.Header{
				"Content-Type":  {"text/event-stream; charset=utf-8"},
				"Connection":    {"keep-alive"},
				"Cache-Control": {"no-cache"},
			},
		},
		{
			name: "cors",
			opts: []ServerOption{
				WithCORSAllowOrigin("*"),
			},
			expectStatus: http.StatusOK,
			expectHeaders: http.Header{
				"Content-Type":                {"text/event-stream; charset=utf-8"},
				"Connection":                  {"keep-alive"},
				"Cache-Control":               {"no-cache"},
				"Access-Control-Allow-Origin": {"*"},
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// need a running hub otherwise connection handler will block trying to register
			s, err := NewServer(tc.opts...)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Shutdown()
			handler := connectionHandler(s)

			// the connection will remain open to be available to stream content, so here we set a
			// timeout on the request context in order to drop the connection from the client side
			// after we have the headers
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			// check status code
			if got, want := rr.Code, tc.expectStatus; got != want {
				t.Errorf("unexpected status code: got %v want %v", got, want)
			}

			// check for missing headers or incorrect header values
			gotHeaders := rr.Result().Header
			for key, wantVal := range tc.expectHeaders {
				gotVal, found := gotHeaders[key]
				if !found {
					t.Errorf("missing expected header: %v: %v", key, wantVal)
				} else if !reflect.DeepEqual(gotVal, wantVal) {
					t.Errorf("%v: got %v want %v", key, gotVal, wantVal)
				}
			}

			// check for presence of any unexpected headers
			for k, v := range gotHeaders {
				_, found := tc.expectHeaders[k]
				if !found {
					t.Errorf("found unexpected header: %v: %v", k, v)
				}
			}
		})
	}

}

/*
Connection receives broadcast messages to its send channel.
*/
func TestConnectionSend(t *testing.T) {
	// useless mocks since we're testing internals
	// if able to move [r,w] out of connection wont need these...
	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	c := newConnection(rr, req, "test", DefaultConnMsgBufferSize)

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

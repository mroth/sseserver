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
		expectHeaders http.Header
	}{
		{
			name: "default",
			opts: nil,
			expectHeaders: http.Header{
				"Content-Type":  {"text/event-stream; charset=utf-8"},
				"Connection":    {"keep-alive"},
				"Cache-Control": {"no-cache"},
				"Server":        {"mroth/sseserver"},
			},
		},
		{
			name: "cors",
			opts: []ServerOption{
				WithCORSAllowOrigin("*"),
			},
			expectHeaders: http.Header{
				"Content-Type":                {"text/event-stream; charset=utf-8"},
				"Connection":                  {"keep-alive"},
				"Cache-Control":               {"no-cache"},
				"Access-Control-Allow-Origin": {"*"},
				"Server":                      {"mroth/sseserver"},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			// need to have a running hub, otherwise conn blocks trying to register
			s, err := NewServer(tc.opts...)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Shutdown()

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
			handler := connectionHandler(s)
			ctx := req.Context()
			ctx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
			defer cancel()
			handler.ServeHTTP(rr, req.WithContext(ctx))

			t.Run("status", func(t *testing.T) {
				// for now, expected status code is hardcoded OK
				expect := http.StatusOK
				if status := rr.Code; status != expect {
					t.Errorf("handler returned wrong status code: got %v want %v",
						status, expect)
				}
			})

			t.Run("headers", func(t *testing.T) {
				// check for missing or incorrect headers
				gotHeaders := rr.Result().Header
				for key, wantVal := range tc.expectHeaders {
					gotVal, found := gotHeaders[key]
					if !found {
						t.Errorf("missing expected header: %v: %v", key, wantVal)
					} else if !reflect.DeepEqual(gotVal, wantVal) {
						t.Errorf("%v: got %v want %v", key, gotVal, wantVal)
					}
				}
				// check for unexpected headers
				for k, v := range gotHeaders {
					_, found := tc.expectHeaders[k]
					if !found {
						t.Errorf("found unexpected header: %v: %v", k, v)
					}
				}
			})

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
	c := newConnection(rr, req, "butts", defaultConnBufSize)

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

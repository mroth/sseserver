package sseserver

import "testing"

var messageTests = []struct {
	msg         SSEMessage
	expected    []byte
	description string
}{
	{
		SSEMessage{"", []byte("foobar"), "abcd"},
		[]byte("data:foobar\n\n"),
		"A message with only data field",
	},
	{
		SSEMessage{"e12", []byte("foobar"), "abcd"},
		[]byte("event:e12\ndata:foobar\n\n"),
		"Message with event and data field",
	},
}

func TestFormat(t *testing.T) {
	for _, test := range messageTests {
		observed := test.msg.sseFormat()
		if string(observed) != string(test.expected) {
			t.Fatalf("Expected: %q, Actual: %q", test.expected, observed)
		}
	}
}

func BenchmarkFormat(b *testing.B) {
	b.StopTimer()

	for _, test := range messageTests {
		b.StartTimer()
		for i := 0; i < b.N; i++ {
			test.msg.sseFormat()
		}
		b.StopTimer()
	}
}

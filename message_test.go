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
		"DataFieldOnly",
	},
	{
		SSEMessage{"e12", []byte("foobar"), "abcd"},
		[]byte("event:e12\ndata:foobar\n\n"),
		"Event+DataField",
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
	for _, test := range messageTests {
		b.Run(test.description, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				test.msg.sseFormat()
			}
		})
	}
}

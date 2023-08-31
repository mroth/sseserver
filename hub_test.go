package sseserver

import (
	"strconv"
	"testing"
	"time"
)

func mockHub(numConnections int) (h *hub) {
	h = newHub()
	for i := 0; i < numConnections; i++ {
		h.Register(mockConn("/test"))
	}
	return h
}

func mockConn(namespace string) *connection {
	return &connection{
		send:      make(chan []byte, DefaultConnMsgBufferSize),
		created:   time.Now(),
		namespace: namespace,
	}
}

func mockSinkedHub(initialConnections map[string]int) (h *hub) {
	h = newHub()
	for namespace, num := range initialConnections {
		for i := 0; i < num; i++ {
			h.Register(mockSinkedConn(namespace, h))
		}
	}
	return h
}

// mock a connection that sinks data sent to it
func mockSinkedConn(namespace string, h *hub) *connection {
	c := &connection{
		send:      make(chan []byte, DefaultConnMsgBufferSize),
		created:   time.Now(),
		namespace: namespace,
	}
	go func() {
		for range c.send {
			// no-op, but will break loop if chan is closed
			// (versus using <-c.send in infinite loop)
		}
		// in practice, a connection tries to unregister itself here
		h.Unregister(c)
	}()
	return c
}

type deliveryCase struct {
	conn     *connection
	expected int
}

func TestBroadcastSingleplex(t *testing.T) {
	h := mockHub(0)
	defer h.Shutdown()

	c1 := mockConn("/foo")
	c2 := mockConn("/bar")
	h.Register(c1)
	h.Register(c2)

	//broadcast to foo channel
	h.Broadcast(SSEMessage{"", []byte("yo"), "/foo"})

	//check for proper delivery
	d := []deliveryCase{
		{c1, 1},
		{c2, 0},
	}
	for _, c := range d {
		if actual := len(c.conn.send); actual != c.expected {
			t.Errorf("Expected conn to have %d message in queue, actual: %d",
				c.expected, actual)
		}
	}

}

func TestBroadcastMultiplex(t *testing.T) {
	h := mockHub(0)
	defer h.Shutdown()

	c1 := mockConn("/foo")
	c2 := mockConn("/foo")
	c3 := mockConn("/burrito")
	h.Register(c1)
	h.Register(c2)
	h.Register(c3)

	//broadcast to channels
	h.Broadcast(SSEMessage{"", []byte("yo"), "/foo"})
	h.Broadcast(SSEMessage{"", []byte("yo"), "/foo"})
	h.Broadcast(SSEMessage{"", []byte("yo"), "/bar"})

	//check for proper delivery
	d := []deliveryCase{
		{c1, 2},
		{c2, 2},
		{c3, 0},
	}
	for _, c := range d {
		if actual := len(c.conn.send); actual != c.expected {
			t.Errorf("Expected conn to have %d messages in queue, actual: %d",
				c.expected, actual)
		}
	}
}

func TestBroadcastWildcards(t *testing.T) {
	h := mockHub(0)
	defer h.Shutdown()

	cDogs := mockConn("/pets/dogs")
	cCats := mockConn("/pets/cats")
	cWild := mockConn("/pets")
	cOther := mockConn("/kids")

	h.Register(cDogs)
	h.Register(cCats)
	h.Register(cWild)
	h.Register(cOther)

	//broadcast to channels
	h.Broadcast(SSEMessage{"", []byte("woof"), "/pets/dogs"})
	h.Broadcast(SSEMessage{"", []byte("meow"), "/pets/cats"})
	h.Broadcast(SSEMessage{"", []byte("wahh"), "/kids"})

	//check for proper delivery
	d := []deliveryCase{
		{cDogs, 1},
		{cCats, 1},
		{cWild, 2},
		{cOther, 1},
	}
	for _, c := range d {
		if actual := len(c.conn.send); actual != c.expected {
			t.Errorf("Expected conn %v to have %d message in queue, actual: %d",
				c.conn.namespace, c.expected, actual)
		}
	}
}

// if we force unregister a connection from the hub, we tell it exit by closing
// its send channel when a connection exits for any reason, it tries to
// unregister itself from the hub thus, this could theoretically lead to a panic
// if we try to close twice...
func TestDoubleUnregister(t *testing.T) {
	h := mockHub(0)
	defer h.Shutdown()

	c1 := mockSinkedConn("/badseed", h)
	c2 := mockSinkedConn("/goodseed", h)
	h.Register(c1)
	h.Register(c2)
	h.Unregister(c1)
	h.Unregister(c1)

	actual, expected := len(h.connections), 1
	if actual != expected {
		t.Errorf("unexpected num of conns: got %v want %v", actual, expected)
	}
}

// test double register is no-op
func TestDoubleRegister(t *testing.T) {
	h := mockHub(0)
	defer h.Shutdown()

	c1 := mockSinkedConn("/badseed", h)
	h.Register(c1)
	h.Register(c1)

	actual, expected := len(h.connections), 1
	if actual != expected {
		t.Errorf("unexpected num of conns: got %v want %v", actual, expected)
	}
}

// a connection that is not reading should eventually be killed
func TestKillsStalledConnection(t *testing.T) {
	h := mockHub(0)
	defer h.Shutdown()

	namespace := "/tacos"
	stalled := mockConn(namespace)         // slow connection - taco overflow
	sinked := mockSinkedConn(namespace, h) // hungry connection - loves tacos
	h.Register(stalled)
	h.Register(sinked)

	numSetupConns := len(h.connections)
	if numSetupConns != 2 {
		t.Fatal("unexpected num of conns after test setup!:", numSetupConns)
	}

	// send connBufSize+50% messages, ensuring the buffer overflows
	msg := SSEMessage{Data: []byte("hi"), Namespace: namespace}
	for i := 0; i <= DefaultConnMsgBufferSize+(DefaultConnMsgBufferSize*0.5); i++ {
		h.Broadcast(msg)
		// need to pause execution the tiniest bit to allow
		// other goroutines to execute if running on GOMAXPROCS=1
		time.Sleep(time.Microsecond)
	}

	// one of the connections should have been shutdown now...
	expected := 1
	if actual := len(h.connections); actual != expected {
		t.Errorf("unexpected num of conns: got %v want %v", actual, expected)
	}
	// ...and it better not be our taco loving friend
	if _, ok := h.connections[sinked]; !ok {
		t.Error("wrong connection appears to have been shutdown!")
	}
}

func BenchmarkRegister(b *testing.B) {
	h := mockHub(0)
	defer h.Shutdown()

	c := mockConn("/pets/cats")

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		h.Register(c)
	}
	b.StopTimer()
}

func BenchmarkUnregister(b *testing.B) {
	h := mockHub(1000)
	defer h.Shutdown()

	c := mockConn("/pets/cats")

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		h.Register(c)
		b.StartTimer()
		h.Unregister(c)
		b.StopTimer()
	}
}

func BenchmarkBroadcast(b *testing.B) {
	var msgBytes = []byte("foo bar woo")
	var sizes = []int{1, 10, 100, 500, 1000, 10000}

	for _, s := range sizes {
		b.Run(strconv.Itoa(s), func(b *testing.B) {
			h := mockSinkedHub(map[string]int{"/test": s})
			defer h.Shutdown()

			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				h.Broadcast(SSEMessage{"", msgBytes, "/test"})
			}
			b.StopTimer()
		})
	}
}

// benchmark for namespaced benchmarked
func BenchmarkBroadcastNS(b *testing.B) {
	var msgBytes = []byte("foo bar woo")
	var sizes = []int{100, 1000, 10000}

	mockDensityHub := func(s int) *hub {
		return mockSinkedHub(map[string]int{
			"/dense":  int(float64(s) * 0.95),
			"/sparse": int(float64(s) * 0.05),
		})
	}

	var benchAllSizes = func(namespace string) {
		b.Run(namespace, func(b *testing.B) {
			for _, s := range sizes {
				slashName := "/" + namespace
				b.Run(strconv.Itoa(s), func(b *testing.B) {
					hub := mockDensityHub(s)
					defer hub.Shutdown()

					b.ResetTimer()
					for n := 0; n < b.N; n++ {
						hub.Broadcast(SSEMessage{"", msgBytes, slashName})
					}
					b.StopTimer()
				})
			}
		})
	}
	benchAllSizes("dense")
	benchAllSizes("sparse")
}

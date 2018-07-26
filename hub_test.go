package sseserver

import (
	"strconv"
	"testing"
	"time"
)

func mockHub(numConnections int) (h *hub) {
	h = newHub()
	h.Start()
	for i := 0; i < numConnections; i++ {
		h.register <- mockConn("/test")
	}
	return h
}

func mockConn(namespace string) *connection {
	return &connection{
		send:      make(chan []byte, connBufSize),
		created:   time.Now(),
		namespace: namespace,
	}
}

func mockSinkedHub(initialConnections map[string]int) (h *hub) {
	h = newHub()
	h.Start()
	for namespace, num := range initialConnections {
		for i := 0; i < num; i++ {
			h.register <- mockSinkedConn(namespace, h)
		}
	}
	return h
}

// mock a connection that sinks data sent to it
func mockSinkedConn(namespace string, h *hub) *connection {
	c := &connection{
		send:      make(chan []byte, connBufSize),
		created:   time.Now(),
		namespace: namespace,
	}
	go func() {
		for range c.send {
			// no-op, but will break loop if chan is closed
			// (versus using <-c.send in infinite loop)
		}
		// in practice, a connection tries to unregister itself here
		h.unregister <- c
	}()
	return c
}

type deliveryCase struct {
	conn     *connection
	expected int
}

func TestBroadcastSingleplex(t *testing.T) {
	h := mockHub(0)
	c1 := mockConn("/foo")
	c2 := mockConn("/bar")
	h.register <- c1
	h.register <- c2

	//broadcast to foo channel
	h.broadcast <- SSEMessage{"", []byte("yo"), "/foo"}
	h.Shutdown() // ensures delivery is finished

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
	c1 := mockConn("/foo")
	c2 := mockConn("/foo")
	c3 := mockConn("/burrito")
	h.register <- c1
	h.register <- c2
	h.register <- c3

	//broadcast to channels
	h.broadcast <- SSEMessage{"", []byte("yo"), "/foo"}
	h.broadcast <- SSEMessage{"", []byte("yo"), "/foo"}
	h.broadcast <- SSEMessage{"", []byte("yo"), "/bar"}
	h.Shutdown() // ensures delivery is finished

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
	cDogs := mockConn("/pets/dogs")
	cCats := mockConn("/pets/cats")
	cWild := mockConn("/pets")
	cOther := mockConn("/kids")

	h.register <- cDogs
	h.register <- cCats
	h.register <- cWild
	h.register <- cOther

	//broadcast to channels
	h.broadcast <- SSEMessage{"", []byte("woof"), "/pets/dogs"}
	h.broadcast <- SSEMessage{"", []byte("meow"), "/pets/cats"}
	h.broadcast <- SSEMessage{"", []byte("wahh"), "/kids"}
	h.Shutdown() // ensures delivery is finished

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
	h.register <- c1
	h.register <- c2
	h.unregister <- c1
	h.unregister <- c1
	h.broadcast <- SSEMessage{Data: []byte("no-op to ensure finished")}
	// ^^ the above broadcast forces the hub run loop to be past the initial
	// registrations, preventing a possible race condition.
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
	h.register <- c1
	h.register <- c1
	h.broadcast <- SSEMessage{Data: []byte("no-op to ensure finished")}
	// ^^ the above broadcast forces the hub run loop to be past the initial
	// registrations, preventing a possible race condition.
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
	h.register <- stalled
	h.register <- sinked
	h.broadcast <- SSEMessage{Data: []byte("no-op to ensure finished")}
	// ^^ the above broadcast forces the hub run loop to be past the initial
	// registrations, preventing a possible race condition.
	numSetupConns := len(h.connections)
	if numSetupConns != 2 {
		t.Fatal("unexpected num of conns after test setup!:", numSetupConns)
	}

	// send connBufSize+50% messages, ensuring the buffer overflows
	msg := SSEMessage{Data: []byte("hi"), Namespace: namespace}
	for i := 0; i <= connBufSize+(connBufSize*0.5); i++ {
		h.broadcast <- msg
		// need to pause execution the tiniest bit to allow
		// other goroutines to execute if running on GOMAXPROCS=1
		time.Sleep(time.Nanosecond)
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
		h.register <- c
	}
	b.StopTimer()
}

func BenchmarkUnregister(b *testing.B) {
	h := mockHub(1000)
	defer h.Shutdown()
	c := mockConn("/pets/cats")

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		h.register <- c
		b.StartTimer()
		h.unregister <- c
		b.StopTimer()
	}
}

func BenchmarkBroadcast(b *testing.B) {
	var msgBytes = []byte("foo bar woo")
	var sizes = []int{1, 10, 100, 500, 1000, 10000}

	for _, s := range sizes {
		b.Run(strconv.Itoa(s), func(b *testing.B) {
			h := mockSinkedHub(map[string]int{"/test": s})
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				h.broadcast <- SSEMessage{"", msgBytes, "/test"}
			}
			b.StopTimer()
			h.Shutdown()
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
					b.ResetTimer()
					for n := 0; n < b.N; n++ {
						hub.broadcast <- SSEMessage{"", msgBytes, slashName}
					}
					b.StopTimer()
					hub.Shutdown()
				})
			}
		})
	}
	benchAllSizes("dense")
	benchAllSizes("sparse")
}

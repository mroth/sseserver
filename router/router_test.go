package router

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randKey(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

// mock up something that looks kind of like a emojitrack-gostreamer at low load
func mockEmojiTable() Node {
	tree := New()

	// one raw subscriber
	tree.InsertAt(NS("raw"), "raw-Client-1")

	// a bunch of detail chans
	for i := 0; i < 500; i++ {
		k := randKey(6)
		d := tree.FindOrCreate(Namespace{"details", k})
		// lets say maybe 10% have an active sub or two
		r := rand.Intn(20)
		if r <= 1 {
			d.Insert(fmt.Sprintf("%s-Client-%d", k, 1))
		}
		if r == 0 {
			d.Insert(fmt.Sprintf("%s-Client-%d", k, 2))
		}
	}

	// and a bunch of eps subscribers
	eps := tree.FindOrCreate(NS("eps"))
	for i := 0; i < 100; i++ {
		eps.Insert(fmt.Sprintf("eps-Client-%d", i))
	}

	return tree
}

func TestStringToNamespace(t *testing.T) {
	testCases := []struct {
		s  string
		ns Namespace
	}{
		{s: "/", ns: Namespace{}},
		{s: "", ns: Namespace{}},
		{s: "/pets", ns: Namespace{"pets"}},
		{s: "pets", ns: Namespace{"pets"}},
		{s: "pets/", ns: Namespace{"pets"}},
		{s: "/pets/", ns: Namespace{"pets"}},
		{s: "/pets/cats", ns: Namespace{"pets", "cats"}},
		{s: "pets/cats", ns: Namespace{"pets", "cats"}},
		{s: "pets/cats/", ns: Namespace{"pets", "cats"}},
		{s: "/pets/cats/", ns: Namespace{"pets", "cats"}},
		{s: "/pets/dogs/terriers", ns: Namespace{"pets", "dogs", "terriers"}},
		{s: "pets/dogs/terriers", ns: Namespace{"pets", "dogs", "terriers"}},
		{s: "pets/dogs/terriers/", ns: Namespace{"pets", "dogs", "terriers"}},
		{s: "/pets/dogs/terriers/", ns: Namespace{"pets", "dogs", "terriers"}},
	}
	for _, tc := range testCases {
		actual := NS(tc.s)
		if !reflect.DeepEqual(tc.ns, actual) {
			t.Errorf("for string %#v\nwant %#v\ngot  %#v", tc.s, tc.ns, actual)
		}
	}
}

func BenchmarkStringToNamespace(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NS("/pets/dogs/terriers/")
	}
}

func TestNamespaceToString(t *testing.T) {
	testCases := []struct {
		ns Namespace
		s  string
	}{
		{ns: Namespace{}, s: "/"},
		{ns: Namespace{"pets"}, s: "/pets"},
		{ns: Namespace{"pets", "cats"}, s: "/pets/cats"},
		{ns: Namespace{"pets", "dogs", "terriers"}, s: "/pets/dogs/terriers"},
	}

	for _, tc := range testCases {
		actual := tc.ns.String()
		if actual != tc.s {
			t.Errorf("want %#v got %#v", tc.s, actual)
		}
	}
}

// TODO TestBasicOperations
// TODO TestInsert
// TODO TestRemove
// TODO TestFind

// func TestShift(t *testing.T) {
// 	ns := NS("foo/bar/hey")
// 	f := shift(&ns)
// 	expectF := "foo"
// 	if f != expectF {
// 		t.Errorf("want %v got %v", expectF, f)
// 	}
// 	expectNS := NS("bar/hey")
// 	if !reflect.DeepEqual(expectNS, ns) {
// 		t.Errorf("want %v got %v", expectNS, ns)
// 	}
// }

func TestRelationships(t *testing.T) {
	root := New()
	foo := root.FindOrCreate(NS("foo"))
	foo.FindOrCreate(NS("bar1"))
	foo.FindOrCreate(NS("bar2"))
	bar3 := foo.FindOrCreate(NS("bar3"))
	hey1 := bar3.FindOrCreate(NS("hey1"))
	bar3.FindOrCreate(NS("hey2"))

	testCases := []struct {
		node            *Node
		expectChilds    int
		expectDescs     int
		expectAncestors int
		expectNS        Namespace
	}{
		{
			node:            hey1,
			expectChilds:    0,
			expectDescs:     0,
			expectAncestors: 3,
			expectNS:        Namespace{"foo", "bar3", "hey1"}},
		{
			node:            bar3,
			expectChilds:    2,
			expectDescs:     2,
			expectAncestors: 2,
			expectNS:        Namespace{"foo", "bar3"}},
		{
			node:            foo,
			expectChilds:    3,
			expectDescs:     5,
			expectAncestors: 1,
			expectNS:        Namespace{"foo"}},
		{
			node:            &root,
			expectChilds:    1,
			expectDescs:     6,
			expectAncestors: 0,
			expectNS:        Namespace{""}},
	}

	for _, tc := range testCases {
		actualChilds := len(tc.node.Children())
		if actualChilds != tc.expectChilds {
			t.Errorf("wrong number of children - want %v got %v", tc.expectChilds, actualChilds)
		}
		actualDescs := len(tc.node.Descendents())
		if actualDescs != tc.expectDescs {
			t.Errorf("wrong number of descendents - want %v got %v", tc.expectDescs, actualDescs)
		}
		actualAncestors := len(tc.node.Ancestors())
		if actualAncestors != tc.expectAncestors {
			t.Errorf("wrong number of ancestors - want %v got %v", tc.expectAncestors, actualAncestors)
		}
		actualNS := tc.node.Namespace()
		if !reflect.DeepEqual(actualNS, tc.expectNS) {
			t.Errorf("wrong namespace - want %v got %v", tc.expectNS, actualNS)
		}
	}
}

func BenchmarkDescendents(b *testing.B) {
	root := New()
	foo := root.FindOrCreate(NS("foo"))
	foo.FindOrCreate(NS("bar1"))
	foo.FindOrCreate(NS("bar2"))
	bar3 := foo.FindOrCreate(NS("bar3"))
	bar3.FindOrCreate(NS("hey1"))
	bar3.FindOrCreate(NS("hey2"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		root.Descendents()
	}
}

func BenchmarkComparative(b *testing.B) {
	et := mockEmojiTable()
	// artificially insert one more we can look for later
	nsd := Namespace{"details", randKey(6)}
	et.InsertAt(nsd, "myspecialfriend")

	// sinked receiver
	sink := make(chan string)
	defer close(sink)
	go func() {
		for range sink {
		}
	}()

	// mock something kinda like an oldskool conn
	// namespace as a string
	// for here we just do value like a string too for comparison
	type osCon struct {
		namespace string
		value     string
	}

	// kinda like the old school way, but we're using string
	// as value in this example instead of Connection
	var oldskool = make(map[*osCon]struct{})
	et.TraverseDown(func(n *Node) {
		for k := range n.values {
			namespace := fmt.Sprintf("%s", n.Namespace())
			nnn := &osCon{namespace: namespace, value: k.(string)}
			oldskool[nnn] = struct{}{}
		}
	})

	_benchOld := func(b *testing.B, targetNS string) {
		for i := 0; i < b.N; i++ {
			for c := range oldskool {
				if strings.HasPrefix(c.namespace, targetNS) {
					sink <- c.value
				}
			}
		}
	}

	_benchNew := func(b *testing.B, targetNS Namespace) {
		for i := 0; i < b.N; i++ {
			et.FindOrCreate(targetNS).ForEachAscendingValue(func(v Value) {
				sink <- v.(string)
			})
		}
	}

	b.Run("dense", func(b *testing.B) {
		b.Run("old", func(b *testing.B) {
			_benchOld(b, "/eps")
		})
		b.Run("new", func(b *testing.B) {
			_benchNew(b, Namespace{"eps"})
		})
	})

	b.Run("sparse", func(b *testing.B) {
		b.Run("old", func(b *testing.B) {
			_benchOld(b, nsd.String())
		})
		b.Run("new", func(b *testing.B) {
			_benchNew(b, nsd)
		})
	})
}

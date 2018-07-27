// Package router provides the routing tree for message delivery in sseserver.
//
// It essentially handles building a tree optimized for certain use cases...
//
// TODO: document me more if we actually end using this!
package router

import (
	"errors"
	"strings"
)

type Namespace []string
type Value interface{}

// NS converts a slash-delimited string into a Namespace.
func NS(s string) Namespace {
	trimmed := strings.Trim(s, "/ ")
	if trimmed == "" {
		return Namespace{}
	}
	return Namespace(strings.Split(trimmed, "/"))
}

func (ns Namespace) String() string {
	return "/" + strings.Join(ns, "/")
}

// see https://groups.google.com/forum/#!topic/golang-nuts/obZI4uyZTe0
// nope: causing too many problems with unexpected mutation of NS passed externally
// func shift(ns *Namespace) (firstKey string) {
// 	firstKey = (*ns)[0]
// 	copy(*ns, (*ns)[1:])
// 	*ns = (*ns)[:len(*ns)-1]
// 	return
// }

// A node has 0..N value objects associated with it
type Node struct {
	parent   *Node
	children map[string]*Node
	key      string
	values   map[Value]struct{} // closest to a Set in Go
}

// New returns a new root Node (without a parent)
func New() Node {
	return *newNode(nil, "")
}

// newNode initializes a new Node with parent `parent`.
// does not automatically add it into lookup table, for
// internal usage
func newNode(parent *Node, key string) *Node {
	return &Node{
		key:      key,
		parent:   parent,
		children: make(map[string]*Node),
		values:   make(map[Value]struct{}),
	}
}

// Find returns a reference child Node at relative target namespace, or nil if
// it does not exist.
func (n *Node) Find(ns Namespace) (*Node, error) {
	if len(ns) == 0 {
		return n, nil
	}
	// target := shift(&ns)
	target, ns := ns[0], ns[1:]
	if c, ok := n.children[target]; ok {
		return c.Find(ns)
	}
	return nil, errors.New(ns.String() + " not found")
}

// FindOrCreate will return the reference child Node at relative namespace ns,
// creating it if it does not already exist.
func (n *Node) FindOrCreate(ns Namespace) *Node {
	if len(ns) == 0 {
		return n
	}
	// target := shift(&ns)
	target, ns := ns[0], ns[1:]
	if _, exists := n.children[target]; !exists {
		n.children[target] = newNode(n, target)
	}
	return n.children[target].FindOrCreate(ns)
}

/****************************************************************************
  Dealing with values
****************************************************************************/

// Values returns all values associated with a Node
func (n *Node) Values() []Value {
	var vs []Value
	for v := range n.values {
		vs = append(vs, v)
	}
	return vs
}

// Insert adds a value to a Node directly.
func (n *Node) Insert(v Value) {
	n.values[v] = struct{}{}
}

// Remove removes a value from a Node.
func (n *Node) Remove(v Value) {
	delete(n.values, v)
}

// InsertAt inserts a value at the target namespace relative to Node n.
//
// Returns address *Node where things were inserted.
func (n *Node) InsertAt(ns Namespace, v Value) *Node {
	var dst = n.FindOrCreate(ns)
	dst.Insert(v)
	return dst
}

/****************************************************************************
  Graph traversal and relationships
****************************************************************************/

// TraverseDown visits the node & each descendent node, applying traverseFn
func (n *Node) TraverseDown(traverseFn func(*Node)) {
	traverseFn(n)
	for i := range n.children {
		n.children[i].TraverseDown(traverseFn)
	}
}

// TraverseUp visits the node and each ancestor node, applying traverseFn
func (n *Node) TraverseUp(traverseFn func(*Node)) {
	traverseFn(n)
	if n.parent != nil {
		n.parent.TraverseUp(traverseFn)
	}
}

// Children returns the direct children of a Node only
func (n *Node) Children() []*Node {
	childs := make([]*Node, 0, len(n.children))
	for _, c := range n.children {
		childs = append(childs, c)
	}
	return childs
}

// Descendents returns all children of the Node, and their children, and their
// children...
func (n *Node) Descendents() []*Node {
	var descNodes []*Node
	n.TraverseDown(func(n *Node) {
		descNodes = append(descNodes, n)
	})
	return descNodes[1:]
}

// Ancestors returns the parent of the Node, and it's parent, and it's parent...
// Note that this will include the root node of the tree.
func (n *Node) Ancestors() []*Node {
	var ancNodes []*Node
	n.TraverseUp(func(n *Node) {
		ancNodes = append(ancNodes, n)
	})
	return ancNodes[1:]
}

// ForEachAscendingValue traverses up the tree, applying fn to all the values
// contained in every Node along the way.
func (n *Node) ForEachAscendingValue(fn func(Value)) {
	n.TraverseUp(func(np *Node) {
		for v := range np.values {
			fn(v)
		}
	})
}

// ForEachDescendingValue traverses up the tree, applying fn to all the values
// contained in every Node along the way.
func (n *Node) ForEachDescendingValue(fn func(Value)) {
	n.TraverseDown(func(np *Node) {
		for v := range np.values {
			fn(v)
		}
	})
}

// Namespace returns the fully-qualified namespace for a Node by walking up the
// tree.
//
// TODO: modify to use TraveseUp?
func (n *Node) Namespace() Namespace {
	keys := Namespace{n.key}
	p := n.parent
	for p != nil && p.parent != nil {
		keys = append(Namespace{p.key}, keys...)
		p = p.parent
	}
	return keys
}

package scan

import (
	"reflect"
	"sync"
)

// nodeID is used to uniquely identify a node
type nodeID struct {
	typeStr string
	value   reflect.Value
}

// Node stores the information for a specific golang object
type Node struct {
	lock *sync.Mutex

	// basic informations for identifying an object
	value reflect.Value

	// Children is the set of children nodes of the golang object
	Children map[nodeID]*Node

	// Map marks the the KV pair in a map
	// the node itself is a key element, the Map points to its value element
	Map *Node
}

// NewNode initiates a new node for a given golang object
func NewNode(obj interface{}) *Node {
	node := &Node{
		lock:  &sync.Mutex{},
		value: reflect.ValueOf(obj),
	}
	node.Children = make(map[nodeID]*Node)
	return node
}

// ResolveObj returns the reference of the original object
func (n *Node) ResolveObj() interface{} {
	return n.value.Interface()
}

// nodeID derives the nodeID for a given node
func (n *Node) nodeID(typeIdGen func(reflect.Type) string) nodeID {
	return nodeID{
		typeStr: typeIdGen(n.value.Type()),
		value:   n.value,
	}
}

// RegisterChild returns true if successfully register child node
// return false if the child has already been registered
func (n *Node) RegisterChild(node *Node, typeIdGen func(reflect.Type) string) bool {
	n.lock.Lock()
	defer n.lock.Unlock()

	id := node.nodeID(typeIdGen)
	if _, exist := n.Children[id]; exist {
		return false
	}
	n.Children[id] = node
	return true
}

// InferPtr returns the object being pointed by this pointer node
// will panic if the node is not a pointer
func (n *Node) InferPtr() interface{} {
	return n.value.Elem().Interface()
}

func (n *Node) AddMap(target *Node) {
	n.lock.Lock()
	defer n.lock.Unlock()

	n.Map = target
}

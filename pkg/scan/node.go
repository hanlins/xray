package scan

import (
	"reflect"
	"sync"
)

// Node stores the information for a specific golang object
type Node struct {
	lock *sync.Mutex

	// basic informations for identifying an object
	value    reflect.Value
	typeInfo reflect.Type

	// Children is the set of children nodes of the golang object
	Children map[*Node]bool
}

// NewNode initiates a new node for a given golang object
func NewNode(obj interface{}) *Node {
	node := &Node{
		lock:     &sync.Mutex{},
		value:    reflect.ValueOf(obj),
		typeInfo: reflect.TypeOf(obj),
	}
	node.Children = make(map[*Node]bool)
	return node
}

// ResolveObj returns the reference of the original object
func (n *Node) ResolveObj() interface{} {
	return n.value.Interface()
}

// RegisterChild returns true if successfully register child node
// return false if the child has already been registered
func (n *Node) RegisterChild(node *Node) bool {
	n.lock.Lock()
	defer n.lock.Unlock()

	if _, exist := n.Children[node]; exist {
		return false
	}
	n.Children[node] = true
	return true
}

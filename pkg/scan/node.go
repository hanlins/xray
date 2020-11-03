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

	// Fields is for structed object, field name will be mapped to a node ID
	Fields map[string]nodeID
}

// NewNode initiates a new node for a given golang object
func NewNode(value reflect.Value) *Node {
	node := &Node{
		lock:  &sync.Mutex{},
		value: value,
	}
	node.Children = make(map[nodeID]*Node)
	if value.Kind() == reflect.Struct {
		node.Fields = make(map[string]nodeID)
	}
	return node
}

// ResolveObj returns the reference of the original object
func (n *Node) ResolveObj() interface{} {
	return n.value.Interface()
}

// nodeID derives the nodeID for a given node
func (n *Node) nodeID(typeIdGen func(reflect.Type) string) nodeID {
	id := nodeID{value: n.value}
	emptyValue := reflect.Value{}
	if n.value == emptyValue {
		id.typeStr = "nil"
	} else {
		id.typeStr = typeIdGen(n.value.Type())
	}
	return id
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
func (n *Node) InferPtr() reflect.Value {
	return n.value.Elem()
}

// Kind returns the kind of the current node
func (n *Node) Kind() reflect.Kind {
	return n.value.Kind()
}

// Kind returns the kind of the current node
func (n *Node) Value() reflect.Value {
	return n.value
}

// RegisterField add the field name -> node ID mapping to the node
func (n *Node) RegisterField(fieldName string, id nodeID) {
	n.lock.Lock()
	defer n.lock.Unlock()

	n.Fields[fieldName] = id
}

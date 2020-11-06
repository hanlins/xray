package scan

import (
	"fmt"
	"hash/fnv"
	"reflect"
	"sync"
)

// NodeID is used to uniquely identify a node
type NodeID struct {
	typeStr string
	value   reflect.Value
}

// Node stores the information for a specific golang object
type Node struct {
	lock *sync.Mutex

	// basic informations for identifying an object
	value reflect.Value

	// Children is the set of children nodes of the golang object
	Children map[NodeID]*Node

	// Fields is for structed object, field name will be mapped to a node ID
	Fields map[string]NodeID
}

// NewNode initiates a new node for a given golang object
func NewNode(value reflect.Value) *Node {
	node := &Node{
		lock:  &sync.Mutex{},
		value: value,
	}
	node.Children = make(map[NodeID]*Node)
	if value.Kind() == reflect.Struct {
		node.Fields = make(map[string]NodeID)
	}
	return node
}

// ResolveObj returns the reference of the original object
func (n *Node) ResolveObj() interface{} {
	return n.value.Interface()
}

// NodeID derives the NodeID for a given node
func (n *Node) NodeID(typeIdGen func(reflect.Type) string) NodeID {
	id := NodeID{value: n.value}
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

	id := node.NodeID(typeIdGen)
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
func (n *Node) RegisterField(fieldName string, id NodeID) {
	n.lock.Lock()
	defer n.lock.Unlock()

	n.Fields[fieldName] = id
}

// string returns the string representation of the NodeID object
// It supposed to be unique among for each node ID
func (nid *NodeID) string() string {
	return fmt.Sprintf("%s.%#v", nid.typeStr, nid.value)
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// Hash returns the hash version of NodeID identifier
// It supposed to be unique among for each node ID
func (nid *NodeID) Hash() string {
	return fmt.Sprintf("%d", hash(nid.string()))
}

// String returns the string representation of NodeID identifier
func (nid *NodeID) String() string {
	return fmt.Sprintf("%v", nid.value)
}

// IsNil returns true if the node ID is for a nil object
func (nid *NodeID) IsNil() bool {
	return nid.typeStr == "nil"
}

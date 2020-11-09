package xray

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewNode(t *testing.T) {
	var a int
	node := NewNode(reflect.ValueOf(a))
	assert.NotNil(t, node)
}

type student struct {
	name string
	id   string
}

func TestResolveObj(t *testing.T) {
	var integer int

	testCases := []struct {
		obj interface{}
		msg string
	}{
		{&integer, "integer"},
		{make(map[string]string), "map"},
		{make([]int, 5), "slice"},
		{student{"foo", "89757"}, "struct"},
	}

	for _, testCase := range testCases {
		node := NewNode(reflect.ValueOf(testCase.obj))
		assert.Equal(t, interface{}(testCase.obj), node.ResolveObj(), "expect %s equal", testCase.msg)
	}
}

func TestRegisterChild(t *testing.T) {
	s1, s2, s3 := student{"s1", "id1"}, student{"s2", "id2"}, student{"s3", "id3"}
	n1, n2, n3 := NewNode(reflect.ValueOf(&s1)), NewNode(reflect.ValueOf(&s2)), NewNode(reflect.ValueOf(&s3))

	// n2_2 reference the same obj as n2 but it's a different node obj
	n2_2 := NewNode(reflect.ValueOf(&s2))

	assert.True(t, n1.RegisterChild(n2, getTypeID))
	assert.True(t, n1.RegisterChild(n3, getTypeID))
	assert.False(t, n1.RegisterChild(n2, getTypeID))

	// make sure node objects that points to the same object won't duplicate
	assert.False(t, n1.RegisterChild(n2_2, getTypeID))
}

func TestNilNodeIDString(t *testing.T) {
	nilNode := NewNode(reflect.ValueOf(nil))
	nilNodeID := nilNode.NodeID(getTypeID)
	assert.Equal(t, "nil.<invalid reflect.Value>", nilNodeID.string())
}

func TestPrimitiveNodeIDString(t *testing.T) {
	intNode := NewNode(reflect.ValueOf(100))
	intNodeID := intNode.NodeID(getTypeID)
	assert.Equal(t, "/int/int.100", intNodeID.string())
}

type nodeTestStruct struct {
	field1 int
	field2 int
}

func TestStructNodeIDString(t *testing.T) {
	tn := nodeTestStruct{field1: 1, field2: 2}
	nilNode := NewNode(reflect.ValueOf(tn))
	nilNodeID := nilNode.NodeID(getTypeID)
	assert.Equal(t, "github.com/hanlins/xray/nodeTestStruct/xray.nodeTestStruct.xray.nodeTestStruct{field1:1, field2:2}", nilNodeID.string())
}

func TestNodeIDHash(t *testing.T) {
	nilNode := NewNode(reflect.ValueOf(nil))
	nilNodeID := nilNode.NodeID(getTypeID)
	assert.Equal(t, "2951924275", nilNodeID.Hash())

}

func TestNodeIDStringValue(t *testing.T) {
	intNode := NewNode(reflect.ValueOf(100))
	intNodeID := intNode.NodeID(getTypeID)
	assert.Equal(t, "100", intNodeID.String())
}

func TestNodeIDIsNil(t *testing.T) {
	nilNode := NewNode(reflect.ValueOf(nil))
	nilNodeID := nilNode.NodeID(getTypeID)
	assert.True(t, nilNodeID.IsNil())

	// pointer pointing to a nil value should not be nil
	// as itself is an variable
	var ptr *Node
	nilPtrNode := NewNode(reflect.ValueOf(ptr))
	nilPtrNodeID := nilPtrNode.NodeID(getTypeID)
	assert.False(t, nilPtrNodeID.IsNil())
}

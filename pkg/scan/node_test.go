package scan

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewNode(t *testing.T) {
	var a int
	node := NewNode(a)
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
		node := NewNode(testCase.obj)
		assert.Equal(t, interface{}(testCase.obj), node.ResolveObj(), "expect %s equal", testCase.msg)
	}
}

func TestRegisterChild(t *testing.T) {
	s1, s2, s3 := student{"s1", "id1"}, student{"s2", "id2"}, student{"s3", "id3"}
	n1, n2, n3 := NewNode(&s1), NewNode(&s2), NewNode(&s3)

	assert.True(t, n1.RegisterChild(n2))
	assert.True(t, n1.RegisterChild(n3))
	assert.False(t, n1.RegisterChild(n2))
}

package scan

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultTypeIdGenerator(t *testing.T) {
	assert.Equal(t, "//*context.emptyCtx", getTypeID(reflect.TypeOf(context.TODO())))

	// test third-party struct
	thirdPartyObj := assert.Comparison(func() bool { return true })
	assert.Equal(t, "github.com/stretchr/testify/assert/Comparison/assert.Comparison", getTypeID(reflect.TypeOf(thirdPartyObj)))
}

// checkNodeBlock return true if node channel is blocked
func checkNodeBlock(s *Scanner, timeout time.Duration) bool {
	resultCh := make(chan bool)
	go func(nodech chan<- *Node, resultCh chan<- bool) {
		select {
		case s.nodeCh <- NewNode(s):
			resultCh <- false
		case <-time.After(timeout):
			resultCh <- true
		}
	}(s.nodeCh, resultCh)
	return <-resultCh

}

func TestWithParallism(t *testing.T) {
	scanner := NewScanner(nil)
	timeout := 5 * time.Microsecond

	assert.Equal(t, 0, scanner.parallism)
	assert.True(t, checkNodeBlock(scanner, timeout))

	scanner = scanner.WithParallism(2)
	// first two attempts unblocked
	assert.False(t, checkNodeBlock(scanner, timeout))
	assert.False(t, checkNodeBlock(scanner, timeout))
	// third attempt blocked
	assert.True(t, checkNodeBlock(scanner, timeout))
}

func TestWithFilter(t *testing.T) {
	f1 := func(n *Node) bool {
		return false
	}
	f2 := func(n *Node) bool {
		return true
	}

	scanner := NewScanner(nil)
	assert.Equal(t, 0, len(scanner.filters))

	scanner = scanner.WithFilter(f1).WithFilter(f2)
	assert.Equal(t, 2, len(scanner.filters))

	scanner = scanner.WithFilter(f1).WithFilter(f2)
	assert.Equal(t, 2, len(scanner.filters))
}

func TestAcceptNode(t *testing.T) {
	varStr := "varStr"
	varStruct := struct{ a int }{a: 1}

	noStr := func(node *Node) bool {
		k := node.value.Type().Kind()
		return k != reflect.String
	}
	noStruct := func(node *Node) bool {
		k := node.value.Type().Kind()
		return k != reflect.Struct
	}

	s0 := NewScanner(nil)
	assert.True(t, s0.acceptNode(NewNode(varStr)))
	assert.True(t, s0.acceptNode(NewNode(varStruct)))

	s1 := NewScanner(nil).WithFilter(noStr)
	assert.False(t, s1.acceptNode(NewNode(varStr)))
	assert.True(t, s1.acceptNode(NewNode(varStruct)))

	s2 := NewScanner(nil).WithFilter(noStruct)
	assert.True(t, s2.acceptNode(NewNode(varStr)))
	assert.False(t, s2.acceptNode(NewNode(varStruct)))

	s3 := NewScanner(nil).WithFilter(noStr).WithFilter(noStruct)
	assert.False(t, s3.acceptNode(NewNode(varStr)))
	assert.False(t, s3.acceptNode(NewNode(varStruct)))
}

func TestDecomposePrimitives(t *testing.T) {
	s := NewScanner(nil).WithParallism(3)

	s.decompose(context.Background(), nil, 1)
	s.decompose(context.Background(), nil, true)
	s.decompose(context.Background(), nil, "test")

	// should store all primitives
	assert.Len(t, s.nodes, 3)

	// primitives has no children
	intNode := <-s.nodeCh
	assert.Len(t, intNode.Children, 0)
	boolNode := <-s.nodeCh
	assert.Len(t, boolNode.Children, 0)
	strNode := <-s.nodeCh
	assert.Len(t, strNode.Children, 0)
}

func TestDecomposePtr(t *testing.T) {
	str := "string"
	strPtr1 := &str
	strPtr2 := &str
	s := NewScanner(nil).WithParallism(2)

	s.decompose(context.Background(), nil, strPtr1)
	assert.Len(t, s.nodes, 2)

	// same pointer is stored only once
	s.decompose(context.Background(), nil, strPtr2)
	assert.Len(t, s.nodes, 2)

	// string should has no children
	strNode := <-s.nodeCh
	assert.Len(t, strNode.Children, 0)

	// strPtr should have children
	strPtrNode := <-s.nodeCh
	assert.Len(t, strPtrNode.Children, 1)
}

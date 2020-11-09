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

func newNode(obj interface{}) *Node {
	return NewNode(reflect.ValueOf(obj))
}

// checkNodeBlock return true if node channel is blocked
func checkNodeBlock(s *Scanner, timeout time.Duration) bool {
	resultCh := make(chan bool)
	go func(nodech chan<- NodeID, resultCh chan<- bool) {
		select {
		case s.nodeIDCh <- s.getNodeID(newNode(s)):
			resultCh <- false
		case <-time.After(timeout):
			resultCh <- true
		}
	}(s.nodeIDCh, resultCh)
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
	assert.Len(t, scanner.filters, 0)

	scanner = scanner.WithFilter(f1).WithFilter(f2)
	assert.Len(t, scanner.filters, 2)

	scanner = scanner.WithFilter(f1).WithFilter(f2)
	assert.Len(t, scanner.filters, 2)
}

func TestAcceptNode(t *testing.T) {
	varStr := "varStr"
	varStruct := struct{ a int }{a: 1}

	noStr := func(node *Node) bool {
		k := node.Kind()
		return k != reflect.String
	}
	noStruct := func(node *Node) bool {
		k := node.Kind()
		return k != reflect.Struct
	}

	s0 := NewScanner(nil)
	assert.True(t, s0.acceptNode(newNode(varStr)))
	assert.True(t, s0.acceptNode(newNode(varStruct)))

	s1 := NewScanner(nil).WithFilter(noStr)
	assert.False(t, s1.acceptNode(newNode(varStr)))
	assert.True(t, s1.acceptNode(newNode(varStruct)))

	s2 := NewScanner(nil).WithFilter(noStruct)
	assert.True(t, s2.acceptNode(newNode(varStr)))
	assert.False(t, s2.acceptNode(newNode(varStruct)))

	s3 := NewScanner(nil).WithFilter(noStr).WithFilter(noStruct)
	assert.False(t, s3.acceptNode(newNode(varStr)))
	assert.False(t, s3.acceptNode(newNode(varStruct)))
}

func TestDecomposePrimitives(t *testing.T) {
	s := NewScanner(nil).WithParallism(4)

	s.decompose(context.Background(), nil, reflect.ValueOf(1), nil)
	s.decompose(context.Background(), nil, reflect.ValueOf(true), nil)
	s.decompose(context.Background(), nil, reflect.ValueOf("test"), nil)
	s.decompose(context.Background(), nil, reflect.ValueOf(nil), nil)

	// should store all primitives
	assert.Len(t, s.nodes, 4)

	// primitives has no children
	intNode := s.Node(<-s.nodeIDCh)
	assert.Len(t, intNode.Children, 0)
	boolNode := s.Node(<-s.nodeIDCh)
	assert.Len(t, boolNode.Children, 0)
	strNode := s.Node(<-s.nodeIDCh)
	assert.Len(t, strNode.Children, 0)
	nilNode := s.Node(<-s.nodeIDCh)
	assert.Len(t, nilNode.Children, 0)
}

func TestDecomposeInterface(t *testing.T) {
	s := NewScanner(nil).WithParallism(1)

	func(iface interface{}) {
		s.decompose(context.Background(), nil, reflect.ValueOf(iface), nil)
	}("fake obj")

	ifaceNode := s.Node(<-s.nodeIDCh)
	assert.Len(t, ifaceNode.Children, 0)
	// the kind won't be affected by where the value is being used
	// it's still considered as a string
	assert.Equal(t, reflect.String, ifaceNode.Kind())
}

func TestDecomposeInterfaceStruct(t *testing.T) {
	dummyNode := testStruct{
		Exported:    1,
		nonExported: 2,
	}
	s := NewScanner(nil).WithParallism(4)

	func(iface interface{}) {
		s.decompose(context.Background(), nil, reflect.ValueOf(iface), nil)
	}(&dummyNode)

	assert.Len(t, s.nodes, 4)
	_ = <-s.nodeIDCh
	_ = <-s.nodeIDCh
	ifaceNode := s.Node(<-s.nodeIDCh)
	assert.Len(t, ifaceNode.Fields, 2)
	assert.Len(t, ifaceNode.Children, 2)
	assert.Equal(t, reflect.Struct, ifaceNode.Kind())
	ptrNode := s.Node(<-s.nodeIDCh)
	assert.Len(t, ptrNode.Children, 1)
}

func TestDecomposePtr(t *testing.T) {
	str := "string"
	strPtr1 := &str
	strPtr2 := &str
	s := NewScanner(nil).WithParallism(2)

	s.decompose(context.Background(), nil, reflect.ValueOf(strPtr1), nil)
	assert.Len(t, s.nodes, 2)

	// same pointer is stored only once
	s.decompose(context.Background(), nil, reflect.ValueOf(strPtr2), nil)
	assert.Len(t, s.nodes, 2)

	// string should has no children
	strNode := s.Node(<-s.nodeIDCh)
	assert.Len(t, strNode.Children, 0)

	// strPtr should have children
	strPtrNode := s.Node(<-s.nodeIDCh)
	assert.Len(t, strPtrNode.Children, 1)
}

func TestDecomposerSimpleArray(t *testing.T) {
	arr := []int{1, 2, 3}
	s := NewScanner(nil).WithParallism(4)

	s.decompose(context.Background(), nil, reflect.ValueOf(arr), nil)
	assert.Len(t, s.nodes, 4)

	_ = <-s.nodeIDCh
	_ = <-s.nodeIDCh
	_ = <-s.nodeIDCh

	// array node should have 3 children
	arrNode := s.Node(<-s.nodeIDCh)
	assert.Len(t, arrNode.Children, 3)
	assert.Len(t, arrNode.Array, 3)
	// check array element by order
	assert.Equal(t, 1, int(arrNode.Array[0].value.Int()))
	assert.Equal(t, 2, int(arrNode.Array[1].value.Int()))
	assert.Equal(t, 3, int(arrNode.Array[2].value.Int()))
}

type testStruct struct {
	Exported    int
	nonExported int
}

func TestDecomposeSimpleStruct(t *testing.T) {
	dummyNode := testStruct{
		Exported:    1,
		nonExported: 2,
	}

	s := NewScanner(nil).WithParallism(5)

	s.decompose(context.Background(), nil, reflect.ValueOf(dummyNode), nil)
	assert.Len(t, s.nodes, 3)

	// exported int node should has no children
	intNode1 := s.Node(<-s.nodeIDCh)
	assert.Len(t, intNode1.Children, 0)

	// unexported int node should have no children
	intNode2 := s.Node(<-s.nodeIDCh)
	assert.Len(t, intNode2.Children, 0)

	// struct node should has no children
	structNode := s.Node(<-s.nodeIDCh)
	assert.Len(t, structNode.Children, 2)
}

func TestStructFields(t *testing.T) {
	dummyNode := testStruct{
		Exported:    1,
		nonExported: 2,
	}

	s := NewScanner(nil).WithParallism(5)

	s.decompose(context.Background(), nil, reflect.ValueOf(dummyNode), nil)
	assert.Len(t, s.nodes, 3)

	_ = <-s.nodeIDCh
	_ = <-s.nodeIDCh
	structNode := s.Node(<-s.nodeIDCh)
	assert.Len(t, structNode.Fields, 2)
	assert.Contains(t, structNode.Fields, "Exported")
	assert.Contains(t, structNode.Fields, "nonExported")
	assert.Contains(t, structNode.Children, structNode.Fields["Exported"])
	assert.Contains(t, structNode.Children, structNode.Fields["nonExported"])
}

func TestDecompositionSimpleMap(t *testing.T) {
	m := make(map[string]int)
	m["foo"] = 1
	s := NewScanner(nil).WithParallism(3)

	s.decompose(context.Background(), nil, reflect.ValueOf(m), nil)
	assert.Len(t, s.nodes, 3)

	kNode := s.Node(<-s.nodeIDCh)
	assert.Len(t, kNode.Children, 0)
	vNode := s.Node(<-s.nodeIDCh)
	assert.Len(t, vNode.Children, 0)

	// switch order for KV node if got type mismatch
	if s.getNodeID(kNode).typeStr != "/string/string" {
		kNode, vNode = vNode, kNode
	}

	mapNode := s.Node(<-s.nodeIDCh)
	assert.Len(t, mapNode.Children, 1)

	assert.Len(t, s.maps, 1)
	assert.Len(t, s.maps[s.getNodeID(mapNode)], 1)
	assert.Equal(t, s.getNodeID(vNode), s.maps[s.getNodeID(mapNode)][s.getNodeID(kNode)])
}

func TestDecompositionMapMultiVal(t *testing.T) {
	m := make(map[string]int)
	m["foo"] = 1
	m["bar"] = 2
	s := NewScanner(nil).WithParallism(100)
	s.decompose(context.Background(), nil, reflect.ValueOf(m), nil)
	_ = <-s.nodeIDCh
	_ = <-s.nodeIDCh
	_ = <-s.nodeIDCh
	_ = <-s.nodeIDCh
	mapId, ok := <-s.nodeIDCh
	assert.True(t, ok)

	assert.Len(t, s.nodes, 5)
	assert.Len(t, s.maps, 1)
	assert.Len(t, s.maps[mapId], 2)
}

// KV values for map are stored as different copies as the address of the
// variables are different on stack
func TestDecompositionMultiMapSameValue(t *testing.T) {
	foo := "foo"
	bar := "bar"
	m1 := make(map[string]*string)
	m1[foo] = &bar
	m2 := make(map[string]*string)
	m2[foo] = &bar
	s := NewScanner(nil).WithParallism(6)

	// add "foo", ptr to "bar", "bar" and the map
	s.decompose(context.Background(), nil, reflect.ValueOf(m1), nil)
	assert.Len(t, s.nodes, 4)
	// add new map m2 and another instance of "foo"
	// pointer and bar are shared
	s.decompose(context.Background(), nil, reflect.ValueOf(m2), nil)
	assert.Len(t, s.nodes, 4+2)

	// if just shallow copy the map, should not add anything as the map
	// itself is also a reference
	m3 := m1
	s.decompose(context.Background(), nil, reflect.ValueOf(m3), nil)
	assert.Len(t, s.nodes, 4+2+0)
}

func TestDecompositionMapSimpleStruct(t *testing.T) {
	m := make(map[string]testStruct)
	m["foo"] = testStruct{1, 2}
	s := NewScanner(nil).WithParallism(5)

	s.decompose(context.Background(), nil, reflect.ValueOf(m), nil)
	assert.Len(t, s.nodes, 5)
	assert.Len(t, s.maps, 1)

	var structNode, mapNode *Node
	for {
		n := s.Node(<-s.nodeIDCh)
		switch n.Kind() {
		case reflect.Struct:
			structNode = n
		case reflect.Map:
			mapNode = n
		}
		if structNode != nil && mapNode != nil {
			break
		}
	}
	assert.Len(t, mapNode.Children, 1)
	assert.Len(t, structNode.Children, 2)
}

type nestedStruct struct {
	ts  testStruct
	tsp *testStruct
}

func TestDecompositionNestedStruct(t *testing.T) {
	innerStruct := testStruct{10, 20}
	testObj1 := nestedStruct{
		ts:  innerStruct, // notice this struct will be copied
		tsp: &innerStruct,
	}
	testObj2 := nestedStruct{
		ts:  innerStruct, // notice this struct will be copied
		tsp: nil,
	}

	s1 := NewScanner(nil).WithParallism(8)
	s1.decompose(context.Background(), nil, reflect.ValueOf(testObj1), nil)
	assert.Len(t, s1.nodes, 8)

	s2 := NewScanner(nil).WithParallism(5)
	s2.decompose(context.Background(), nil, reflect.ValueOf(testObj2), nil)
	assert.Len(t, s2.nodes, 5)
}

func TestSimpleScan(t *testing.T) {
	s := NewScanner(nil)
	c := s.Scan("foo", "bar")

	_, ok := <-c
	assert.True(t, ok)
	_, ok = <-c
	assert.True(t, ok)
	_, ok = <-c
	assert.False(t, ok)
}

type list struct {
	next *list
}

func TestLinkedList(t *testing.T) {
	// create a loop
	head := &list{}
	head.next = &list{}
	head.next.next = head

	s1 := NewScanner(nil).WithParallism(6)
	s1.decompose(context.Background(), nil, reflect.ValueOf(head), nil)
	assert.Len(t, s1.nodes, 6)
}

func TestContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := NewScanner(ctx)
	// channel should be blocked on the first node ID
	c := s.Scan("foo", "bar", "deadbeef")
	// before the first node ID got fetched from the channel, cancel the job
	// no further node ID should be fetched after the first one
	cancel()

	select {
	// #1: should never be able to fetch from channel as it should
	// have been closed as of cancel
	case _, ok := <-c:
		assert.True(t, ok)
		assert.FailNow(t, "should have never hit this")
	// #2: 2nd fetch should fail
	default:
		return
	}
	assert.FailNow(t, "should have never hit this either")
}

func TestScannerNodes(t *testing.T) {
	s := NewScanner(nil)
	_ = s.Scan(nil)

	assert.Equal(t, s.nodes, s.Nodes())
}

func TestScannerMaps(t *testing.T) {
	s := NewScanner(nil)
	_ = s.Scan(nil)

	assert.Equal(t, s.maps, s.Maps())
}

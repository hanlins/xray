package dot

import (
	"testing"

	"github.com/awalterschulze/gographviz"
	"github.com/hanlins/xray"
	"github.com/stretchr/testify/assert"
)

func TestNewGraphInfo(t *testing.T) {
	s := xray.NewScanner(nil)
	nodeCh := s.Scan(100)
	_ = <-nodeCh

	gi := NewGraphInfo(s)
	assert.Len(t, gi.Nodes, 1)
	assert.Len(t, gi.Maps, 0)
}

func TestPrimitiveLabel(t *testing.T) {
	s := xray.NewScanner(nil)
	nodeCh := s.Scan(100)
	intID := <-nodeCh

	assert.Regexp(t, "<*>100", labelPrimitive(intID))
}

func validateGraph(g *gographviz.Graph) error {
	_, err := gographviz.ParseString(g.String())
	return err
}

func TestProcessPrimitive(t *testing.T) {
	s := xray.NewScanner(nil)
	nodeCh := s.Scan(100)
	nid, ok := <-nodeCh
	assert.True(t, ok)

	gi := NewGraphInfo(s)
	p := &DefaultHandler{*NewProcessor()}
	p.Process(gi, nid)
	g, err := p.Render()
	assert.NoError(t, err)
	assert.NotNil(t, g)
	assert.True(t, g.Directed)
	assert.False(t, g.Strict)
	assert.NotNil(t, g.Nodes)
	assert.Len(t, g.Nodes.Nodes, 1)
	assert.NotNil(t, g.Edges)
	assert.Len(t, g.Edges.Edges, 0)
	assert.NotNil(t, g.SubGraphs)
	assert.Len(t, g.SubGraphs.SubGraphs, 0)
	assert.NotNil(t, g.Relations)
	assert.Len(t, g.Relations.ParentToChildren, 1)
	assert.Len(t, g.Relations.ChildToParents, 1)
	assert.NoError(t, validateGraph(g))

	_, ok = <-nodeCh
	assert.False(t, ok)
}

func TestProcessPointer(t *testing.T) {
	str := "deadbeef"
	s := xray.NewScanner(nil)
	nodeCh := s.Scan(&str)
	gi := NewGraphInfo(s)
	p := &DefaultHandler{*NewProcessor()}

	sid, ok := <-nodeCh
	assert.True(t, ok)
	p.Process(gi, sid)

	pid, ok := <-nodeCh
	assert.True(t, ok)
	p.Process(gi, pid)

	g, err := p.Render()
	assert.NoError(t, err)
	assert.NotNil(t, g)
	assert.True(t, g.Directed)
	assert.False(t, g.Strict)
	assert.NotNil(t, g.Nodes)
	assert.Len(t, g.Nodes.Nodes, 2)
	assert.NotNil(t, g.Edges)
	assert.Len(t, g.Edges.Edges, 1)
	assert.NotNil(t, g.SubGraphs)
	assert.Len(t, g.SubGraphs.SubGraphs, 0)
	assert.NotNil(t, g.Relations)
	assert.Len(t, g.Relations.ParentToChildren, 1)
	assert.Len(t, g.Relations.ChildToParents, 2)
	assert.NoError(t, validateGraph(g))

	_, ok = <-nodeCh
	assert.False(t, ok)
}

func TestProcessMap(t *testing.T) {
	m := map[string]string{"foo": "bar"}
	m["dead"] = "beef"
	s := xray.NewScanner(nil)
	nodeCh := s.Scan(m)
	gi := NewGraphInfo(s)
	p := &DefaultHandler{*NewProcessor()}

	for i := 0; i < 5; i++ {
		id, ok := <-nodeCh
		assert.True(t, ok)
		p.Process(gi, id)
	}
	_, ok := <-nodeCh
	assert.False(t, ok)

	g, err := p.Render()
	assert.NoError(t, err)
	assert.NotNil(t, g)
	assert.True(t, g.Directed)
	assert.False(t, g.Strict)
	assert.NotNil(t, g.Nodes)
	assert.Len(t, g.Nodes.Nodes, 5)
	assert.NotNil(t, g.Edges)
	assert.Len(t, g.Edges.Edges, 4)
	assert.NotNil(t, g.SubGraphs)
	assert.Len(t, g.SubGraphs.SubGraphs, 1)
	assert.NotNil(t, g.Relations)
	assert.Len(t, g.Relations.ParentToChildren, 2)
	assert.Len(t, g.Relations.ChildToParents, 5)
	assert.NoError(t, validateGraph(g))

	_, ok = <-nodeCh
	assert.False(t, ok)
}

type test1 struct {
	p   *string
	arr []int
	m   map[string]int
}

func TestProcessStruct(t *testing.T) {
	str := "deadbeef"
	ts := test1{
		p:   &str,
		arr: []int{80, 40, 8840},
		m:   map[string]int{"foo": 1, "bar": 2},
	}
	s := xray.NewScanner(nil)
	nodeCh := s.Scan(ts)
	gi := NewGraphInfo(s)
	p := &DefaultHandler{*NewProcessor()}

	for i := 0; i < 12; i++ {
		id, ok := <-nodeCh
		assert.True(t, ok)
		p.Process(gi, id)
	}
	_, ok := <-nodeCh

	assert.False(t, ok)
	g, err := p.Render()
	assert.NoError(t, err)
	assert.NotNil(t, g)
	assert.True(t, g.Directed)
	assert.False(t, g.Strict)
	assert.NotNil(t, g.Nodes)
	assert.Len(t, g.Nodes.Nodes, 8)
	assert.NotNil(t, g.Edges)
	assert.Len(t, g.Edges.Edges, 7)
	assert.NotNil(t, g.SubGraphs)
	assert.Len(t, g.SubGraphs.SubGraphs, 1)
	assert.NotNil(t, g.Relations)
	assert.Len(t, g.Relations.ParentToChildren, 2)
	assert.Len(t, g.Relations.ChildToParents, 8)
	assert.NoError(t, validateGraph(g))

	_, ok = <-nodeCh
	assert.False(t, ok)
}

type listnode struct {
	val  int
	next *listnode
}

func TestDrawLinkedListLoop(t *testing.T) {
	l1 := &listnode{val: 1}
	l1.next = &listnode{val: 2}
	l1.next.next = &listnode{val: 3}
	l1.next.next.next = l1

	s := xray.NewScanner(nil)
	nodeCh := s.Scan(l1)

	g, err := Draw(NewGraphInfo(s), nodeCh, nil)
	assert.NoError(t, err)
	assert.NotNil(t, g)
}

func BenchmarkDrawStruct(b *testing.B) {
	l1 := &listnode{val: 1}
	l1.next = &listnode{val: 2}
	l1.next.next = &listnode{val: 3}
	l1.next.next.next = l1

	s := xray.NewScanner(nil)
	nodeCh := s.Scan(l1)

	Draw(NewGraphInfo(s), nodeCh, nil)
}

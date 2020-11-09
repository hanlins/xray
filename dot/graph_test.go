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
	intId := <-nodeCh

	assert.Regexp(t, "<*> 100", labelPrimitive(intId))
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
	p.Render()
	assert.NotNil(t, p.graph)
	assert.True(t, p.graph.Directed)
	assert.False(t, p.graph.Strict)
	assert.NotNil(t, p.graph.Nodes)
	assert.Len(t, p.graph.Nodes.Nodes, 1)
	assert.NotNil(t, p.graph.Edges)
	assert.Len(t, p.graph.Edges.Edges, 0)
	assert.NotNil(t, p.graph.SubGraphs)
	assert.Len(t, p.graph.SubGraphs.SubGraphs, 0)
	assert.NotNil(t, p.graph.Relations)
	assert.Len(t, p.graph.Relations.ParentToChildren, 1)
	assert.Len(t, p.graph.Relations.ChildToParents, 1)
	assert.NoError(t, validateGraph(p.graph))

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
	p.Render()

	assert.NotNil(t, p.graph)
	assert.True(t, p.graph.Directed)
	assert.False(t, p.graph.Strict)
	assert.NotNil(t, p.graph.Nodes)
	assert.Len(t, p.graph.Nodes.Nodes, 2)
	assert.NotNil(t, p.graph.Edges)
	assert.Len(t, p.graph.Edges.Edges, 1)
	assert.NotNil(t, p.graph.SubGraphs)
	assert.Len(t, p.graph.SubGraphs.SubGraphs, 0)
	assert.NotNil(t, p.graph.Relations)
	assert.Len(t, p.graph.Relations.ParentToChildren, 1)
	assert.Len(t, p.graph.Relations.ChildToParents, 2)
	assert.NoError(t, validateGraph(p.graph))

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
	p.Render()

	assert.NotNil(t, p.graph)
	assert.True(t, p.graph.Directed)
	assert.False(t, p.graph.Strict)
	assert.NotNil(t, p.graph.Nodes)
	assert.Len(t, p.graph.Nodes.Nodes, 5)
	assert.NotNil(t, p.graph.Edges)
	assert.Len(t, p.graph.Edges.Edges, 4)
	assert.NotNil(t, p.graph.SubGraphs)
	assert.Len(t, p.graph.SubGraphs.SubGraphs, 1)
	assert.NotNil(t, p.graph.Relations)
	assert.Len(t, p.graph.Relations.ParentToChildren, 2)
	assert.Len(t, p.graph.Relations.ChildToParents, 5)
	assert.NoError(t, validateGraph(p.graph))

	_, ok = <-nodeCh
	assert.False(t, ok)
}

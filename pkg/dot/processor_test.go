package dot

import (
	"testing"

	"github.com/awalterschulze/gographviz"
	"github.com/hanlins/objscan/pkg/scan"
	"github.com/stretchr/testify/assert"
)

func TestNewGraphInfo(t *testing.T) {
	s := scan.NewScanner(nil)
	nodeCh := s.Scan(100)
	_ = <-nodeCh

	gi := NewGraphInfo(s, "")
	assert.Len(t, gi.Nodes, 1)
	assert.Len(t, gi.Maps, 0)
	assert.NotNil(t, gi.Graph)
	assert.Equal(t, GraphName, gi.Name)
	assert.Equal(t, GraphName, gi.Graph.Name)
}

func TestGraphWithName(t *testing.T) {
	s := scan.NewScanner(nil)
	nodeCh := s.Scan(100)
	_ = <-nodeCh

	customName := "deadbeef"
	gi := NewGraphInfo(s, customName)
	assert.Equal(t, customName, gi.Name)
	assert.Equal(t, customName, gi.Graph.Name)
}

func TestPrimitiveLabel(t *testing.T) {
	s := scan.NewScanner(nil)
	nodeCh := s.Scan(100)
	intId := <-nodeCh

	assert.Regexp(t, "<*> 100", labelPrimitive(intId))
}

func validateGraph(g *gographviz.Graph) error {
	_, err := gographviz.ParseString(g.String())
	return err
}

func TestProcessPrimitive(t *testing.T) {
	s := scan.NewScanner(nil)
	nodeCh := s.Scan(100)
	nid, ok := <-nodeCh
	assert.True(t, ok)

	gi := NewGraphInfo(s, "")
	p := &primitiveProcessor{}
	p.Process(gi, nid)
	assert.NotNil(t, gi.Graph)
	assert.True(t, gi.Graph.Directed)
	assert.False(t, gi.Graph.Strict)
	assert.NotNil(t, gi.Graph.Nodes)
	assert.Len(t, gi.Graph.Nodes.Nodes, 1)
	assert.NotNil(t, gi.Graph.Edges)
	assert.Len(t, gi.Graph.Edges.Edges, 0)
	assert.NotNil(t, gi.Graph.SubGraphs)
	assert.Len(t, gi.Graph.SubGraphs.SubGraphs, 0)
	assert.NotNil(t, gi.Graph.Relations)
	assert.Len(t, gi.Graph.Relations.ParentToChildren, 1)
	assert.Len(t, gi.Graph.Relations.ChildToParents, 1)
	assert.NoError(t, validateGraph(gi.Graph))

	_, ok = <-nodeCh
	assert.False(t, ok)
}

func TestProcessPointer(t *testing.T) {
	str := "deadbeef"
	s := scan.NewScanner(nil)
	nodeCh := s.Scan(&str)
	gi := NewGraphInfo(s, "")

	sid, ok := <-nodeCh
	assert.True(t, ok)
	p1 := &primitiveProcessor{}
	p1.Process(gi, sid)

	pid, ok := <-nodeCh
	assert.True(t, ok)

	p2 := &ptrProcessor{}
	p2.Process(gi, pid)

	assert.NotNil(t, gi.Graph)
	assert.True(t, gi.Graph.Directed)
	assert.False(t, gi.Graph.Strict)
	assert.NotNil(t, gi.Graph.Nodes)
	assert.Len(t, gi.Graph.Nodes.Nodes, 2)
	assert.NotNil(t, gi.Graph.Edges)
	assert.Len(t, gi.Graph.Edges.Edges, 1)
	assert.NotNil(t, gi.Graph.SubGraphs)
	assert.Len(t, gi.Graph.SubGraphs.SubGraphs, 0)
	assert.NotNil(t, gi.Graph.Relations)
	assert.Len(t, gi.Graph.Relations.ParentToChildren, 1)
	assert.Len(t, gi.Graph.Relations.ChildToParents, 2)
	assert.NoError(t, validateGraph(gi.Graph))

	_, ok = <-nodeCh
	assert.False(t, ok)

}

package dot

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/hanlins/objscan/pkg/scan"
	"github.com/stretchr/testify/assert"
)

func TestNewProcessor(t *testing.T) {
	p := newProcessor()

	assert.NotNil(t, p)
	assert.NotNil(t, p.lock)
	assert.NotNil(t, p.nodeRef)
	assert.NotNil(t, p.nodes)
	assert.NotNil(t, p.edges)
	assert.NotNil(t, p.subgraphs)
	assert.NotNil(t, p.graph)
	assert.Equal(t, GraphName, p.name)
	assert.Len(t, p.nodeRef, 0)
	assert.Len(t, p.nodes, 0)
	assert.Len(t, p.edges, 0)
	assert.Len(t, p.subgraphs, 0)
}

// getTypeID returns the unique identifier for the type
func getTypeID(t reflect.Type) string {
	return fmt.Sprintf("%s/%s/%s", t.PkgPath(), t.Name(), t.String())
}

// getObjNodeID returns the ID for an non-nil object
func getObjNodeID(obj interface{}) scan.NodeID {
	node := scan.NewNode(reflect.ValueOf(obj))
	return node.NodeID(getTypeID)
}

func TestRegisterNodeReference(t *testing.T) {
	p := newProcessor()
	id1 := getObjNodeID(100)
	p.setNodeRef(id1, id1.Hash())

	assert.Len(t, p.nodeRef, 1)
	assert.Equal(t, id1.Hash(), p.nodeRef[id1])
	// update node reference
	p.setNodeRef(id1, "deadbeef")
	assert.Len(t, p.nodeRef, 1)
	assert.Equal(t, "deadbeef", p.nodeRef[id1])
}

func TestRegisterNode(t *testing.T) {
	p := newProcessor()
	id1 := getObjNodeID(100)
	id2 := getObjNodeID("deadbeef")

	p.registerNode(id1, nil, map[string]string{"foo": "bar"})
	assert.Len(t, p.nodes, 1)
	assert.Equal(t, map[string]string{"foo": "bar"}, p.nodes[id1].attr)
	p.registerNode(id2, &id1, nil)
	assert.Len(t, p.nodes, 2)
}

func TestRegisterEdge(t *testing.T) {
	p := newProcessor()
	id1 := getObjNodeID(100)
	id2 := getObjNodeID("foo")
	p.registerEdge(id1, id2, nil)

	assert.Len(t, p.edges, 1)
	// re-registration won't take effect
	p.registerEdge(id1, id2, nil)
	assert.Len(t, p.edges, 1)
}

func TestRegisterSubgraph(t *testing.T) {
	p := newProcessor()
	id1 := getObjNodeID(100)
	id2 := getObjNodeID("foo")
	p.registerEdge(id1, id2, nil)

	assert.Len(t, p.edges, 1)
	p.registerSubgraph(id1, &id2)
	assert.Len(t, p.subgraphs, 1)
	p.registerSubgraph(id2, nil)
	assert.Len(t, p.subgraphs, 2)
}

func TestRenderEmpty(t *testing.T) {
	p := newProcessor()
	err := p.render()

	assert.Equal(t, GraphName, p.graph.Name)
	assert.True(t, p.graph.Directed)
	assert.False(t, p.graph.Strict)
	assert.NotNil(t, p.graph.Nodes)
	assert.Len(t, p.graph.Nodes.Nodes, 0)
	assert.NotNil(t, p.graph.Edges)
	assert.Len(t, p.graph.Edges.Edges, 0)
	assert.NotNil(t, p.graph.SubGraphs)
	assert.Len(t, p.graph.SubGraphs.SubGraphs, 0)
	assert.NotNil(t, p.graph.Relations)
	assert.Len(t, p.graph.Relations.ParentToChildren, 0)
	assert.Len(t, p.graph.Relations.ChildToParents, 0)
	assert.NoError(t, err)
}

package dot

import (
	"fmt"

	dot "github.com/awalterschulze/gographviz"
	"github.com/hanlins/objscan/pkg/scan"
)

const (
	GraphName = "G"
)

type GraphInfo struct {
	// scanned results, all nodes that has been scanned
	Nodes map[scan.NodeID]*scan.Node
	// maps stores the KV pairs for each map
	Maps map[scan.NodeID]map[scan.NodeID]scan.NodeID
	// graph stores the dot formatted graph to be rendered
	Graph *dot.Graph
	// name of the graph
	Name string
}

func NewGraphInfo(s *scan.Scanner, name string) *GraphInfo {
	gi := &GraphInfo{}
	gi.Nodes = s.Nodes()
	gi.Maps = s.Maps()

	gi.Graph = dot.NewGraph()
	if name == "" {
		gi.Name = GraphName
	} else {
		gi.Name = name
	}
	gi.Graph.SetName(gi.Name)
	gi.Graph.SetDir(true)
	return gi
}

func (gi *GraphInfo) WithName(name string) *GraphInfo {
	if name == "" {
		return gi
	}
	gi.Name = name
	gi.Graph.SetName(name)
	return gi
}

// Processor is the interface for hanling a specific node
// it can read the global nodes information and write to the graph
type Processor interface {
	Process(*GraphInfo, scan.NodeID)
}

type primitiveProcessor struct{}

func setRecord(attr map[string]string) {
	attr["shape"] = "record"
}

func labelPrimitive(id scan.NodeID) string {
	return fmt.Sprintf("<%s> %s", id.Hash(), id.String())
}

func (p *primitiveProcessor) Process(g *GraphInfo, id scan.NodeID) {
	idHash := id.Hash()

	attr := map[string]string{}
	setRecord(attr)
	attr["label"] = labelPrimitive(id)
	g.Graph.AddNode(g.Name, idHash, attr)
}

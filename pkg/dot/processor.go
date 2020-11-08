package dot

import (
	"fmt"
	"sync"

	"github.com/awalterschulze/gographviz"
	"github.com/hanlins/objscan/pkg/scan"
)

const (
	GraphName = "G"
)

type processor struct {
	lock *sync.Mutex

	// nodeRef keeps track of the ID to reference mapping
	// the mapping will be used to reference the node in the graph
	// e.g. creating an edge
	nodeRef map[scan.NodeID]string
	// nodes stores the information to construct the node
	nodes map[scan.NodeID]nodeInfo
	// edges keeps track of the edges to be rendered
	// key is the source->dst pair, value is the attribute of the edge
	edges map[edgePair]map[string]string
	// subgraphs keeps track of the subgraphs to be rendered
	// key is the child graph and value is its parent
	// if parent is nil then add the child to the default graph
	subgraphs map[scan.NodeID]*scan.NodeID

	// graph is used for dot graph rendering
	graph *gographviz.Graph
	// name is the name of the graph
	name string
}

// nodeInfo stores the necessary information to construct a node in dot format
// if graph points to nil then the node should be added to the outmost graph
type nodeInfo struct {
	graph *scan.NodeID
	attr  map[string]string
}

// edgePair describes the source and desgination of an edge
// it's also used to identify the edge
type edgePair struct {
	src scan.NodeID
	dst scan.NodeID
}

// newProcessor initiates a processor instance
func newProcessor() *processor {
	p := &processor{}
	p.lock = &sync.Mutex{}
	p.nodeRef = make(map[scan.NodeID]string)
	p.nodes = make(map[scan.NodeID]nodeInfo)
	p.edges = make(map[edgePair]map[string]string)
	p.subgraphs = make(map[scan.NodeID]*scan.NodeID)
	p.graph = gographviz.NewGraph()
	p.name = GraphName
	return p
}

// setNodeRef maps a node ID to its reference string
func (p *processor) setNodeRef(id scan.NodeID, ref string) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.nodeRef[id] = ref
}

// registerNode registers a node
func (p *processor) registerNode(node scan.NodeID, graph *scan.NodeID, attr map[string]string) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.nodes[node] = nodeInfo{graph: graph, attr: attr}
}

// registerEdge registers an edge
func (p *processor) registerEdge(src, dst scan.NodeID, attr map[string]string) {
	p.lock.Lock()
	defer p.lock.Unlock()

	ep := edgePair{src: src, dst: dst}
	p.edges[ep] = attr
}

// registerSubgraph registers an subgraph
func (p *processor) registerSubgraph(child scan.NodeID, parent *scan.NodeID) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.subgraphs[child] = parent
}

// getParentGraph returns the name of the parent graph
func (p *processor) getParentGraph(parent *scan.NodeID) string {
	if parent != nil {
		return parent.Hash()
	}
	return p.name
}

// render add the observed graph objects to the current graph
func (p *processor) render() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.graph.SetName(p.name)
	p.graph.SetDir(true)

	// add subgraphs
	for node, graph := range p.subgraphs {
		err := p.graph.AddSubGraph(p.getParentGraph(graph), node.Hash(), nil)
		if err != nil {
			return err
		}
	}
	// add nodes
	for id, info := range p.nodes {
		err := p.graph.AddNode(p.getParentGraph(info.graph), id.Hash(), info.attr)
		if err != nil {
			return err
		}
	}
	// add edges
	for ep, attr := range p.edges {
		srcRef, exist := p.nodeRef[ep.src]
		if !exist {
			return fmt.Errorf("failed to find node reference for src '%#v'", ep.src)
		}
		dstRef, exist := p.nodeRef[ep.src]
		if !exist {
			return fmt.Errorf("failed to find node reference for dst '%#v'", ep.dst)
		}
		err := p.graph.AddEdge(srcRef, dstRef, true, attr)
		if err != nil {
			return err
		}
	}
	return nil
}

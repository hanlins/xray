package dot

import (
	"fmt"
	"strings"
	"sync"

	"github.com/awalterschulze/gographviz"
	"github.com/hanlins/xray"
)

const (
	GraphName = "G"
)

type Processor struct {
	lock *sync.Mutex

	// nodeRef keeps track of the ID to reference mapping
	// the mapping will be used to reference the node in the graph
	// e.g. creating an edge
	nodeRef map[xray.NodeID]string
	// nodes stores the information to construct the node
	nodes map[xray.NodeID]NodeInfo
	// edges keeps track of the edges to be rendered
	// key is the source->dst pair, value is the attribute of the edge
	edges map[EdgePair]map[string]string
	// subgraphs keeps track of the subgraphs to be rendered
	// key is the child graph and value is its parent
	// if parent is nil then add the child to the default graph
	subgraphs map[xray.NodeID]*xray.NodeID

	// graph is used for dot graph rendering
	graph *gographviz.Graph
	// name is the name of the graph
	name string
}

// NodeInfo stores the necessary information to construct a node in dot format
// if graph points to nil then the node should be added to the outmost graph
type NodeInfo struct {
	graph *xray.NodeID
	attr  map[string]string
}

// EdgePair describes the source and desgination of an edge
// it's also used to identify the edge
type EdgePair struct {
	src xray.NodeID
	dst xray.NodeID

	srcField string
}

// NewProcessor initiates a Processor instance
func NewProcessor() *Processor {
	p := &Processor{}
	p.lock = &sync.Mutex{}
	p.nodeRef = make(map[xray.NodeID]string)
	p.nodes = make(map[xray.NodeID]NodeInfo)
	p.edges = make(map[EdgePair]map[string]string)
	p.subgraphs = make(map[xray.NodeID]*xray.NodeID)
	p.graph = gographviz.NewGraph()
	p.name = GraphName
	return p
}

// setnodeRef maps a node ID to its reference string
func (p *Processor) setNodeRef(id xray.NodeID, ref string) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.nodeRef[id] = ref
}

// AddNode registers a node
func (p *Processor) AddNode(node xray.NodeID, graph *xray.NodeID, attr map[string]string) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if ni, exist := p.nodes[node]; exist && attr == nil {
		attr = ni.attr
	}
	p.nodes[node] = NodeInfo{graph: graph, attr: attr}
}

// RemoverNode removes the node
func (p *Processor) RemoveNode(node xray.NodeID) {
	p.lock.Lock()
	defer p.lock.Unlock()

	delete(p.nodes, node)
}

// AddEdge registers an edge
func (p *Processor) AddEdge(src, dst xray.NodeID, srcField string, attr map[string]string) {
	p.lock.Lock()
	defer p.lock.Unlock()

	ep := EdgePair{src: src, dst: dst, srcField: srcField}
	p.edges[ep] = attr
}

// AddSubgraph registers an subgraph
func (p *Processor) AddSubgraph(child xray.NodeID, parent *xray.NodeID) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.subgraphs[child] = parent
}

// getParentGraph returns the name of the parent graph
func (p *Processor) getParentGraph(parent *xray.NodeID) string {
	if parent != nil {
		return p.nodeRef[*parent]
	}
	return p.name
}

// breakEdgeEndpoint breaks the node reference to a dot node and its port
func breakEdgeEndpoint(nodeRef string) (string, string) {
	if strings.Contains(nodeRef, ":") {
		split := strings.Split(nodeRef, ":")
		return split[0], split[1]
	}
	return nodeRef, ""
}

// Render add the observed graph objects to the current graph
func (p *Processor) Render() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.graph.SetName(p.name)
	p.graph.SetDir(true)

	// add subgraphs
	for node, graph := range p.subgraphs {
		err := p.graph.AddSubGraph(p.getParentGraph(graph), p.nodeRef[node], nil)
		if err != nil {
			return err
		}
	}
	// add nodes
	for id, info := range p.nodes {
		nodeRef := p.nodeRef[id]
		// skip merged fields
		if strings.Contains(nodeRef, ":") {
			continue
		}

		err := p.graph.AddNode(p.getParentGraph(info.graph), nodeRef, info.attr)
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
		if ep.srcField != "" {
			srcRef = fmt.Sprintf("%s:%s", srcRef, ep.srcField)
		}
		dstRef, exist := p.nodeRef[ep.dst]
		if !exist {
			return fmt.Errorf("failed to find node reference for dst '%#v'", ep.dst)
		}
		srcNode, srcPort := breakEdgeEndpoint(srcRef)
		dstNode, dstPort := breakEdgeEndpoint(dstRef)
		err := p.graph.AddPortEdge(srcNode, srcPort, dstNode, dstPort, true, attr)
		if err != nil {
			return err
		}
	}
	return nil
}

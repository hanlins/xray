package dot

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/awalterschulze/gographviz"
	"github.com/hanlins/xray"
)

const (
	// Label marks the label key for the node
	Label = "label"
)

// GraphInfo contains the raw node data collected without being processed
type GraphInfo struct {
	// scanned results, all nodes that has been scanned
	Nodes map[xray.NodeID]*xray.Node
	// maps stores the KV pairs for each map
	Maps map[xray.NodeID]map[xray.NodeID]xray.NodeID
}

// NewGraphInfo construct the graph info from scanner
func NewGraphInfo(s *xray.Scanner) *GraphInfo {
	gi := &GraphInfo{}
	gi.Nodes = s.Nodes()
	gi.Maps = s.Maps()

	return gi
}

// Handler is the interface for hanling a specific node
// it can read the global nodes information, and it's can use its custome logic
// to manage the objects to be rendered
type Handler interface {
	Process(*GraphInfo, xray.NodeID)
	Render() (*gographviz.Graph, error)
}

// DefaultHandler is the default dot handler
type DefaultHandler struct {
	Processor
}

func setRecord(attr map[string]string) {
	attr["shape"] = "record"
}

func addAttr(attr map[string]string, key, value string) map[string]string {
	if attr == nil {
		attr = make(map[string]string)
	}
	attr[key] = value
	return attr
}

func escapeString(str string) string {
	escapeChar := []string{
		"{", "}",
		"<", ">",
	}
	for _, ch := range escapeChar {
		str = strings.ReplaceAll(str, ch, "\\"+ch)
	}
	return str
}

func labelPrimitive(id xray.NodeID) string {
	return fmt.Sprintf("<%s>%s", id.Hash(), escapeString(id.String()))
}

func labelPrimitiveWithField(id xray.NodeID, field string) string {
	return fmt.Sprintf("<%s>%s: %s", id.Hash(), field, escapeString(id.String()))
}

// wrapAttr is the helper to wrap an attribute as a string
// this should be applied as the out-most wrapper for each attribute
func wrapAttr(attrStr string) string {
	return fmt.Sprintf("\"%s\"", attrStr)
}

// Process is the node handling logic for default handler
func (p *DefaultHandler) Process(g *GraphInfo, id xray.NodeID) {
	switch id.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16:
		fallthrough
	case reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8:
		fallthrough
	case reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		fallthrough
	case reflect.Chan, reflect.String, reflect.UnsafePointer:
		fallthrough
	case reflect.Float32, reflect.Float64, reflect.Complex64:
		fallthrough
	case reflect.Complex128, reflect.Interface, reflect.Func:
		fallthrough
	case reflect.Invalid:
		p.handlePrimitive(g, id)
	case reflect.Ptr:
		p.handlePtr(g, id)
	case reflect.Array, reflect.Slice:
		p.handleArray(g, id)
	case reflect.Map:
		p.handleMap(g, id)
	case reflect.Struct:
		p.handleStruct(g, id)
	default:
	}
	return
}

func (p *DefaultHandler) handlePrimitive(g *GraphInfo, id xray.NodeID) {
	attr := map[string]string{}
	setRecord(attr)
	attr[Label] = wrapAttr(labelPrimitive(id))
	p.setNodeRef(id, id.Hash())
	p.AddNode(id, nil, attr)
}

func labelPointer(id xray.NodeID) string {
	return id.Type()
}

func (p *DefaultHandler) handlePtr(g *GraphInfo, id xray.NodeID) {
	attr := map[string]string{}
	attr[Label] = wrapAttr(labelPointer(id))
	p.setNodeRef(id, id.Hash())
	p.AddNode(id, nil, attr)
	for childID := range g.Nodes[id].Children {
		p.AddEdge(id, childID, "", map[string]string{"style": "dashed"})
	}
}

func (p *DefaultHandler) handleArray(g *GraphInfo, id xray.NodeID) {
	p.setNodeRef(id, id.Hash())

	prims := []string{}
	for index, childID := range g.Nodes[id].Array {
		// merge primitive type objects
		p.setNodeRef(childID, fmt.Sprintf("%s:%d", id.Hash(), index))
		if !childID.IsPrimitive() {
			p.AddEdge(id, childID, strconv.Itoa(index), nil)
			prims = append(prims, fmt.Sprintf("<%d> %d", index, index))
			continue
		}
		prims = append(prims, labelPrimitive(childID))
		// remove prev nodes
		p.RemoveNode(childID)
	}
	attr := map[string]string{}
	setRecord(attr)
	addAttr(attr, "label", wrapAttr(strings.Join(prims, "|")))
	p.AddNode(id, nil, attr)
}

func (p *DefaultHandler) handleMap(g *GraphInfo, id xray.NodeID) {
	mapRef := fmt.Sprintf("cluster_%s", id.Hash())
	p.setNodeRef(id, mapRef)
	p.AddSubgraph(id, nil)
	// add node with same name to the graph
	p.AddNode(id, &id, map[string]string{"label": "map", "shape": "plaintext"})

	m := g.Maps[id]
	for k, v := range m {
		// add both k and v to the subgraph
		p.AddNode(k, &id, nil)
		p.AddNode(v, &id, nil)
		// add edge pointing k to v
		p.AddEdge(k, v, "", map[string]string{"style": "dashed", "color": "blue"})
		p.AddEdge(id, k, "", map[string]string{"style": "dashed", "color": "green"})
	}
}

func (p *DefaultHandler) handleStruct(g *GraphInfo, id xray.NodeID) {
	p.setNodeRef(id, id.Hash())

	fields := []string{}
	for fieldName, field := range g.Nodes[id].Fields {
		if !field.IsPrimitive() && field.Kind() != reflect.Ptr {
			p.AddEdge(id, field, fieldName, nil)
			fields = append(fields, fmt.Sprintf("<%s>%s", fieldName, fieldName))
			continue
		}
		// merge primitive type objects
		p.setNodeRef(field, fmt.Sprintf("%s:%s", id.Hash(), field.Hash()))
		if field.Kind() == reflect.Ptr {
			fields = append(fields, fmt.Sprintf("{%s|<%s>%s}", fieldName, field.Hash(), field.Type()))
		} else {
			fields = append(fields, labelPrimitiveWithField(field, fieldName))
		}
		// fields = append(fields, labelPrimitiveWithField(field, fieldName))
		// remove prev nodes
		p.RemoveNode(field)
	}
	attr := map[string]string{}
	setRecord(attr)
	addAttr(attr, "label", wrapAttr(strings.Join(fields, "|")))
	p.AddNode(id, nil, attr)
}

// Draw returns the Graph object of the given scan result
// Notice this operation is blocking
func Draw(gi *GraphInfo, nodeCh <-chan xray.NodeID, h Handler) (*gographviz.Graph, error) {
	if h == nil {
		h = &DefaultHandler{*NewProcessor()}
	}

	idList := []xray.NodeID{}
	// cache objects until the channel is closed
	// don't process node until done with scan to avoid concurrent RW
	for id, ok := <-nodeCh; ok; id, ok = <-nodeCh {
		idList = append(idList, id)
	}
	for _, id := range idList {
		h.Process(gi, id)
	}

	return h.Render()
}

package scan

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

// Scanner stores the global object scan results
type Scanner struct {
	// control objects
	ctx       context.Context
	lock      *sync.Mutex
	wg        *sync.WaitGroup
	parallism int
	nodeIDCh  chan NodeID

	// scanned results, all nodes that has been scanned
	nodes map[NodeID]*Node
	// maps stores the KV pairs for each map
	maps map[NodeID]map[NodeID]NodeID

	// typeIdGen is used to customize type name
	typeIdGen func(reflect.Type) string
	// filter is used to eliminate nodes with certain conditions in the
	// scan result
	// return false to reject nodes
	filters []func(*Node) bool
	// terminals is used to prevent further decomposition for nodes with
	// conditions. The note itself will still be kept in the scan result.
	// return true to prevent further decomposition
	terminals []func(*Node) bool
	// errHandler allows users to register customized error handler
	errHandler func(error)
}

// decomposeMeta contains the metadata for decomposing an object
// it stores the object's relation with its parent. e.g. it's a field of its
// parent object or it's an element of an array
// if object is independent of its parent, should pass an nil meta on decompose
type decomposeMeta struct {
	fieldName string
	index     int
}

// getTypeID returns the unique identifier for the type
func getTypeID(t reflect.Type) string {
	return fmt.Sprintf("%s/%s/%s", t.PkgPath(), t.Name(), t.String())
}

// NewScanner instantiates a default Scanner instance
func NewScanner(ctx context.Context) *Scanner {
	scanner := &Scanner{
		ctx:       ctx,
		lock:      &sync.Mutex{},
		wg:        &sync.WaitGroup{},
		nodeIDCh:  make(chan NodeID),
		nodes:     make(map[NodeID]*Node),
		maps:      make(map[NodeID]map[NodeID]NodeID),
		typeIdGen: getTypeID,
	}
	if ctx == nil {
		scanner.ctx = context.Background()
	}

	return scanner
}

// WithParallism configures the number of goroutines running in parallel
// effectivelly it's controlling the channel buffer size
// by default it's processed in serial
func (s *Scanner) WithParallism(p int) *Scanner {
	s.parallism = p
	s.nodeIDCh = make(chan NodeID, p)

	return s
}

// WithFilter appends a filter for Node filtering
// User should NOT modify the object, and it's user's concern to keep the
// objects untouched.
func (s *Scanner) WithFilter(filter func(*Node) bool) *Scanner {
	for _, f := range s.filters {
		if reflect.ValueOf(f) != reflect.ValueOf(filter) {
			continue
		}
		return s
	}
	s.filters = append(s.filters, filter)

	return s
}

// WithTerminal appends a terminal for Node termination checking
// User should NOT modify the object, and it's user's concern to keep the
// objects untouched.
func (s *Scanner) WithTerminal(terminal func(*Node) bool) *Scanner {
	for _, t := range s.terminals {
		if reflect.ValueOf(t) != reflect.ValueOf(terminal) {
			continue
		}
		return s
	}
	s.filters = append(s.terminals, terminal)

	return s
}

// acceptNode checks that the node passes all fileters
func (s *Scanner) acceptNode(node *Node) bool {
	for _, filter := range s.filters {
		if accept := filter(node); !accept {
			return false
		}
	}
	return true
}

// isTerminal checks if terminates the node decomposition
func (s *Scanner) isTerminal(node *Node) bool {
	for _, terminal := range s.terminals {
		if yes := terminal(node); yes {
			return true
		}
	}
	return false
}

// registerNode adds the node to the scan result
// return nil if the node is successfully registered
// return non-nil *Node if the node has already been registered, and the pointer
// returned is the previously registered node associated with the ID
func (s *Scanner) registerNode(node *Node) *Node {
	s.lock.Lock()
	defer s.lock.Unlock()

	nid := node.NodeID(s.typeIdGen)
	if prevNodePtr, exist := s.nodes[nid]; exist {
		return prevNodePtr
	}

	s.nodes[nid] = node
	return nil
}

func (s *Scanner) getNodeID(node *Node) NodeID {
	return node.NodeID(s.typeIdGen)
}

// decompose breaks the given interface down
// decompose will block until the object and its underlying elements are all
// scanned
func (s *Scanner) decompose(ctx context.Context, parent *Node, obj reflect.Value, meta *decomposeMeta) {
	node := NewNode(obj)

	// ignore the filtered nodes
	if !s.acceptNode(node) {
		return
	}

	// register node to the global node set
	existingNodePtr := s.registerNode(node)
	if existingNodePtr != nil {
		node = existingNodePtr
	}

	// register node itself as its parents' node
	if parent != nil {
		defer parent.RegisterChild(node, s.typeIdGen)
	}
	// register node itself as its parent's field
	if parent != nil && meta != nil {
		defer parent.registerMeta(meta, s.getNodeID(node))
	}

	// skip decomposition as the node has already been decomposed
	if existingNodePtr != nil {
		return
	}

	// send the node that is decomposed completely
	defer func() {
		select {
		case s.nodeIDCh <- s.getNodeID(node):
		case <-ctx.Done():
			return
		}
	}()

	// check if user don't want to further break down the object
	if s.isTerminal(node) {
		return
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)

	switch node.Kind() {
	// reflexive primitives
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16:
		fallthrough
	case reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8:
		fallthrough
	case reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		fallthrough
	case reflect.Chan, reflect.String, reflect.UnsafePointer:
		fallthrough
	// primitives
	case reflect.Float32, reflect.Float64, reflect.Complex64:
		fallthrough
	case reflect.Complex128, reflect.Interface, reflect.Func:
		fallthrough
	case reflect.Invalid:
		// no further decomposition required, directly return
		return
	case reflect.Ptr:
		// skip further infer if object is nil
		if obj.IsNil() {
			wg.Done()
			break
		}
		go func() {
			defer wg.Done()
			s.decompose(ctx, node, node.InferPtr(), nil)
		}()
	case reflect.Array, reflect.Slice:
		s.handleArray(ctx, wg, node)
	case reflect.Map:
		s.handleMap(ctx, wg, node)
	case reflect.Struct:
		s.handleStruct(ctx, wg, node)
	default:
		// TODO: error handling
		wg.Done()
	}

	// return when the node is completely handled
	wg.Wait()

	return
}

func (s *Scanner) handleArray(ctx context.Context, wg *sync.WaitGroup, node *Node) {
	defer wg.Done()
	node.Array = make([]NodeID, node.value.Len())
	for i := 0; i < node.value.Len(); i++ {
		obj := node.value.Index(i)
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			s.decompose(ctx, node, obj, &decomposeMeta{index: index})
		}(i)
	}
}

func (s *Scanner) handleMap(ctx context.Context, wg *sync.WaitGroup, node *Node) {
	for _, keyVal := range node.value.MapKeys() {
		valVal := node.value.MapIndex(keyVal)
		ctlCh := make(chan int, 2)
		// notice: only mark key as map's children
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.decompose(ctx, node, keyVal, nil)
			// signal key complete
			ctlCh <- 1
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.decompose(ctx, nil, valVal, nil)
			// signal val complete
			ctlCh <- 2
		}()
		mapID := s.getNodeID(node)
		// map the key and val
		go func() {
			defer wg.Done()
			keyNode, valNode := NewNode(keyVal), NewNode(valVal)
			keyID, valID := s.getNodeID(keyNode), s.getNodeID(valNode)

			for {
				select {
				case <-ctlCh:
					s.lock.Lock()
					if prevKeyNode, exist := s.nodes[keyID]; !exist {
						s.lock.Unlock()
						continue
					} else {
						keyNode = prevKeyNode
					}
					if prevValNode, exist := s.nodes[valID]; !exist {
						s.lock.Unlock()
						continue
					} else {
						valNode = prevValNode
					}
					s.registerKVPair(mapID, keyID, valID)
					s.lock.Unlock()
					return
				case <-ctx.Done():
					return
				}
			}
		}()
	}
}

func (s *Scanner) handleStruct(ctx context.Context, wg *sync.WaitGroup, node *Node) {
	defer wg.Done()
	for i := 0; i < node.value.NumField(); i++ {
		field := node.value.Field(i)
		fieldName := node.value.Type().Field(i).Name
		wg.Add(1)
		go func(fieldName string) {
			defer wg.Done()
			s.decompose(ctx, node, field, &decomposeMeta{fieldName: fieldName})
		}(fieldName)
	}
}

// registerKVPair assumps the environment is locked
func (s *Scanner) registerKVPair(mid, kid, vid NodeID) {
	if _, exist := s.maps[mid]; !exist {
		s.maps[mid] = make(map[NodeID]NodeID)
	}
	s.maps[mid][kid] = vid
	return
}

// Scan starts the object decomposition process. It's a non-blocking operation.
// It will return a channel of node IDs and consumer can listen on the channel.
// When all nodes are sent over the channel, it will be closed.
func (s *Scanner) Scan(objs ...interface{}) <-chan NodeID {
	wg := &sync.WaitGroup{}
	wg.Add(len(objs))
	for _, obj := range objs {
		go func(obj interface{}) {
			defer wg.Done()
			s.decompose(s.ctx, nil, reflect.ValueOf(obj), nil)
		}(obj)
	}
	go func() {
		defer close(s.nodeIDCh)
		// when all routines finished, close the channel
		wg.Wait()
	}()
	return s.nodeIDCh
}

// Nodes returns the scanned nodes map maintained by the scanner
func (s *Scanner) Nodes() map[NodeID]*Node {
	return s.nodes
}

// Maps returns the scanned KV pairs stored for maps maintained by the scanner
func (s *Scanner) Maps() map[NodeID]map[NodeID]NodeID {
	return s.maps
}

// Node returns the Node object based on ID
func (s *Scanner) Node(id NodeID) *Node {
	return s.nodes[id]
}

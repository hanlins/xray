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
	nodeCh    chan *Node

	// scanned results, all nodes that has been scanned
	nodes map[nodeID]*Node
	// maps stores the KV pairs for each map
	maps map[nodeID]map[nodeID]nodeID

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
		nodeCh:    make(chan *Node),
		nodes:     make(map[nodeID]*Node),
		maps:      make(map[nodeID]map[nodeID]nodeID),
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
	s.nodeCh = make(chan *Node, p)

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

	nid := node.nodeID(s.typeIdGen)
	if prevNodePtr, exist := s.nodes[nid]; exist {
		return prevNodePtr
	}

	s.nodes[nid] = node
	return nil
}

func (s *Scanner) getNodeID(node *Node) nodeID {
	return node.nodeID(s.typeIdGen)
}

// decompose breaks the given interface down
// decompose will block until the object and its underlying elements are all
// scanned
func (s *Scanner) decompose(ctx context.Context, parent *Node, obj reflect.Value) {
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

	// skip decomposition as the node has already been decomposed
	if existingNodePtr != nil {
		return
	}

	// send the node that is decomposed completely
	defer func() {
		s.nodeCh <- node
	}()

	// check if user don't want to further break down the object
	if s.isTerminal(node) {
		return
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)

	switch node.value.Kind() {
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
			s.decompose(ctx, node, node.InferPtr())
			wg.Done()
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
	for i := 0; i < node.value.Len(); i++ {
		obj := node.value.Index(i)
		wg.Add(1)
		go func() {
			s.decompose(ctx, node, obj)
			wg.Done()
		}()
	}
	wg.Done()
}

func (s *Scanner) handleMap(ctx context.Context, wg *sync.WaitGroup, node *Node) {
	for _, keyVal := range node.value.MapKeys() {
		valVal := node.value.MapIndex(keyVal)
		ctlCh := make(chan int, 2)
		// notice: only mark key as map's children
		wg.Add(1)
		go func() {
			s.decompose(ctx, node, keyVal)
			wg.Done()
			// signal key complete
			ctlCh <- 1
		}()
		wg.Add(1)
		go func() {
			s.decompose(ctx, nil, valVal)
			wg.Done()
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
	for i := 0; i < node.value.NumField(); i++ {
		field := node.value.Field(i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.decompose(ctx, node, field)
		}()
	}
	wg.Done()
}

// registerKVPair assumps the environment is locked
func (s *Scanner) registerKVPair(mid, kid, vid nodeID) {
	if _, exist := s.maps[mid]; !exist {
		s.maps[mid] = make(map[nodeID]nodeID)
	}
	s.maps[mid][kid] = vid
	return
}

// Scan starts the object decomposition process. It's a non-blocking operation.
// It will return a channel of nodes and consumer can listen on the channel.
// When all nodes are sent over the channel, it will be closed.
func (s *Scanner) Scan(objs ...interface{}) <-chan *Node {
	wg := &sync.WaitGroup{}
	for _, obj := range objs {
		go func() {
			wg.Add(1)
			s.decompose(s.ctx, nil, reflect.ValueOf(obj))
			wg.Done()
		}()
	}
	go func() {
		// when all routines finished, close the channel
		wg.Wait()
		close(s.nodeCh)
	}()
	return s.nodeCh
}

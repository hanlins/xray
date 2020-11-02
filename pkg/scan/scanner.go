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
func (s *Scanner) decompose(ctx context.Context, parent *Node, obj interface{}) {
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
	defer func() { s.nodeCh <- node }()

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
		obj := node.value.Index(i).Interface()
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
		wg.Add(1)
		go func() {
			s.decompose(ctx, node, keyVal.Interface())
			// signal key complete
			ctlCh <- 1
			wg.Done()
		}()
		wg.Add(1)
		go func() {
			s.decompose(ctx, node, valVal.Interface())
			// signal val complete
			ctlCh <- 2
			wg.Done()
		}()
		// map the key and val
		go func() {
			keyNode, valNode := NewNode(keyVal.Interface()), NewNode(valVal.Interface())
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
					keyNode.AddMap(valNode)
					s.lock.Unlock()
					break
				case <-ctx.Done():
					break
				}
			}
			wg.Done()
		}()
	}
}

func (s *Scanner) handleStruct(ctx context.Context, wg *sync.WaitGroup, node *Node) {
	for i := 0; i < node.value.NumField(); i++ {
		field := node.value.Field(i)
		wg.Add(1)
		go func() {
			s.decompose(ctx, node, field.Interface())
			wg.Done()
		}()
	}
	wg.Done()
}

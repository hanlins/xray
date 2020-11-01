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
	objCh     chan interface{}

	// scanned results
	nodes map[string]*Node

	// customizations
	typeIdGen func(reflect.Type) string
	filters   []func(*Node) bool
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
		objCh:     make(chan interface{}),
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
	s.objCh = make(chan interface{}, p)

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

// acceptNode checks that the node passes all fileters
func (s *Scanner) acceptNode(node *Node) bool {
	for _, filter := range s.filters {
		if accept := filter(node); !accept {
			return false
		}
	}
	return true
}

package symbol

// Source: src/util/symbol.cpp:70-83; internal_symbol_table members:33-35.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
)

var ErrServiceOwner = errors.New("g3: invalid symbol service owner")
var ErrServiceUninitialized = errors.New("g3: source symbol table is null")

// Service holds the source symbol-table pointer in an explicit single-lane
// owner. Initialization publishes a completed native Go manager; finalization
// closes it before clearing the pointer. The service header is charged once.
// Process singleton and global Runtime accounting remain separate bindings.
type Service struct {
	self       *Service
	config     Config
	manager    *Manager
	trace      *diagnostic.TraceTags
	process    *memory.ProcessAccounting
	ownerBytes uint64
	closed     bool
}

func NewService(c Config) (*Service, error) {
	n := uint64(reflect.TypeOf(Service{}).Size())
	if e := c.Budget.Reserve(n); e != nil {
		return nil, e
	}
	s := &Service{config: c, ownerBytes: n}
	s.self = s
	return s, nil
}
func (s *Service) open() error {
	if s == nil || s.self != s {
		return ErrServiceOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	return nil
}
func (s *Service) InitializeSource() error {
	if e := s.open(); e != nil {
		return e
	}
	if s.manager != nil {
		return nil
	}
	c := s.config
	c.onAllocated = s.traceAllocated
	m, e := NewManager(c)
	if e != nil {
		return e
	}
	s.manager = m
	return nil
}
func (s *Service) FinalizeSource() error {
	if e := s.open(); e != nil {
		return e
	}
	if s.manager != nil {
		if e := s.manager.Close(); e != nil {
			return e
		}
		s.traceDeallocated(s.manager)
		s.manager = nil
	}
	return nil
}

// Manager borrows the currently published table. Finalization invalidates all
// symbols of that owner; a later initialization acquires a fresh owner identity.
func (s *Service) Manager() (*Manager, error) {
	if e := s.open(); e != nil {
		return nil, e
	}
	if s.manager == nil {
		return nil, ErrServiceUninitialized
	}
	return s.manager, nil
}
func (s *Service) Close() error {
	if s == nil || s.self != s {
		return ErrServiceOwner
	}
	if s.closed {
		return nil
	}
	if e := s.FinalizeSource(); e != nil {
		return e
	}
	s.config.Budget.Release(s.ownerBytes)
	s.ownerBytes = 0
	s.closed = true
	return nil
}

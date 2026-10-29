package params

// Source: src/util/gparams.cpp:633-644; source imp:81-105.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

var ErrGlobalParamsUninitialized = errors.New("g3: source global parameter pointer is null")

// An explicit single-lane source global pointer. The actual catalog, Symbol
// and Params owners are borrowed and outlive every named current/old/failed
// node. Process singleton, TRACE stream, native ABI and global Runtime table
// accounting remain separate bindings. The header and all nodes are charged.
type GlobalParamsService struct {
	self                        *GlobalParamsService
	namespaceAssertions         *udiag.DebugContext
	namespaceTrace              *udiag.TraceTags
	memoryProcess               *umemory.ProcessAccounting
	Symbols                     *usymbol.Manager
	Context                     *ParamsContext
	Config                      GlobalParamsConfig
	Current, Superseded, Failed *GlobalParams
	OwnerBytes                  uint64
	closed                      bool
}

func NewGlobalParamsService(s *usymbol.Manager, p *ParamsContext, c GlobalParamsConfig) (*GlobalParamsService, error) {
	if s == nil || p == nil || p.SymbolsField != s {
		return nil, ErrGlobalParamsOwner
	}
	if e := p.open(); e != nil {
		return nil, e
	}
	if c.Register != nil {
		return nil, arena.ErrConfig
	} // source uses the concrete catalog
	if c.Catalog == nil {
		return nil, ErrGlobalParamsRegistrationMissing
	}
	c.Catalog.mu.Lock()
	e := c.Catalog.open()
	same := c.Catalog.symbols == s
	c.Catalog.mu.Unlock()
	if e != nil {
		return nil, e
	}
	if !same {
		return nil, ErrGlobalParamsOwner
	}
	n := uint64(reflect.TypeOf(GlobalParamsService{}).Size())
	if e := c.Budget.Reserve(n); e != nil {
		return nil, e
	}
	result := &GlobalParamsService{Symbols: s, Context: p, Config: c, OwnerBytes: n}
	result.self = result
	return result, nil
}
func (s *GlobalParamsService) open() error {
	if s == nil || s.self != s {
		return ErrGlobalParamsOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	return nil
}
func (s *GlobalParamsService) InitializeSource() error {
	if e := s.open(); e != nil {
		return e
	}
	s.traceNamespace("init", 634, "gparams::init()\n")
	// Source init is unconditional; a previous live pointer does not skip alloc.
	c := s.Config
	c.onAllocated = s.traceMemoryAllocated
	c.memoryTrace = s.memoryTraceTags()
	c.memoryProcess = s.memoryProcess
	g, e := newSourceGlobalParams(s.Symbols, s.Context, c)
	if e != nil {
		if g != nil {
			g.RetainedNext = s.Failed
			s.Failed = g
		}
		return e
	}
	old := s.Current
	s.Current = g // actual source g_imp=alloc(imp) publication
	if old != nil {
		old.RetainedNext = s.Superseded
		s.Superseded = old
	}
	return nil
}
func (s *GlobalParamsService) Manager() (*GlobalParams, error) {
	if e := s.open(); e != nil {
		return nil, e
	}
	if s.Current == nil {
		return nil, ErrGlobalParamsUninitialized
	}
	return s.Current, nil
}
func (s *GlobalParamsService) FinalizeSource() error {
	if e := s.open(); e != nil {
		return e
	}
	s.traceNamespace("finalize", 639, "gparams::finalize()\n")
	if s.Current != nil {
		current := s.Current
		if e := current.SourceFinalize(); e != nil {
			return e
		}
		s.traceMemoryDeallocated(current)
		s.Current = nil // only after completed actual source dealloc
	}
	return nil
}
func (s *GlobalParamsService) Close() error {
	if s == nil || s.self != s {
		return ErrGlobalParamsOwner
	}
	if s.closed {
		return nil
	}
	// Owning close can acknowledge only the actual stopped child owned here.
	if s.Current != nil {
		if e := s.Current.SourceFinalize(); e != nil {
			if cleanup := s.Current.CleanupStopped(); cleanup != nil {
				return errors.Join(e, cleanup)
			}
		}
		s.Current = nil
	}
	for s.Superseded != nil {
		node := s.Superseded
		if e := node.SourceFinalize(); e != nil {
			if cleanup := node.CleanupStopped(); cleanup != nil {
				return errors.Join(e, cleanup)
			}
		}
		s.Superseded = node.RetainedNext
		node.RetainedNext = nil
	}
	for s.Failed != nil {
		node := s.Failed
		if e := node.abortConstructor(); e != nil {
			return e
		}
		s.Failed = node.RetainedNext
		node.RetainedNext = nil
	}
	s.Config.Budget.Release(s.OwnerBytes)
	s.OwnerBytes = 0
	s.closed = true
	return nil
}

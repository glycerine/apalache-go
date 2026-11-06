package numeral

// Source: src/util/rational.cpp:64-83; src/util/rational.h:67;
// src/util/inf_rational.cpp:185-198; src/util/inf_int_rational.cpp:46-59.

import (
	"errors"
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

var ErrRationalGlobalOwner = errors.New("g3: rational global binding owner mismatch")

// A node owns only actual returned constructor roots. Its reflected cold
// storage is Go continuation metadata, not an invented source malloc event.
type rationalGlobalAcquisition struct {
	next                *rationalGlobalAcquisition
	owner               *RationalGlobalService
	manager             *RationalManager
	failed              *IntegerManager
	cause, cleanupCause error
	invocation          uint64
	bytes               uint64
	acquired            uint8
}
type RationalGlobalService struct {
	self                                            *RationalGlobalService
	service                                         *RationalService
	current                                         *RationalManager // actual borrowed source pointer; no second owner
	config                                          RationalServiceConfig
	memoryTrace                                     *diagnostic.TraceTags
	acquisitions                                    *rationalGlobalAcquisition
	invocation, header                              uint64
	sourceCause                                     error
	finalizationStarted, finalized, closing, closed bool
}

func NewRationalGlobalService(c RationalServiceConfig) (*RationalGlobalService, error) {
	b := c.Integers.Cells.Budget
	if c.Integers.Memory == nil || c.Integers.Memory.Budget() != b || c.Integers.Words.Descriptors.Budget != b {
		return nil, arena.ErrConfig
	}
	n := uint64(reflect.TypeOf(RationalGlobalService{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	s, e := PrepareRationalService(c)
	if e != nil {
		b.Release(n)
		return nil, e
	}
	g := &RationalGlobalService{service: s, config: c, header: n}
	g.self = g
	if e = s.BindSourceGlobalSlot(&g.current); e != nil {
		cleanup := s.CloseChecked()
		b.Release(n)
		return nil, errors.Join(e, cleanup)
	}
	return g, nil
}
func (g *RationalGlobalService) open() error {
	if g == nil || g.self != g {
		return ErrRationalGlobalOwner
	}
	if g.closed || g.closing {
		return arena.ErrClosed
	}
	return g.service.ValidateSourceGlobalSlot(&g.current)
}
func (g *RationalGlobalService) InitializeSource() error {
	if e := g.open(); e != nil {
		return e
	}
	if g.current != nil {
		return nil
	} // source pointer-present shortcut before ended/static stops
	// The original static objects are never reconstructed. Prepare concrete
	// owning metadata before the real new manager constructor; publication still
	// follows actual constructor success and precedes reached constant access.
	if g.invocation == math.MaxUint64 {
		return arena.ErrBounds
	}
	b := g.config.Integers.Cells.Budget
	bytes := uint64(reflect.TypeOf(rationalGlobalAcquisition{}).Size())
	if e := b.Reserve(bytes); e != nil {
		return e
	}
	g.invocation++
	node := &rationalGlobalAcquisition{owner: g, invocation: g.invocation, bytes: bytes, next: g.acquisitions}
	g.acquisitions = node
	m, e := NewRationalManager(RationalConfig{Integers: g.config.Integers, Synchronized: true, onAllocated: g.traceManagerAllocated})
	if e != nil {
		node.cause = e
		var failed *IntegerManager
		if errors.As(e, &failed) {
			node.failed = failed
			node.acquired = 2
		}
		return e
	}
	node.manager = m
	node.acquired = 1 // actual constructor returned owning identity
	if !g.finalizationStarted {
		if e = g.service.PublishAcquiredSourceManager(&g.current, m); e != nil {
			node.cause = e
			return e
		}
		// Actual publication transferred the completed manager to the original
		// static context. This node retains history, never duplicate ownership.
		node.manager = nil
		node.acquired = 0
	} else {
		g.current = m // actual pointer assignment before ended static access
	}
	e = g.service.InitializeReboundSourceConstants(&g.current, m)
	node.cause = e
	return e
}
func (g *RationalGlobalService) Manager() (*RationalManager, error) {
	if e := g.open(); e != nil {
		return nil, e
	}
	if g.current == nil {
		return nil, ErrRationalServiceNoManager
	}
	return g.current, nil
}

// Service borrows the source static rational context used by the generated
// process lifecycle and its parameter-context consumer.
func (g *RationalGlobalService) Service() (*RationalService, error) {
	if e := g.open(); e != nil {
		return nil, e
	}
	return g.service, nil
}

// ProcessOwner exposes the borrowed memory cohort of the source rational
// global manager without transferring ownership of its runtime.
func (g *RationalGlobalService) ProcessOwner() (*umemory.ProcessAccounting, error) {
	if e := g.open(); e != nil {
		return nil, e
	}
	return g.config.Integers.Memory.ProcessOwner(), nil
}
func (g *RationalGlobalService) FinalizeSource() error {
	if e := g.open(); e != nil {
		return e
	}
	if g.sourceCause != nil {
		return g.sourceCause
	}
	if g.finalizationStarted {
		// A second call reaches the first already-ended Inf static destructor even
		// if the global manager pointer is null or was successfully republished.
		e := g.service.CheckSourceStaticLifetime(&g.current, SourceStaticInfZero)
		if e != nil {
			g.sourceCause = e
			return e
		}
	}
	g.finalizationStarted = true
	manager := g.current
	e := g.service.SourceFinalize()
	if e != nil {
		g.sourceCause = e
		return e
	}
	if manager != nil {
		g.traceManagerDeallocated(manager)
	}
	g.finalized = true
	return nil
}
func (g *RationalGlobalService) Close() error {
	if g == nil || g.self != g {
		return ErrRationalGlobalOwner
	}
	if g.closed {
		return nil
	}
	g.closing = true
	b := g.config.Integers.Cells.Budget
	for g.acquisitions != nil {
		node := g.acquisitions
		if node.owner != g || node.invocation == 0 {
			return ErrRationalGlobalOwner
		}
		var e error
		switch node.acquired {
		case 1:
			e = node.manager.CloseChecked()
		case 2:
			e = node.failed.CloseChecked()
		case 0:
		default:
			return ErrRationalGlobalOwner
		}
		if e != nil {
			if node.cleanupCause == nil {
				node.cleanupCause = e
			}
			return e
		}
		// Only actual completed owning release permits unlink/refund. The source
		// pointer remains a diagnostic scalar until the whole binding closes.
		g.acquisitions = node.next
		node.next = nil
		node.manager = nil
		node.failed = nil
		b.Release(node.bytes)
		node.bytes = 0
	}
	if e := g.service.CloseChecked(); e != nil {
		return e
	}
	if e := g.service.DetachSourceGlobalSlot(&g.current); e != nil {
		return e
	}
	b.Release(g.header)
	g.header = 0
	g.closed = true
	return nil
}

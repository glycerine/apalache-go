package sat

// Source: src/sat/sat_solver.cpp; src/sat/sat_solver.h:268-282; src/sat/sat_clause.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/messages"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

var ErrSATMissingSourceExtBinding = errors.New("g3: missing source SAT extension binding")

// Promoted exception methods preserve the source base behavior; Unwrap
// supplies the existing Go errors.As adapter for a base-class catch.
type SATSolverException struct{ z3exception.DefaultException }

func (e SATSolverException) Unwrap() error { return e.DefaultException }

var ErrSATSolverCanceled = SATSolverException{z3exception.NewDefaultException("canceled")}
var ErrSATSolverMaxMemory = SATSolverException{z3exception.NewDefaultException("max. memory exceeded")}
var ErrSATSignedCounterDomain = errors.New("g3: undefined source signed SAT counter decrement")

// This actual member-state grouping does not construct the full solver or child
// policy engines. All bound services are borrowed and must outlive it. Counters
// are the source members touched by dec(), not stand-in Run implementations.
type satSolverPropagationState struct {
	self                                                                        *satSolverPropagationState
	variables                                                                   *satSolverVariableState
	clauses                                                                     *ClauseAllocator
	converter                                                                   *satModelConverter
	limit                                                                       *control.Reslimit
	memory                                                                      *umemory.Runtime
	config                                                                      *satMainConfig
	traceTags                                                                   *diagnostic.TraceTags
	budget                                                                      *arena.Budget
	diffLevels                                                                  satVariableCharColumn
	conflict                                                                    satJustification
	notLiteral                                                                  Literal
	cleanerCounter, asymmCounter, probingCounter                                int32
	numCheckpoints, binaryPropagations, ternaryPropagations, clausePropagations uint32
	ownerBytes                                                                  uint64
	inconsistent, checkpointEnabled, extensionPresent, closed                   bool
}

func newSATSolverPropagationState(v *satSolverVariableState, a *ClauseAllocator, mc *satModelConverter, r *control.Reslimit, mem *umemory.Runtime, cfg *satMainConfig, b *arena.Budget) (*satSolverPropagationState, error) {
	if e := v.constructionBorrow(); e != nil {
		return nil, e
	}
	if e := a.open(); e != nil {
		return nil, e
	}
	if e := mc.constructionBorrow(); e != nil {
		return nil, e
	}
	if e := cfg.open(); e != nil {
		return nil, e
	}
	if r == nil || r.Closed() || mem == nil || v.memory != mem {
		return nil, ErrSATOwner
	}
	if _, e := mem.AllocationSize(); e != nil {
		return nil, e
	}
	if e := v.justification.bindClauses(a); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(satSolverPropagationState{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	s := &satSolverPropagationState{variables: v, clauses: a, converter: mc, limit: r, memory: mem, config: cfg, budget: b, ownerBytes: n, checkpointEnabled: true, notLiteral: DefaultLiteral()}
	s.self = s
	return s, nil
}
func (s *satSolverPropagationState) constructionBorrow() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	return s.variables.constructionBorrow()
}
func (s *satSolverPropagationState) open() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	return s.variables.open()
}
func (s *satSolverPropagationState) Close() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closed {
		return nil
	}
	s.diffLevels.finalize()
	s.budget.Release(s.ownerBytes)
	s.ownerBytes = 0
	s.closed = true
	return nil
}
func (s *satSolverPropagationState) checkpoint() error {
	if e := s.open(); e != nil {
		return e
	}
	if !s.checkpointEnabled {
		return nil
	}
	if s.limit.Closed() {
		return arena.ErrClosed
	}
	if !s.limit.Inc() {
		if e := s.converter.Reset(); e != nil {
			return e
		}
		s.variables.modelIsCurrent = false
		s.variables.modelKnown = true
		return SATSolverException{z3exception.NewDefaultException(messages.Canceled)}
	}
	s.numCheckpoints++
	if s.numCheckpoints < 10 {
		return nil
	}
	s.numCheckpoints = 0
	cfg, e := s.config.State()
	if e != nil {
		return e
	}
	n, e := s.memory.AllocationSize()
	if e != nil {
		return e
	}
	if n > cfg.MaxMemory {
		return SATSolverException{z3exception.NewDefaultException(messages.MaxMemory)}
	}
	return nil
}
func (s *satSolverPropagationState) numDiffLevels(c StoredClauseRef) (uint32, error) {
	if e := s.open(); e != nil {
		return 0, e
	}
	n, e := s.clauses.Size(c)
	if e != nil {
		return 0, e
	}
	return s.numDiffLevelsCore(n, nil, c, true)
}

// Source num_diff_levels accepts a pointer and an independent unsigned count.
// The caller's concrete literal storage is borrowed; accesses are checked at
// each source dereference, after reserve and any preceding marks.
func (s *satSolverPropagationState) numDiffLevelsLiterals(n uint32, lits []Literal) (uint32, error) {
	if e := s.open(); e != nil {
		return 0, e
	}
	return s.numDiffLevelsCore(n, lits, StoredClauseRef{}, false)
}

func (s *satSolverPropagationState) diffLevelLiteral(i uint32, lits []Literal, c StoredClauseRef, clause bool) (Literal, error) {
	if clause {
		return s.clauses.Literal(c, i)
	}
	if uint64(i) >= uint64(cap(lits)) {
		return 0, arena.ErrBounds
	}
	return lits[:cap(lits)][i], nil
}

// Both concrete storage routes execute the same source reserve/mark/reset body.
func (s *satSolverPropagationState) numDiffLevelsCore(n uint32, lits []Literal, c StoredClauseRef, clause bool) (uint32, error) {
	return s.numDiffLevelsSpanCore(n, satSolverLiteralSpan{literals: lits}, c, clause)
}
func (s *satSolverPropagationState) numDiffLevelsSpan(n uint32, span satSolverLiteralSpan) (uint32, error) {
	if e := s.open(); e != nil {
		return 0, e
	}
	return s.numDiffLevelsSpanCore(n, span, StoredClauseRef{}, false)
}
func (s *satSolverPropagationState) numDiffLevelsSpanCore(n uint32, span satSolverLiteralSpan, c StoredClauseRef, clause bool) (uint32, error) {
	need := s.variables.scopeLevel + 1
	// The reached byte marks use native slice growth; newly exposed slots are zero.
	s.diffLevels.extend(need)
	var count uint32
	for i := uint32(0); i < n; i++ {
		var l Literal
		var e error
		if clause {
			l, e = s.clauses.Literal(c, i)
		} else {
			l, e = span.literal(i)
		}
		if e != nil {
			return count, e
		}
		if satStorageSourceDebug {
			v, e := s.variables.valueLiteral(l)
			if e != nil {
				return count, e
			}
			if v == lbool.Undef {
				return count, ErrSATAssertion
			}
		}
		level, e := s.variables.levelVar(l.variable())
		if e != nil {
			return count, e
		}
		m, e := s.diffLevels.read(level)
		if e != nil {
			return count, e
		}
		if m == 0 {
			if e = s.diffLevels.write(level, 1); e != nil {
				return count, e
			}
			count++
		}
	}
	for i := uint32(0); i < n; i++ {
		var l Literal
		var e error
		if clause {
			l, e = s.clauses.Literal(c, i)
		} else {
			l, e = span.literal(i)
		}
		if e != nil {
			return count, e
		}
		level, e := s.variables.levelVar(l.variable())
		if e != nil {
			return count, e
		}
		if e = s.diffLevels.write(level, 0); e != nil {
			return count, e
		}
	}
	return count, nil
}

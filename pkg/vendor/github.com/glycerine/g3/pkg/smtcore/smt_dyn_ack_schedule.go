package smtcore

// Source: src/smt/dyn_ack.cpp:118-138,315-340; src/smt/dyn_ack.h:49-120;
// src/smt/params/dyn_ack_params.h:23-29.

import (
	"math"

	"github.com/glycerine/g3/pkg/ast"
)

// dynAckCandidate preserves the source pair-before-triple propagation order.
// The enclosing SMT context still constructs each selected clause.
type dynAckCandidate struct {
	pair   dynAckPair
	triple dynAckTriple
	isPair bool
}

type dynAckStrategy uint8

const (
	dynAckDisabled dynAckStrategy = iota
	dynAckRoot
	dynAckConflictResolution
)

type dynAckSchedule struct {
	pairs        *dynAckPairState
	triples      *dynAckTripleState
	gcEvery      uint32
	factor       float64
	propagations uint32
	instances    uint32
}

func newDynAckSchedule(pairs *dynAckPairState, triples *dynAckTripleState, gcEvery uint32, factor float64) (*dynAckSchedule, error) {
	if pairs == nil || triples == nil || pairs.manager != triples.manager || math.IsNaN(factor) || math.IsInf(factor, 0) || factor < 0 {
		return nil, ast.ErrASTDomain
	}
	if e := pairs.check(); e != nil {
		return nil, e
	}
	if e := triples.check(); e != nil {
		return nil, e
	}
	return &dynAckSchedule{pairs: pairs, triples: triples, gcEvery: gcEvery, factor: factor}, nil
}

func validDynAckStrategy(strategy dynAckStrategy) bool {
	return strategy <= dynAckConflictResolution
}

// usedCongruence follows used_cg_eh; its strategy guard precedes cg_eh's
// application preconditions and occurrence accounting.
func (s *dynAckSchedule) usedCongruence(n1, n2 ast.ASTHandle, strategy dynAckStrategy) error {
	if s == nil {
		return ast.ErrASTHandle
	}
	if !validDynAckStrategy(strategy) {
		return ast.ErrASTDomain
	}
	if e := s.pairs.check(); e != nil {
		return e
	}
	if strategy == dynAckConflictResolution {
		return s.pairs.congruenceEvent(n1, n2)
	}
	return nil
}

// conflictCongruence follows cg_conflict_eh.
func (s *dynAckSchedule) conflictCongruence(n1, n2 ast.ASTHandle, strategy dynAckStrategy) error {
	if s == nil {
		return ast.ErrASTHandle
	}
	if !validDynAckStrategy(strategy) {
		return ast.ErrASTDomain
	}
	if e := s.pairs.check(); e != nil {
		return e
	}
	if strategy == dynAckRoot {
		return s.pairs.congruenceEvent(n1, n2)
	}
	return nil
}

// usedEquality follows used_eq_eh. Its gate is independent of the pair
// strategy, including when pair-mode propagation is disabled.
func (s *dynAckSchedule) usedEquality(n1, n2, representative ast.ASTHandle, enabled bool) error {
	if s == nil {
		return ast.ErrASTHandle
	}
	if e := s.triples.check(); e != nil {
		return e
	}
	if enabled {
		return s.triples.eqEvent(n1, n2, representative)
	}
	return nil
}

// propagate selects reached candidate records using the source cumulative
// conflict budget. The caller must construct clauses and invoke each state's
// markInstantiated transition for selected records in returned order.
func (s *dynAckSchedule) propagate(conflicts uint32, strategy dynAckStrategy) ([]dynAckCandidate, error) {
	if s == nil {
		return nil, ast.ErrASTHandle
	}
	if !validDynAckStrategy(strategy) {
		return nil, ast.ErrASTDomain
	}
	if e := s.pairs.check(); e != nil {
		return nil, e
	}
	if e := s.triples.check(); e != nil {
		return nil, e
	}
	if strategy == dynAckDisabled {
		return nil, nil
	}
	s.propagations++
	if s.propagations > s.gcEvery {
		if e := s.pairs.gc(); e != nil {
			return nil, e
		}
		s.propagations = 0
	}
	budget := float64(conflicts) * s.factor
	if budget > math.MaxUint32 {
		return nil, ast.ErrASTDomain
	}
	maxInstances := uint32(budget) // source static_cast<unsigned> truncates
	var selected []dynAckCandidate
	for s.instances < maxInstances && s.pairs.queueHead < len(s.pairs.toInstantiate) {
		key := s.pairs.toInstantiate[s.pairs.queueHead]
		s.pairs.queueHead++
		s.instances++
		selected = append(selected, dynAckCandidate{pair: key, isPair: true})
	}
	for s.instances < maxInstances && s.triples.queueHead < len(s.triples.toInstantiate) {
		key := s.triples.toInstantiate[s.triples.queueHead]
		s.triples.queueHead++
		s.instances++
		selected = append(selected, dynAckCandidate{triple: key})
	}
	return selected, nil
}

// initSearch keeps instantiated-clause records, as the source hook does.
func (s *dynAckSchedule) initSearch() error {
	if s == nil {
		return ast.ErrASTHandle
	}
	if e := s.pairs.initSearch(); e != nil {
		return e
	}
	if e := s.triples.initSearch(); e != nil {
		return e
	}
	s.instances = 0
	s.propagations = 0
	return nil
}

func (s *dynAckSchedule) reset() error {
	if e := s.initSearch(); e != nil {
		return e
	}
	clear(s.pairs.instantiated)
	clear(s.triples.instantiated)
	return nil
}

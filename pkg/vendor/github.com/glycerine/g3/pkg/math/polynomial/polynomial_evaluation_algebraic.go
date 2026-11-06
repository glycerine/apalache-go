package polynomial

// Source: src/math/polynomial/polynomial.cpp:6436-6522;
// src/math/polynomial/algebraic_numbers.cpp:2387-2388,2406.

import (
	"errors"
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type polynomialEvalAlgebraicFrame struct {
	aux         Algebraic
	manager     *AlgebraicRootCore
	constructed bool
}
type polynomialAlgebraicEvaluationScope struct {
	trace                       polynomialEvaluationTraceScope
	isolationCaller             *algebraicMultivariateIsolationFrame
	frames                      []*polynomialEvalAlgebraicFrame
	vm                          *AlgebraicRootCore
	depth                       uint32
	lexGeneration               uint64
	active, executing, borrowed bool
	bodyCause, sourceCause      error
}

func (m *MultivariatePolynomialManager) evaluationAlgebraicFrame() (*polynomialEvalAlgebraicFrame, error) {
	s := &m.algebraicEvaluation
	if int(s.depth) == len(s.frames) {
		if e := m.originalBudget.Reserve(uint64(reflect.TypeOf(polynomialEvalAlgebraicFrame{}).Size())); e != nil {
			return nil, e
		}
		s.frames = append(s.frames, &polynomialEvalAlgebraicFrame{})
	}
	f := s.frames[s.depth]
	if f.constructed {
		return nil, ErrPolynomialPending
	}
	*f = polynomialEvalAlgebraicFrame{manager: s.vm, constructed: true}
	s.depth++
	return f, nil
}
func (m *MultivariatePolynomialManager) finishEvaluationAlgebraicFrame(f *polynomialEvalAlgebraicFrame, owning bool) error {
	s := &m.algebraicEvaluation
	if !f.constructed {
		return nil
	}
	if !owning && s.sourceCause != nil {
		return s.sourceCause
	}
	if owning {
		l, e := f.manager.locationCleanup(&f.aux)
		if e != nil {
			return e
		}
		if l != nil {
			if e = f.manager.destroyCell(&f.aux, l, true); e != nil {
				return e
			}
		}
	} else if e := f.manager.Del(&f.aux); e != nil {
		return e
	}
	f.constructed = false
	f.manager = nil
	return nil
}
func (s *polynomialAlgebraicEvaluationScope) pending() bool {
	if s.trace.active {
		return true
	}
	for _, f := range s.frames {
		if f.constructed {
			return true
		}
	}
	return s.lexGeneration != 0 && s.vm != nil // released explicitly after a completed sort
}
func (s *polynomialAlgebraicEvaluationScope) releaseBorrow() {
	if s.borrowed {
		s.vm.polynomialEvaluationBorrowers--
		s.borrowed = false
	}
}

// EvaluateAlgebraic instantiates source t_eval for the actual algebraic manager.
// Values are borrowed; power and arithmetic may refine mapped root cells.
// On a stopped arithmetic call, its algebraic owner is cleaned explicitly
// before CleanupAlgebraicEvaluation releases retained lexical auxiliaries.
func (m *MultivariatePolynomialManager) EvaluateAlgebraic(p Polynomial, vm *AlgebraicRootCore, v PolynomialAlgebraicEvaluation, out *Algebraic) (cause error) {
	return m.evaluateAlgebraicForIsolation(p, vm, v, out, nil)
}
func (m *MultivariatePolynomialManager) evaluateAlgebraicForIsolation(p Polynomial, vm *AlgebraicRootCore, v PolynomialAlgebraicEvaluation, out *Algebraic, caller *algebraicMultivariateIsolationFrame) (cause error) {
	if e := m.ready(); e != nil {
		return e
	}
	if vm == nil || v == nil {
		return arena.ErrConfig
	}
	if e := vm.check(); e != nil {
		return e
	}
	if vm.config.Rationals.Integers() != m.core.integer {
		return arena.ErrConfig
	}
	zero, e := m.core.IsZero(p)
	if e != nil {
		return e
	}
	if zero {
		return vm.Del(out)
	}
	constant, e := m.core.IsConstant(p)
	if e != nil {
		return e
	}
	if constant {
		debugOwner, enabled, e := algebraicAssertionSwitch()
		if e != nil {
			return e
		}
		if enabled {
			size, e := m.core.readWord(p, 8)
			if e != nil {
				return e
			}
			if size != 1 {
				return diagnostic.DebugAssertionFailure(debugOwner, "src/math/polynomial/polynomial.cpp", 6516, "size(p)==1")
			}
		}
		n, e := m.core.mutableCoefficientCore(p, 0)
		if e != nil {
			return e
		}
		return vm.SetInteger(out, &n)
	}
	s := &m.algebraicEvaluation
	if s.active || s.executing || m.evaluation.active {
		return ErrPolynomialPending
	}
	if vm.polynomialEvaluationBorrowers == math.MaxUint32 {
		return arena.ErrSlotLimit
	}
	s.isolationCaller = caller
	if caller != nil {
		caller.evaluation = m
	}
	s.vm = vm
	s.active = true
	s.executing = true
	s.borrowed = true
	s.bodyCause = nil
	s.sourceCause = nil
	vm.polynomialEvaluationBorrowers++
	vm.polynomialEvaluationRunning++
	defer func() {
		s.executing = false
		vm.polynomialEvaluationRunning--
		if cause != nil && s.bodyCause == nil {
			s.bodyCause = cause
		}
		if !s.pending() {
			s.active = false
			s.releaseBorrow()
		}
	}()
	if e = m.lexSortForAlgebraic(p, s); e != nil {
		return e
	}
	s.lexGeneration = 0
	n, e := m.core.readWord(p, 8)
	if e != nil {
		return e
	}
	x, e := m.core.MaxVariable(p)
	if e != nil {
		return e
	}
	return m.evaluationAlgebraicCore(p, v, 0, n, x, out)
}

func (s *polynomialAlgebraicEvaluationScope) childSourceCause() error {
	vm := s.vm
	if vm == nil {
		return nil
	}
	if vm.selection.polynomialCaller == s && vm.selection.active && vm.selection.sourceStop != nil {
		return vm.selection.sourceStop
	}
	if vm.inversion.polynomialCaller == s && vm.inversion.remaining != 0 && vm.inversion.sourceStop != nil {
		return vm.inversion.sourceStop
	}
	if vm.division.polynomialCaller == s && vm.division.active && vm.division.sourceStop != nil {
		return vm.division.sourceStop
	}
	if vm.binary.polynomialCaller == s && vm.binary.active && vm.binary.sourceStop != nil {
		return vm.binary.sourceStop
	}
	if vm.resultant.polynomialCaller == s && vm.resultant.active && vm.resultant.sourceStop != nil {
		return vm.resultant.sourceStop
	}
	if vm.arithmetic.polynomialCaller == s && vm.arithmetic.active && vm.arithmetic.sourceStop != nil {
		return vm.arithmetic.sourceStop
	}
	if vm.basicRootScope.polynomialCaller == s && vm.basicRootScope.active && vm.basicRootScope.sourceStop != nil {
		return vm.basicRootScope.sourceStop
	}
	if vm.integerSet.polynomialCaller == s && vm.integerSet.active && vm.integerSet.sourceStop != nil {
		return vm.integerSet.sourceStop
	}
	if vm.integerAdd.polynomialCaller == s && vm.integerAdd.active && vm.integerAdd.sourceStop != nil {
		return vm.integerAdd.sourceStop
	}
	if vm.localPolynomialCaller == s && vm.localActive && vm.localCleanup != nil {
		return vm.localCleanup
	}
	return nil
}
func (m *MultivariatePolynomialManager) evaluationAlgebraicCall(op uint8, a, b *Algebraic, n *unumeral.Int, k uint32, out *Algebraic) (cause error) {
	s := &m.algebraicEvaluation
	vm := s.vm
	previous := vm.polynomialCaller
	vm.polynomialCaller = s
	defer func() {
		vm.polynomialCaller = previous
		if e := s.childSourceCause(); e != nil {
			s.sourceCause = e
			cause = errors.Join(cause, e)
		}
	}()
	switch op {
	case polynomialEvalSet:
		return vm.SetInteger(out, n)
	case polynomialEvalReset:
		return vm.Del(out)
	case polynomialEvalAddInteger:
		return vm.AddInteger(a, n, out)
	case polynomialEvalAdd:
		return vm.Add(a, b, out)
	case polynomialEvalPower:
		return vm.Power(a, k, out)
	case polynomialEvalMultiply:
		return vm.Mul(a, b, out)
	}
	return arena.ErrBounds
}

func (m *MultivariatePolynomialManager) CleanupAlgebraicEvaluation() error {
	if e := m.identity(); e != nil {
		return e
	}
	s := &m.algebraicEvaluation
	if s.executing || m.core.evaluationTraceCallback() {
		return ErrPolynomialPending
	}
	if e := m.cleanupEvaluationTrace(&s.trace); e != nil {
		return e
	}
	if !s.active {
		return nil
	}
	if s.childSourceCause() != nil {
		return ErrAlgebraicPending
	}
	if s.lexGeneration != 0 {
		if m.lexPending && m.lexGeneration == s.lexGeneration {
			if e := m.cleanupLexSort(); e != nil {
				return e
			}
		}
		s.lexGeneration = 0
	}
	for i := len(s.frames) - 1; i >= 0; i-- {
		if e := m.finishEvaluationAlgebraicFrame(s.frames[i], true); e != nil {
			return e
		}
	}
	s.active = false
	s.depth = 0
	s.releaseBorrow()
	return nil
}
func (m *MultivariatePolynomialManager) closeAlgebraicEvaluationFrames() error {
	s := &m.algebraicEvaluation
	if s.active || s.executing {
		return ErrPolynomialPending
	}
	m.originalBudget.Release(uint64(len(s.frames)) * uint64(reflect.TypeOf(polynomialEvalAlgebraicFrame{}).Size()))
	s.frames = nil
	s.vm = nil
	return nil
}

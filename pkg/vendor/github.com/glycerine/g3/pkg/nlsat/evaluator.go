package nlsat

// Source: src/nlsat/nlsat_evaluator.cpp/.h.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// ArithmeticAtom is the concrete inequality/root sum used by the evaluator.
// Polynomial and atom allocations remain owned by the solver.
type ArithmeticAtom interface{ arithmeticAtom() }

func (*InequalityAtom) arithmeticAtom() {}
func (*RootAtom) arithmeticAtom()       {}

// Evaluator borrows its assignment, polynomial manager and allocator. Its
// interval manager owns factory storage; returned infeasible sets carry one
// reference that the caller must retire before closing this evaluator.
type Evaluator struct {
	self                             *Evaluator
	assignment                       *Assignment
	search                           *searchState
	pm                               *polynomial.MultivariatePolynomialManager
	core                             *polynomial.MultivariatePolynomialCore
	am                               *polynomial.AlgebraicManager
	ism                              *IntervalSetManager
	roots, addRoots                  *polynomial.AlgebraicVector
	addSigns                         []int
	callRef                          *polynomial.PolynomialRef
	table                            *signTable
	locals                           [4]evaluatorIntervalRef
	budget                           *arena.Budget
	header                           uint64
	running, failed, closing, closed bool
}

func NewEvaluator(assignment *Assignment, pm *polynomial.MultivariatePolynomialManager, allocator *umemory.SmallAllocator) (*Evaluator, error) {
	if assignment == nil || pm == nil || allocator == nil {
		return nil, arena.ErrConfig
	}
	am, err := assignment.Manager()
	if err != nil {
		return nil, err
	}
	core, err := pm.ArithmeticCore()
	if err != nil {
		return nil, err
	}
	q, err := am.RationalManager()
	if err != nil {
		return nil, err
	}
	z, err := core.IntegerManager()
	if err != nil {
		return nil, err
	}
	if z != q.Integers() {
		return nil, arena.ErrConfig
	}
	n := uint64(reflect.TypeOf(Evaluator{}).Size())
	if err = q.Budget().Reserve(n); err != nil {
		return nil, err
	}
	e := &Evaluator{assignment: assignment, pm: pm, core: core, am: am, budget: q.Budget(), header: n}
	e.self = e
	e.ism, err = NewIntervalSetManager(am, allocator)
	if err != nil {
		return e, err
	}
	for i := range e.locals {
		e.locals[i].manager = e.ism
	}
	e.roots, err = am.NewVector()
	if err != nil {
		return e, err
	}
	e.addRoots, err = am.NewVector()
	if err != nil {
		return e, err
	}
	e.table, err = newSignTable(am)
	if err != nil {
		return e, err
	}
	return e, nil
}
func (e *Evaluator) owner() error {
	if e == nil || e.self != e {
		return polynomial.ErrAlgebraicOwner
	}
	if e.closed {
		return arena.ErrClosed
	}
	return nil
}
func (e *Evaluator) open() error {
	if err := e.owner(); err != nil {
		return err
	}
	if e.closing || e.running || e.failed || e.table == nil {
		return polynomial.ErrAlgebraicPending
	}
	if err := e.assignment.open(); err != nil {
		return err
	}
	_, err := e.pm.ArithmeticCore()
	return err
}
func (e *Evaluator) IntervalManager() (*IntervalSetManager, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	return e.ism, nil
}
func (e *Evaluator) assertAssigned(line int32, text string, x uint32) error {
	enabled, err := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
	if err != nil || !enabled {
		return err
	}
	assigned, err := e.assignment.Contains(x)
	if err != nil {
		return err
	}
	return atomAssert("nlsat_evaluator.cpp", line, text, assigned)
}
func satisfied(sign int, kind AtomKind, neg bool) bool {
	r := sign == 0 && (kind == AtomEQ || kind == AtomRootEQ || kind == AtomRootLE || kind == AtomRootGE) ||
		sign < 0 && (kind == AtomLT || kind == AtomRootLT || kind == AtomRootLE) ||
		sign > 0 && (kind == AtomGT || kind == AtomRootGT || kind == AtomRootGE)
	if neg {
		return !r
	}
	return r
}
func (e *Evaluator) evalSign(p polynomial.Polynomial) (value int, err error) {
	enabled, err := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
	if err != nil {
		return 0, err
	}
	if enabled {
		x, err := e.core.MaxVariable(p)
		if err != nil {
			return 0, err
		}
		if err = e.assertAssigned(386, "m_assignment.is_assigned(max_var(p))", x); err != nil {
			return 0, err
		}
	}
	if err = e.beginPolynomial(p); err != nil {
		return 0, err
	}
	defer e.finishPolynomial(&err)
	return e.am.EvalSign(e.pm, p, e.assignment)
}
func (e *Evaluator) evalInequality(a *InequalityAtom, neg bool) (bool, error) {
	if err := e.assertAssigned(403, "m_assignment.is_assigned(a->max_var())", a.maxVar); err != nil {
		return false, err
	}
	sign := 1
	for i := uint32(0); i < a.Size(); i++ {
		p, err := a.Polynomial(i)
		if err != nil {
			return false, err
		}
		curr, err := e.evalSign(p)
		if err != nil {
			return false, err
		}
		even, err := a.IsEven(i)
		if err != nil {
			return false, err
		}
		if even && curr < 0 {
			curr = 1
		}
		sign *= curr
		if sign == 0 {
			break
		}
	}
	return satisfied(sign, a.kind, neg), nil
}
func (e *Evaluator) evalRoot(a *RootAtom, neg bool) (bool, error) {
	if err := e.assertAssigned(420, "m_assignment.is_assigned(a->max_var())", a.maxVar); err != nil {
		return false, err
	}
	if err := e.roots.Reset(); err != nil {
		return false, err
	}
	if err := e.isolateRoots(a.polynomial, a.x, e.roots); err != nil {
		return false, err
	}
	if err := atomAssert("nlsat_evaluator.cpp", 440, "a->i() > 0", a.index > 0); err != nil {
		return false, err
	}
	if a.index > e.roots.Len() {
		return neg, nil
	}
	if a.index == 0 {
		return false, arena.ErrBounds
	}
	v, err := e.assignment.Value(a.x)
	if err != nil {
		return false, err
	}
	r, err := e.roots.Borrow(a.index - 1)
	if err != nil {
		return false, err
	}
	sign, err := e.am.Compare(v, r)
	if err != nil {
		return false, err
	}
	return satisfied(sign, a.kind, neg), nil
}
func (e *Evaluator) Eval(a ArithmeticAtom, neg bool) (value bool, err error) {
	if err = e.open(); err != nil {
		return false, err
	}
	e.running = true
	defer func() {
		e.running = false
		if err != nil {
			e.failed = true
		}
	}()
	switch a := a.(type) {
	case *InequalityAtom:
		if a == nil {
			return false, arena.ErrConfig
		}
		return e.evalInequality(a, neg)
	case *RootAtom:
		if a == nil {
			return false, arena.ErrConfig
		}
		return e.evalRoot(a, neg)
	default:
		return false, arena.ErrConfig
	}
}
func (e *Evaluator) Push() error                { return e.open() }
func (e *Evaluator) Pop(numScopes uint32) error { return e.open() }

// Cleanup retires interrupted local owners. The caller first acknowledges any
// stopped numerical callee through its actual algebraic manager. Borrowed
// assignment values are not reset, and unrelated numerical stops are not cleared.
func (e *Evaluator) Cleanup() error {
	if err := e.owner(); err != nil {
		return err
	}
	if e.running {
		return polynomial.ErrAlgebraicPending
	}
	if e.closing {
		return e.Close()
	}
	e.failed = true
	if e.callRef != nil {
		if err := e.callRef.Close(); err != nil {
			return err
		}
		e.callRef = nil
	}
	if e.ism != nil {
		if err := e.ism.Cleanup(); err != nil {
			return err
		}
	}
	for i := len(e.locals) - 1; i >= 0; i-- {
		if err := e.locals[i].reset(); err != nil {
			return err
		}
	}
	if e.roots != nil {
		if err := e.roots.CleanupRetained(); err != nil {
			return err
		}
		if err := e.roots.Reset(); err != nil {
			return err
		}
	}
	if e.addRoots != nil {
		if err := e.addRoots.CleanupRetained(); err != nil {
			return err
		}
		if err := e.addRoots.Reset(); err != nil {
			return err
		}
	}
	if e.table != nil {
		if err := e.table.reset(); err != nil {
			return err
		}
	}
	e.failed = false
	return nil
}
func (e *Evaluator) Close() error {
	if e != nil && e.self == e && e.closed {
		return nil
	}
	if e != nil && e.self == e && e.search != nil {
		return ErrSearchLive
	}
	if err := e.owner(); err != nil {
		return err
	}
	if !e.closing {
		if e.running || e.failed {
			return polynomial.ErrAlgebraicPending
		}
		if e.ism != nil && len(e.ism.live) != 0 {
			return ErrIntervalLive
		}
		e.closing = true
	}
	if e.table != nil {
		if err := e.table.close(); err != nil {
			return err
		}
		e.table = nil
	}
	e.addSigns = nil
	if e.addRoots != nil {
		if err := e.addRoots.CleanupRetained(); err != nil {
			return err
		}
		if err := e.addRoots.Close(); err != nil {
			return err
		}
		e.addRoots = nil
	}
	if e.roots != nil {
		if err := e.roots.CleanupRetained(); err != nil {
			return err
		}
		if err := e.roots.Close(); err != nil {
			return err
		}
		e.roots = nil
	}
	if e.ism != nil {
		if err := e.ism.Close(); err != nil {
			return err
		}
		e.ism = nil
	}
	e.budget.Release(e.header)
	e.header = 0
	e.closed = true
	return nil
}

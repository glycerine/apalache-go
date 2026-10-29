package nlsat

// Source: src/nlsat/nlsat_explain.cpp:190-234,268-303.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// The actual assumption-generation subobject used by normalization/projection.
// It borrows real atom/cache/evaluator owners, not a completed explain/Solver.
type explanationAssumptions struct {
	self                    *explanationAssumptions
	search                  *searchState
	factors                 *polynomial.PolynomialRefVector
	localFactor             *polynomial.PolynomialRef
	vanishing               explanationVanishingScope
	normalization           explanationNormalizationScope
	projectionTodo          *projectionWorklist
	pscTmp                  *polynomial.PolynomialRefVector
	psc                     explanationPSCScope
	resultant               explanationResultantScope
	discriminant            explanationDiscriminantScope
	cell                    explanationCellScope
	factorProjection        bool
	fullDimensional         bool
	projectionFactors       explanationProjectionFactorScope
	leading                 explanationLeadingScope
	simplifiedCore          scopedLiteralVector
	output                  scopedLiteralVector
	alreadyAdded            []bool
	marked                  []uint32
	zeroFactors             []polynomial.Polynomial
	isEven                  []bool
	resultActive            bool
	header                  uint64
	running, failed, closed bool
}

func newExplanationAssumptions(r *searchState) (*explanationAssumptions, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if r.explanationAssumptions != nil {
		return nil, arena.ErrConfig
	}
	h := uint64(reflect.TypeOf(explanationAssumptions{}).Size())
	if err := r.store.atoms.ineq.budget.Reserve(h); err != nil {
		return nil, err
	}
	n := &explanationAssumptions{search: r, header: h}
	n.self = n
	n.simplifiedCore.factory = r.store.atoms
	n.output.factory = r.store.atoms
	r.explanationAssumptions = n
	var err error
	n.factors, err = polynomial.NewPolynomialRefVector(r.store.atoms.ineq.core)
	n.failed = err != nil
	return n, err
}
func (n *explanationAssumptions) owner() error {
	if n == nil || n.self != n {
		return ErrSearchOwner
	}
	if n.closed {
		return arena.ErrClosed
	}
	if err := n.search.owner(); err != nil {
		return err
	}
	if n.search.explanationAssumptions != n {
		return ErrSearchOwner
	}
	return nil
}
func (n *explanationAssumptions) start() error {
	if err := n.owner(); err != nil {
		return err
	}
	if err := n.search.open(); err != nil {
		return err
	}
	if n.resultActive || n.localFactor != nil || n.vanishing.active || n.normalization.active || n.projectionFactors.active || n.leading.active || n.psc.active || n.resultant.active || n.discriminant.active || n.cell.active {
		return ErrSearchPending
	}
	n.running, n.resultActive = true, true
	n.marked = n.marked[:0]
	return nil
}
func (n *explanationAssumptions) finish(cause error) error {

	if cause == nil {
		cause = n.resetAlreadyAdded()
		if cause == nil {
			n.resultActive = false
			n.marked = n.marked[:0]
		}
	}
	n.running = false
	n.failed = cause != nil
	return cause
}
func (n *explanationAssumptions) addLiteral(l sat.Literal) error {
	if err := atomAssert("nlsat_explain.cpp", 191, "m_result != 0", n.resultActive); err != nil {
		return err
	}
	if err := atomAssert("nlsat_explain.cpp", 192, "l != true_literal", l != TrueLiteral); err != nil {
		return err
	}
	if l == FalseLiteral {
		return nil
	}
	if !n.resultActive {
		return ErrSearchPending
	}
	i := uint32(l)
	if uint64(i) < uint64(len(n.alreadyAdded)) && n.alreadyAdded[i] {
		return nil
	}
	if uint64(i) >= uint64(len(n.alreadyAdded)) {
		n.alreadyAdded = append(n.alreadyAdded, make([]bool, int(uint64(i)+1)-len(n.alreadyAdded))...)
	}
	n.alreadyAdded[i] = true
	n.marked = append(n.marked, i)
	return n.output.push(l)
}
func (n *explanationAssumptions) resetAlreadyAdded() error {
	if err := atomAssert("nlsat_explain.cpp", 207, "m_result != 0", n.resultActive); err != nil {
		return err
	}
	if !n.resultActive {
		return ErrSearchPending
	}
	for _, l := range n.output.result {
		i := uint32(l)
		if uint64(i) >= uint64(len(n.alreadyAdded)) {
			return arena.ErrBounds
		}
		n.alreadyAdded[i] = false
	}
	return nil
}
func (n *explanationAssumptions) sign(p polynomial.Polynomial) (int, error) {
	r := n.search
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil {
		return 0, err
	}
	if enabled {
		x, e := r.store.atoms.ineq.core.MaxVariable(p)
		if e != nil {
			return 0, e
		}
		ok := x == NullVariable
		if !ok {
			x, e = r.store.atoms.ineq.core.MaxVariable(p)
			if e != nil {
				return 0, e
			}
			ok, e = r.assignment.Contains(x)
			if e != nil {
				return 0, e
			}
		}
		if !ok {
			return 0, diagnostic.DebugAssertionFailure(d, "src/nlsat/nlsat_explain.cpp", 220, "max_var(p) == null_var || m_assignment.is_assigned(max_var(p))")
		}
	}
	return r.evaluator.am.EvalSign(r.store.atoms.ineq.pm, p, r.assignment)
}
func (n *explanationAssumptions) factor(p polynomial.Polynomial) error {
	if err := n.factors.Reset(); err != nil {
		return err
	}
	return n.search.store.atoms.ineq.polynomials.Factor(p, n.factors)
}
func (n *explanationAssumptions) addZeroAssumption(p polynomial.Polynomial) (err error) {
	if n.localFactor != nil {
		return ErrSearchPending
	}
	if err := n.factor(p); err != nil {
		return err
	}
	size, err := n.factors.Size()
	if err != nil {
		return err
	}
	n.zeroFactors = n.zeroFactors[:0]
	n.isEven = n.isEven[:0]
	n.localFactor, err = polynomial.NewPolynomialRef(n.search.store.atoms.ineq.core)
	if err != nil {
		return err
	}
	defer func() {
		complete, closeErr := n.localFactor.SourceFinalize()
		err = errors.Join(err, closeErr)
		if closeErr == nil && complete {
			n.localFactor = nil
		}
	}()
	for i := uint32(0); i < size; i++ {
		p, err := n.factors.Get(i)
		if err != nil {
			return err
		}
		if err = n.localFactor.Assign(p); err != nil {
			return err
		}
		s, err := n.sign(p)
		if err != nil {
			return err
		}
		if s == 0 {
			p, err = n.factors.Get(i)
			if err != nil {
				return err
			}
			n.zeroFactors = append(n.zeroFactors, p)
			n.isEven = append(n.isEven, false)
		}
	}
	if err = atomAssert("nlsat_explain.cpp", 285, "!m_zero_fs.empty()", len(n.zeroFactors) > 0); err != nil {
		return err
	}
	l, err := n.search.store.atoms.mkInequalityLiteral(AtomEQ, n.zeroFactors, n.isEven)
	if err != nil {
		return err
	}
	return n.addLiteral(l.Negated())
}
func (n *explanationAssumptions) addSimpleAssumption(k AtomKind, p polynomial.Polynomial, sign bool) error {
	if err := atomAssert("nlsat_explain.cpp", 293, "k == atom::EQ || k == atom::LT || k == atom::GT", k == AtomEQ || k == AtomLT || k == AtomGT); err != nil {
		return err
	}
	b, err := n.search.store.atoms.mkInequality(k, []polynomial.Polynomial{p}, []bool{false})
	if err != nil {
		return err
	}
	return n.addLiteral(sat.MakeLiteral(b, !sign))
}
func (n *explanationAssumptions) collectZero(p polynomial.Polynomial) (err error) {
	if err = n.start(); err != nil {
		return err
	}
	defer func() { err = n.finish(err) }()
	return n.addZeroAssumption(p)
}
func (n *explanationAssumptions) collectSimple(k AtomKind, p polynomial.Polynomial, sign bool) (err error) {
	if err = n.start(); err != nil {
		return err
	}
	defer func() { err = n.finish(err) }()
	return n.addSimpleAssumption(k, p, sign)
}
func (n *explanationAssumptions) cleanup() error {
	if err := n.owner(); err != nil {
		return err
	}
	if n.running {
		return ErrSearchPending
	}
	n.running = true
	defer func() { n.running = false }()
	if n.discriminant.active {
		if err := n.finishProjectionDiscriminant(true, true); err != nil {
			return err
		}
	} else if n.cell.active {
		if err := n.finishCell(); err != nil {
			return err
		}
	} else if n.psc.active || n.resultant.active {
		if err := n.finishProjectionResultant(true, true); err != nil {
			return err
		}
	} else if n.projectionFactors.active || n.leading.active {
		if err := n.finishProjectionLeading(true, true); err != nil {
			return err
		}
	} else if n.normalization.active {
		if err := n.finishNormalization(true); err != nil {
			return err
		}
	} else if err := n.finishVanishing(true); err != nil {
		return err
	}
	if n.localFactor != nil {
		if err := n.localFactor.Close(); err != nil {
			return err
		}
		n.localFactor = nil
	}
	if n.pscTmp != nil {
		if err := n.pscTmp.CleanupRetained(); err != nil {
			return err
		}
	}
	if n.factors != nil {
		if err := n.factors.CleanupRetained(); err != nil {
			return err
		}
	}
	if n.cell.roots != nil {
		if err := n.cell.roots.CleanupRetained(); err != nil {
			return err
		}
	}
	if err := n.simplifiedCore.cleanup(); err != nil {
		return err
	}
	if err := n.output.cleanup(); err != nil {
		return err
	}
	for _, i := range n.marked {
		n.alreadyAdded[i] = false
	}
	n.marked = n.marked[:0]
	n.resultActive = false
	n.failed = false
	return nil
}
func (n *explanationAssumptions) reset() error {
	if err := n.owner(); err != nil {
		return err
	}
	if err := n.search.open(); err != nil {
		return err
	}
	n.running = true
	err := n.output.reset()
	n.running = false
	n.failed = err != nil
	return err
}
func (n *explanationAssumptions) close() error {
	if n != nil && n.self == n && n.closed {
		return nil
	}
	if err := n.cleanup(); err != nil {
		return err
	}
	n.failed = true
	if err := n.simplifiedCore.close(); err != nil {
		return err
	}
	if err := n.output.close(); err != nil {
		return err
	}
	if n.factors != nil {
		if err := n.factors.Close(); err != nil {
			return err
		}
		n.factors = nil
	}
	if n.pscTmp != nil {
		if err := n.pscTmp.Close(); err != nil {
			return err
		}
		n.pscTmp = nil
	}
	if n.cell.roots != nil {
		if err := n.cell.roots.Close(); err != nil {
			return err
		}
		n.cell.roots = nil
	}
	n.projectionTodo = nil
	n.search.explanationAssumptions = nil
	n.search.store.atoms.ineq.budget.Release(n.header)
	n.header = 0
	n.alreadyAdded = nil
	n.marked = nil
	n.zeroFactors = nil
	n.isEven = nil
	n.output.result = nil
	n.closed = true
	return nil
}

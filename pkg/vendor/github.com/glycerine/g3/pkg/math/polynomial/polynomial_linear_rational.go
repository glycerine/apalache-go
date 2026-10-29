package polynomial

// Source: src/math/polynomial/linear_eq_solver.h.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// RationalLinearSolver borrows its number-field manager. Matrix/RHS values live only in
// native Go slices. Returned observation values are nonowning scalar projections.
type RationalLinearSolver struct {
	self    *RationalLinearSolver
	manager *unumeral.RationalManager
	linearStorage
	A                 [][]unumeral.Rational
	b                 []unumeral.Rational
	numericRoots      unumeral.NumericAllocationRoots
	numericHooks      unumeral.NumericInvocationHooks
	numericGeneration uint64
	numericExecuting  bool
	rawChildren       [2]unumeral.IntegerRawInvocationReceipt
	rawOrder          [2]uint8
	rawCount          uint8
	displayInsertion  unumeral.NumeralInsertionReceipt
}

func NewRationalLinearSolver(m *unumeral.RationalManager) (*RationalLinearSolver, error) {
	if e := m.CheckOwnerReady(); e != nil {
		return nil, e
	}
	enabled, e := linearSourceAssertionsEnabled()
	if e != nil {
		return nil, e
	}
	if enabled && !m.Field() {
		if e := linearSourceAssertionFailure(33, "m.field()"); e != nil {
			return nil, e
		}
	}
	h := uint64(reflect.TypeOf(RationalLinearSolver{}).Size())
	if e := m.Integers().Budget().Reserve(h); e != nil {
		return nil, e
	}
	s := &RationalLinearSolver{manager: m, linearStorage: linearStorage{integer: m.Integers(), header: h}}
	s.self = s
	return s, nil
}
func (s *RationalLinearSolver) identity() error {
	if s == nil || s.self != s || s.manager == nil {
		return ErrPolynomialOwner
	}
	return s.manager.CheckOwnerReady()
}
func (s *RationalLinearSolver) open() error {
	if e := s.identity(); e != nil {
		return e
	}
	if s.closed || s.closing || s.sourceFinalized || s.sourceStage != 0 {
		return arena.ErrClosed
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if s.numericPending() {
		return ErrPolynomialPending
	}
	return nil
}
func (s *RationalLinearSolver) row(i uint32) (*[]unumeral.Rational, error) {
	if uint64(i) >= uint64(len(s.A)) {
		if polynomialSourceDebug {
			return nil, ErrPolynomialAssertion
		}
		return nil, arena.ErrBounds
	}
	return &s.A[i], nil
}
func (s *RationalLinearSolver) swapRows(i, j uint32) error {
	if _, e := s.row(i); e != nil {
		return e
	}
	if _, e := s.row(j); e != nil {
		return e
	}
	s.A[i], s.A[j] = s.A[j], s.A[i]
	return nil
}
func (s *RationalLinearSolver) load(v *[]unumeral.Rational, i uint32) (unumeral.Rational, error) {
	if uint64(i) >= uint64(len(*v)) {
		if polynomialSourceDebug {
			return unumeral.Rational{}, ErrPolynomialAssertion
		}
		return unumeral.Rational{}, arena.ErrBounds
	}
	return (*v)[i], nil
}
func (s *RationalLinearSolver) copyInto(v *[]unumeral.Rational, i uint32, a *unumeral.Rational) error {
	if _, e := s.load(v, i); e != nil {
		return e
	}
	return s.copyWithRoots(&(*v)[i], a)
}
func (s *RationalLinearSolver) setInto(v *[]unumeral.Rational, i uint32, value int64) error {
	if _, e := s.load(v, i); e != nil {
		return e
	}
	ops, e := s.beginNumeric()
	if e != nil {
		return e
	}
	defer s.endNumeric()
	return ops.SetInt64(&(*v)[i], value)
}
func (s *RationalLinearSolver) divideInto(v *[]unumeral.Rational, i uint32, divisor *unumeral.Rational) error {
	if _, e := s.load(v, i); e != nil {
		return e
	}
	ops, e := s.beginNumeric()
	if e != nil {
		return e
	}
	defer s.endNumeric()
	return ops.Div(&(*v)[i], divisor, &(*v)[i])
}
func (s *RationalLinearSolver) submulInto(v *[]unumeral.Rational, i uint32, b, c *unumeral.Rational) error {
	if _, e := s.load(v, i); e != nil {
		return e
	}
	ops, e := s.beginNumeric()
	if e != nil {
		return e
	}
	defer s.endNumeric()
	return ops.SubMul(&(*v)[i], b, c, &(*v)[i])
}
func (s *RationalLinearSolver) deleteInto(v *[]unumeral.Rational, i uint32, owning bool) error {
	if uint64(i) >= uint64(len(*v)) {
		return arena.ErrBounds
	}
	if owning {
		return s.manager.Del(&(*v)[i])
	}
	return unumeral.SourceRationalDel(s.manager, &(*v)[i])
}
func (s *RationalLinearSolver) RightHandSide(i uint32) (unumeral.Rational, error) {
	if e := s.open(); e != nil {
		return unumeral.Rational{}, e
	}
	return s.load(&s.b, i)
}
func (s *RationalLinearSolver) Resize(n uint32) error {
	if s != nil && s.self == s && s.running {
		return ErrPolynomialPending
	}

	if e := s.open(); e != nil {
		return e
	}
	if e := s.beginExecution(); e != nil {
		return e
	}
	defer s.endExecution()
	if s.n == n {
		return nil
	}
	if e := s.flushSource(); e != nil {
		return e
	}
	s.n = n
	s.A = make([][]unumeral.Rational, n)
	s.b = make([]unumeral.Rational, n)
	for i := range s.A {
		s.A[i] = make([]unumeral.Rational, n)
		for j := range s.A[i] {
			s.A[i][j] = unumeral.NewRational()
		}
		s.b[i] = unumeral.NewRational()
	}
	return nil
}
func (s *RationalLinearSolver) Reset() error {
	if s != nil && s.self == s && s.running {
		return ErrPolynomialPending
	}

	if e := s.open(); e != nil {
		return e
	}
	if e := s.beginExecution(); e != nil {
		return e
	}
	defer s.endExecution()
	for i := uint32(0); i < s.n; i++ {
		r, e := s.row(i)
		if e != nil {
			return e
		}
		for j := uint32(0); j < s.n; j++ {
			if e = s.setInto(r, j, 0); e != nil {
				return e
			}
		}
		if e = s.setInto(&s.b, i, 0); e != nil {
			return e
		}
	}
	return nil
}
func (s *RationalLinearSolver) Add(i uint32, as []unumeral.Rational, b *unumeral.Rational) error {
	if s != nil && s.self == s && s.running {
		return ErrPolynomialPending
	}

	if e := s.open(); e != nil {
		return e
	}
	if e := s.beginExecution(); e != nil {
		return e
	}
	defer s.endExecution()
	if e := linearSourceAssert(i < s.n, 78, "i < n"); e != nil {
		return e
	}
	if e := s.copyInto(&s.b, i, b); e != nil {
		return e
	}
	r, e := s.row(i)
	if e != nil {
		return e
	}
	for j := uint32(0); j < s.n; j++ {
		if uint64(j) >= uint64(len(as)) {
			return arena.ErrBounds
		}
		if e = s.copyInto(r, j, &as[j]); e != nil {
			return e
		}
	}
	return nil
}
func (s *RationalLinearSolver) Solve(xs []unumeral.Rational) (bool, error) {
	if s != nil && s.self == s && s.running {
		return false, ErrPolynomialPending
	}

	if e := s.open(); e != nil {
		return false, e
	}
	if e := s.beginExecution(); e != nil {
		return false, e
	}
	defer s.endExecution()
	for k := uint32(0); k < s.n; k++ {
		if e := s.traceIteration(k, 90); e != nil {
			return false, e
		}
		i := k
		for ; i < s.n; i++ {
			r, e := s.row(i)
			if e != nil {
				return false, e
			}
			a, e := s.load(r, k)
			if e != nil {
				return false, e
			}
			zero, e := s.manager.IsZero(&a)
			if e != nil {
				return false, e
			}
			if !zero {
				break
			}
		}
		if i == s.n {
			return false, nil
		}
		// Literal source swaps only matrix row descriptors, not right-hand sides.
		if e := s.swapRows(k, i); e != nil {
			return false, e
		}
		rk, e := s.row(k)
		if e != nil {
			return false, e
		}
		pivot, e := s.load(rk, k)
		if e != nil {
			return false, e
		}
		enabled, e := linearSourceAssertionsEnabled()
		if e != nil {
			return false, e
		}
		if enabled {
			zero, e := s.manager.IsZero(&pivot)
			if e != nil {
				return false, e
			}
			if zero {
				if e := linearSourceAssertionFailure(102, "!m.is_zero(A_k_k)"); e != nil {
					return false, e
				}
			}
		}
		for j := k + 1; j < s.n; j++ {
			if e = s.divideInto(rk, j, &pivot); e != nil {
				return false, e
			}
		}
		if e = s.divideInto(&s.b, k, &pivot); e != nil {
			return false, e
		}
		if e = s.setInto(rk, k, 1); e != nil {
			return false, e
		}
		if polynomialSourceDebug {
			for j := uint32(0); j < k; j++ {
				enabled, e := linearSourceAssertionsEnabled()
				if e != nil {
					return false, e
				}
				if !enabled {
					continue
				}
				a, e := s.load(rk, j)
				if e != nil {
					return false, e
				}
				zero, e := s.manager.IsZero(&a)
				if e != nil {
					return false, e
				}
				if !zero {
					if e := linearSourceAssertionFailure(109, "m.is_zero(A_k[i])"); e != nil {
						return false, e
					}
				}
			}
		}
		for i := k + 1; i < s.n; i++ {
			ri, e := s.row(i)
			if e != nil {
				return false, e
			}
			factor, e := s.load(ri, k)
			if e != nil {
				return false, e
			}
			for j := k + 1; j < s.n; j++ {
				a, e := s.load(rk, j)
				if e != nil {
					return false, e
				}
				if e = s.submulInto(ri, j, &factor, &a); e != nil {
					return false, e
				}
			}
			b, e := s.load(&s.b, k)
			if e != nil {
				return false, e
			}
			if e = s.submulInto(&s.b, i, &factor, &b); e != nil {
				return false, e
			}
			if e = s.setInto(ri, k, 0); e != nil {
				return false, e
			}
		}
	}
	k := s.n
	for k > 0 {
		k--
		if e := s.traceIteration(k, 124); e != nil {
			return false, e
		}
		rk, e := s.row(k)
		if e != nil {
			return false, e
		}
		enabled, e := linearSourceAssertionsEnabled()
		if e != nil {
			return false, e
		}
		if enabled {
			a, e := s.load(rk, k)
			if e != nil {
				return false, e
			}
			one, e := s.manager.IsOne(&a)
			if e != nil {
				return false, e
			}
			if !one {
				if e := linearSourceAssertionFailure(125, "m.is_one(A[k][k])"); e != nil {
					return false, e
				}
			}
		}
		b, e := s.load(&s.b, k)
		if e != nil {
			return false, e
		}
		if uint64(k) >= uint64(len(xs)) {
			return false, arena.ErrBounds
		}
		if e = s.copyWithRoots(&xs[k], &s.b[k]); e != nil {
			return false, e
		}
		i := k
		for i > 0 {
			i--
			ri, e := s.row(i)
			if e != nil {
				return false, e
			}
			a, e := s.load(ri, k)
			if e != nil {
				return false, e
			}
			if e = s.submulInto(&s.b, i, &a, &b); e != nil {
				return false, e
			}
			if e = s.setInto(ri, k, 0); e != nil {
				return false, e
			}
		}
	}
	return true, nil
}
func (s *RationalLinearSolver) flushSource() (err error) {
	if s.sourceCause != nil {
		return s.sourceCause
	}
	defer func() {
		if err != nil {
			s.sourceCause = err
		}
	}()
	if s.flushStage == 0 {
		if e := linearSourceAssert(len(s.b) == len(s.A), 37, "b.size() == A.size()"); e != nil {
			return e
		}
		s.flushStage = 1
	}
	for uint64(s.flushRow) < uint64(len(s.A)) {
		r, e := s.row(s.flushRow)
		if e != nil {
			return e
		}
		if s.flushStage == 1 {
			if e = s.deleteInto(&s.b, s.flushRow, false); e != nil {
				return e
			}
			s.flushStage = 2
		}
		if e := linearSourceAssert(uint64(len(*r)) == uint64(s.n), 42, "as.size() == n"); e != nil {
			return e
		}
		for s.flushColumn < s.n {
			if e = s.deleteInto(r, s.flushColumn, false); e != nil {
				return e
			}
			s.flushColumn++
		}
		s.flushColumn = 0
		s.flushRow++
		s.flushStage = 1
	}
	// Numeric destruction completed in source order; native containers need no destructor.
	clear(s.A)
	s.A = s.A[:0]
	s.b = s.b[:0]
	s.n = 0
	s.flushRow, s.flushColumn, s.flushStage = 0, 0, 0
	return nil
}
func (s *RationalLinearSolver) Flush() error {
	if s != nil && s.self == s && s.running {
		return ErrPolynomialPending
	}

	if e := s.open(); e != nil {
		return e
	}
	if e := s.beginExecution(); e != nil {
		return e
	}
	defer s.endExecution()
	return s.flushSource()
}
func (s *RationalLinearSolver) SourceFinalize() (err error) {
	if s != nil && s.self == s && s.running {
		return ErrPolynomialPending
	}

	if e := s.identity(); e != nil {
		return e
	}
	if s.closed {
		return arena.ErrClosed
	}
	if s.sourceFinalized {
		return nil
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if e := s.beginExecution(); e != nil {
		return e
	}
	defer s.endExecution()
	defer func() {
		if err != nil {
			s.sourceCause = err
		}
	}()
	if s.sourceStage == 0 {
		if e := s.flushSource(); e != nil {
			return e
		}
		s.sourceStage = 1
	}
	s.b = nil
	s.A = nil
	s.sourceStage = 3
	s.sourceFinalized = true
	return nil
}
func (s *RationalLinearSolver) Close() error {
	if s != nil && s.self == s && s.running {
		return ErrPolynomialPending
	}

	if s == nil || s.self != s {
		return ErrPolynomialOwner
	}
	if s.closed {
		return nil
	}
	if e := s.identity(); e != nil {
		return e
	}
	if e := cleanupLinearDisplayInsertion(&s.displayInsertion, s.integer); e != nil {
		return e
	}
	if !s.closing {
		s.closing = true
		if e := s.SourceFinalize(); e != nil {
			return e
		}
	}
	if e := s.beginExecution(); e != nil {
		return e
	}
	defer s.endExecution()
	// Explicit owning continuation releases actual numeral resources after a stopped source destructor.
	if e := s.cleanupNumericChildren(false); e != nil {
		return e
	}
	for i := range s.A {
		for j := range s.A[i] {
			if e := s.deleteInto(&s.A[i], uint32(j), true); e != nil {
				return e
			}
		}
	}
	for i := range s.b {
		if e := s.deleteInto(&s.b, uint32(i), true); e != nil {
			return e
		}
	}
	if e := s.cleanupNumericChildren(true); e != nil {
		return e
	}
	if e := s.numericRoots.Cleanup(s.integer); e != nil {
		return e
	}
	s.A, s.b = nil, nil
	s.integer.Budget().Release(s.header)
	s.header = 0
	s.closed = true
	return nil
}

func (s *RationalLinearSolver) beginNumeric() (unumeral.NumericRationalOperations, error) {
	if !s.running || s.numericExecuting || s.numericGeneration == ^uint64(0) {
		return unumeral.NumericRationalOperations{}, ErrPolynomialPending
	}
	s.numericExecuting = true
	s.numericGeneration++
	s.numericHooks.Policy = s
	s.numericHooks.Generation = s.numericGeneration
	return s.manager.WithAllocationContext(unumeral.NumericAllocationContext{}.WithCaller(&s.numericHooks)), nil
}
func (s *RationalLinearSolver) endNumeric() { s.numericExecuting = false }
func (s *RationalLinearSolver) copyWithRoots(dst, src *unumeral.Rational) error {
	ops, e := s.beginNumeric()
	if e != nil {
		return e
	}
	defer s.endNumeric()
	return ops.Copy(dst, src)
}
func (s *RationalLinearSolver) NumericBound(executing bool) bool {
	return s != nil && s.self == s && s.manager != nil && s.numericGeneration != 0 && s.numericHooks.Generation == s.numericGeneration && !s.closed && (!executing || (s.running && s.numericExecuting))
}
func (s *RationalLinearSolver) NumericRoots(z *unumeral.IntegerManager, local *unumeral.NumericAllocationRoots) *unumeral.NumericAllocationRoots {
	if !s.NumericBound(true) || s.integer != z {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.numericRoots
}
func (s *RationalLinearSolver) NumericRecord(z *unumeral.IntegerManager, _ string, receipt unumeral.NumericInvocationReceipt) {
	if !s.NumericBound(true) || s.integer != z || receipt.Raw == nil || !receipt.Raw.Matches(z) {
		return
	}
	var index uint8
	switch receipt.Raw.Kind() {
	case unumeral.IntegerRawDivision:
		index = 0
	case unumeral.IntegerRawGCD:
		index = 1
	default:
		return
	}
	s.rawChildren[index] = *receipt.Raw
	for i := uint8(0); i < s.rawCount; i++ {
		if s.rawOrder[i] == index {
			copy(s.rawOrder[i:s.rawCount-1], s.rawOrder[i+1:s.rawCount])
			s.rawCount--
			break
		}
	}
	s.rawOrder[s.rawCount] = index
	s.rawCount++
}
func (s *RationalLinearSolver) numericPending() bool {
	if s.displayInsertion.Active() || unumeral.NumericAllocationPending(&s.numericRoots) {
		return true
	}
	for i := uint8(0); i < s.rawCount; i++ {
		if s.rawChildren[s.rawOrder[i]].Matches(s.integer) {
			return true
		}
	}
	return false
}
func (s *RationalLinearSolver) cleanupNumericChildren(abort bool) error {
	for i := int(s.rawCount) - 1; i >= 0; i-- {
		r := &s.rawChildren[s.rawOrder[i]]
		if r.Matches(s.integer) {
			if e := r.Cleanup(abort); e != nil {
				return e
			}
		}
	}
	return nil
}

package polynomial

// Concrete finite-field square-free decomposition and Berlekamp source bodies.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// ErrUPolynomialNontermination guards a source uint32 loop at its wrap boundary.
var ErrUPolynomialDomain = errors.New("mpnum: polynomial source domain")
var ErrUPolynomialNontermination = errors.New("upolynomial source loop does not terminate")

func (m *UPolynomialManager) factorPrime() (uint32, error) {
	p, e := m.modular.BorrowedModulus()
	if e != nil {
		return 0, e
	}
	ok, e := m.config.Integer.IsUint64(p)
	if e != nil {
		return 0, e
	}
	if !ok {
		return 0, newUPolynomialPrimeException()
	}
	n, e := m.config.Integer.Uint64(p)
	if e != nil {
		return 0, e
	}
	if uint64(uint32(n)) != n {
		return 0, newUPolynomialPrimeException()
	}
	return uint32(n), nil
}
func (m *UPolynomialManager) checkFactors(fs *UPolynomialFactors) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := fs.check(); e != nil {
		return e
	}
	if fs.manager != m {
		return ErrUPolynomialOwner
	}
	return nil
}
func (m *UPolynomialManager) scopedPush(s *upolyVectorState, value *unumeral.Int) error {
	out, e := m.pushDefault(s)
	if e != nil {
		return e
	}
	return m.modular.Copy(out, value)
}

// The source matrix is lexical. Native Go arrays hold its pivots and
// coefficients; interrupted destruction retains the actual matrix owner.
type upolyBerlekampMatrix struct {
	displaying                           bool
	constructed, destroying, sourceGuard bool
	deleteIndex                          uint32
	ownIndex                             uint32
	ownStarted                           bool
	closeCause                           error
	constructorTemp, diagonalTemp        upolyBerlekampScopedNumeral
	numeric                              upolyBerlekampNumeric
	split                                upolyBerlekampSplit
	traceBorrow                          upolyFactorTraceBorrow
	entryTrace                           upolyDisplayInvocationReceipt
	outcomeFactors                       upolyFactorsDisplayReceipt
	m                                    *UPolynomialManager
	values                               upolyVectorState
	size, nullRow                        uint32
	column, row                          []int32
}

// close keeps the lexical matrix reachable through its actual manager when
// a source destructor or constructor-abort cleanup cannot complete.
func (q *upolyBerlekampMatrix) close() {
	if e := q.closeSource(); e != nil && q.m != nil && !q.displaying && !q.destroying && !q.numeric.executing && !q.numeric.cleaning {
		if q.m.pendingBerlekamp == nil || q.m.pendingBerlekamp == q {
			if q.closeCause == nil {
				q.closeCause = e
			}
			q.m.pendingBerlekamp = q
		}
	}
}
func (q *upolyBerlekampMatrix) closeSource() error {
	if q.displaying || q.destroying || q.numeric.executing || q.numeric.cleaning {
		return ErrUPolynomialCleanupPending
	}
	if q.m == nil {
		return nil
	}
	if e := q.traceSourceBlocker(); e != nil {
		return e
	}
	if q.split.declared && q.split.sourceRemaining != 0 && !q.split.ownEnded {
		if q.split.cause != nil {
			return q.split.cause
		}
		return ErrUPolynomialCleanupPending
	}
	if e := q.numeric.sourceCause(); e != nil {
		return e
	}
	if q.m.displayReceiptPending(&q.entryTrace) {
		return ErrUPolynomialCleanupPending
	}
	for _, local := range []*upolyBerlekampScopedNumeral{&q.diagonalTemp, &q.constructorTemp} {
		if local.declared && !local.sourceEnded && !local.ownEnded {
			if local.cause != nil {
				return local.cause
			}
			return ErrUPolynomialCleanupPending
		}
	}
	if q.values.backing == 0 && q.column == nil && q.row == nil {
		return nil
	}
	if q.sourceGuard {
		return q.closeCause
	}
	q.destroying = true
	defer func() { q.destroying = false }()
	m := q.m
	if !q.constructed {
		// A failed C++ constructor grants no class destructor. This separate Go
		// owning abort reclaims only the actual member coefficients acquired so far.
		return q.closeOwnedMembers()
	}
	for q.deleteIndex < q.values.size {
		guard, e := upolynomialSourceNumeralDel(m.config.Integer, &m.values(&q.values)[q.deleteIndex])
		if e != nil {
			q.sourceGuard, q.closeCause = guard, e
			return e
		}
		q.deleteIndex++
	}
	q.releaseMembers()
	return nil
}
func (q *upolyBerlekampMatrix) releaseMembers() {
	q.row = nil
	q.column = nil
	q.m.freeBacking(&q.values)
	q.constructed = false
	if q.m.pendingBerlekamp == q && !q.numeric.pending() {
		q.m.pendingBerlekamp = nil
	}
}

// Owning cleanup does not retry a stopped source capacity read. It uses only
// the remaining concrete member values, preserving the original source cause.
func (q *upolyBerlekampMatrix) closeOwnedMembers() error {
	if e := q.cleanupMonicOwned(); e != nil {
		return e
	}
	local := &q.numeric.divisionAM
	if local.declared && !local.sourceEnded && !local.ownEnded {
		if e := q.m.config.Integer.CleanupRawNumeral(&local.value); e != nil {
			return e
		}
		local.ownEnded = true
	}
	if e := q.cleanupSplitOwned(); e != nil {
		return e
	}
	for _, local := range []*upolyBerlekampScopedNumeral{&q.diagonalTemp, &q.constructorTemp} {
		if local.declared && !local.sourceEnded && !local.ownEnded {
			if e := q.m.config.Integer.CleanupRawNumeral(&local.value); e != nil {
				return e
			}
			local.ownEnded = true
		}
	}
	if !q.ownStarted {
		q.ownIndex, q.ownStarted = q.deleteIndex, true
	}
	for q.ownIndex < q.values.size {
		if e := q.m.config.Integer.CleanupRawNumeral(&q.m.values(&q.values)[q.ownIndex]); e != nil {
			return e
		}
		q.ownIndex++
	}
	q.releaseMembers()
	return nil
}
func (q *upolyBerlekampMatrix) get(i, j uint32) (*unumeral.Int, error) {
	return q.getSource(i, j, 74)
}
func (q *upolyBerlekampMatrix) getConst(i, j uint32) (*unumeral.Int, error) {
	return q.getSource(i, j, 79)
}
func (q *upolyBerlekampMatrix) getSource(i, j uint32, line int32) (*unumeral.Int, error) {
	enabled, e := factorHeaderAssertionsEnabled()
	if e != nil {
		return nil, e
	}
	if enabled && !(i < q.size && j < q.size) {
		owned, e := q.beginAssertionBorrow()
		if e != nil {
			return nil, e
		}
		defer q.endAssertionBorrow(owned)
		if e := factorizationSourceAssertionFailure(line, "i < m_size && j < m_size"); e != nil {
			return nil, e
		}
	}
	// Typed backing/domain adapter: avoid a wrapped matrix index aliasing cells.
	index := uint64(i)*uint64(q.size) + uint64(j)
	if index > math.MaxUint32 {
		return nil, arena.ErrBounds
	}
	return q.m.read(q.m.stateView(&q.values), uint32(index))
}
func (q *upolyBerlekampMatrix) init(m *UPolynomialManager, f UPolynomialView) (cause error) {
	if m != nil && m.pendingBerlekamp != nil {
		return ErrUPolynomialCleanupPending
	}
	if q.displaying || q.numeric.executing || q.numeric.cleaning || q.m != nil && q.m.displayReceiptPending(&q.entryTrace) {
		return ErrUPolynomialCleanupPending
	}
	if q.m != nil && q.m.pendingBerlekamp == q {
		return ErrUPolynomialCleanupPending
	}
	q.deleteIndex, q.closeCause, q.sourceGuard, q.constructed = 0, nil, false, false
	q.ownIndex, q.ownStarted = 0, false
	q.constructorTemp, q.diagonalTemp = upolyBerlekampScopedNumeral{}, upolyBerlekampScopedNumeral{}
	q.split = upolyBerlekampSplit{}
	q.m = m
	q.size = upolyViewDegree(f)
	q.nullRow = 1
	n := q.size
	if e := upolyFactorInt32Resize(&q.column, n, -1); e != nil {
		return e
	}
	if e := upolyFactorInt32Resize(&q.row, n, -1); e != nil {
		return e
	}
	p, e := m.factorPrime()
	if e != nil {
		return e
	}
	if e = q.traceConstructor(f, p); e != nil {
		return e
	}

	v, e := m.pushDefault(&q.values)
	if e != nil {
		return e
	}
	*v = unumeral.NewInt(1)
	for j := uint32(0); j < n; j++ {
		if _, e = m.pushDefault(&q.values); e != nil {
			return e
		}
	}
	q.constructorTemp.declared = true
	tmp := &q.constructorTemp.value
	defer func() {
		cause = q.finishScopedNumeral(&q.constructorTemp, cause)
		q.constructed = cause == nil
	}()
	row, previous := uint32(0), uint32(0)
	for k := uint32(1); ; k++ {
		if p == 0 {
			return ErrUPolynomialDomain
		} // actual undefined modulo read site
		if k%p == 1 {
			row++
			if row >= n {
				break
			}
			for j := uint32(0); j < n; j++ {
				if _, e = m.pushDefault(&q.values); e != nil {
					return e
				}
			}
		}
		top, e := q.get(previous, n-1)
		if e != nil {
			return e
		}
		if e = q.numericCall(0, top, nil, nil, tmp); e != nil {
			return e
		}
		for j := n - 1; j > 0; j-- {
			old, e := q.get(previous, j-1)
			if e != nil {
				return e
			}
			fj, e := m.read(f, j)
			if e != nil {
				return e
			}
			out, e := q.get(row, j)
			if e != nil {
				return e
			}
			if e = q.numericCall(1, old, tmp, fj, out); e != nil {
				return e
			}
		}
		f0, e := m.read(f, 0)
		if e != nil {
			return e
		}
		out, e := q.get(row, 0)
		if e != nil {
			return e
		}
		if e = q.numericCall(2, f0, tmp, nil, out); e != nil {
			return e
		}
		if e = q.numericCall(3, nil, nil, nil, out); e != nil {
			return e
		}
		previous = row
		if k == math.MaxUint32 {
			return ErrUPolynomialNontermination
		}
	}
	for i := uint32(0); i < n; i++ {
		a, e := q.get(i, i)
		if e != nil {
			return e
		}
		if e = q.numericCall(6, nil, nil, nil, a); e != nil {
			return e
		}
	}
	if e := q.traceMatrix(142); e != nil {
		return e
	}
	return nil
}
func (q *upolyBerlekampMatrix) diagonalize() (answer uint32, cause error) {
	if q.displaying || q.numeric.executing || q.numeric.cleaning || q.m != nil && q.m.displayReceiptPending(&q.entryTrace) {
		return 0, ErrUPolynomialCleanupPending
	}
	if q.m.pendingBerlekamp != nil {
		return 0, ErrUPolynomialCleanupPending
	}
	m := q.m
	q.diagonalTemp = upolyBerlekampScopedNumeral{declared: true}
	multiplier := &q.diagonalTemp.value
	defer func() { cause = q.finishScopedNumeral(&q.diagonalTemp, cause) }()
	nullRank := uint32(0)
	for i := uint32(0); i < q.size; i++ {
		found := false
		for j := uint32(0); j < q.size; j++ {
			if q.column[j] >= 0 {
				continue
			}
			a, e := q.get(i, j)
			if e != nil {
				return 0, e
			}
			zero, e := m.modular.IsZero(a)
			if e != nil {
				return 0, e
			}
			if zero {
				continue
			}
			found = true
			q.column[j] = int32(i)
			q.row[i] = int32(j)
			if e = q.numericCall(0, a, nil, nil, multiplier); e != nil {
				return 0, e
			}
			if e = q.numericCall(4, nil, nil, nil, multiplier); e != nil {
				return 0, e
			}
			if e = q.numericCall(3, nil, nil, nil, multiplier); e != nil {
				return 0, e
			}
			for k := q.nullRow; k < q.size; k++ {
				a, e := q.get(k, j)
				if e != nil {
					return 0, e
				}
				if e = q.numericCall(2, a, multiplier, nil, a); e != nil {
					return 0, e
				}
			}
			for other := uint32(0); other < q.size; other++ {
				if other == j {
					continue
				}
				a, e := q.get(i, other)
				if e != nil {
					return 0, e
				}
				if e = q.numericCall(0, a, nil, nil, multiplier); e != nil {
					return 0, e
				}
				for k := q.nullRow; k < q.size; k++ {
					a, e := q.get(k, other)
					if e != nil {
						return 0, e
					}
					pivot, e := q.get(k, j)
					if e != nil {
						return 0, e
					}
					if e = q.numericCall(5, a, multiplier, pivot, a); e != nil {
						return 0, e
					}
				}
			}
		}
		if !found {
			nullRank++
		}
	}
	if e := q.traceMatrix(198); e != nil {
		return 0, e
	}
	return nullRank, nil
}
func (q *upolyBerlekampMatrix) next(v *upolyVectorState) (bool, error) {
	if q.m != nil && q.m.pendingBerlekamp != nil {
		return false, ErrUPolynomialCleanupPending
	}
	if q.displaying || q.numeric.executing || q.numeric.cleaning || q.m != nil && q.m.displayReceiptPending(&q.entryTrace) {
		return false, ErrUPolynomialCleanupPending
	}
	m := q.m
	enabled, e := factorHeaderAssertionsEnabled()
	if e != nil {
		return false, e
	}
	if enabled && !(v.size <= q.size) {
		owned, e := q.beginAssertionBorrow()
		if e != nil {
			return false, e
		}
		defer q.endAssertionBorrow(owned)
		if e := factorizationSourceAssertionFailure(209, "v.size() <= m_size"); e != nil {
			return false, e
		}
	}
	// scoped_numeral_vector::resize, default zero cells; logical shrink deletes.
	if q.size <= v.size {
		if e := m.shrink(v, q.size); e != nil {
			return false, e
		}
	}
	if e := m.reserve(v, q.size); e != nil {
		return false, e
	}
	for ; q.nullRow < q.size; q.nullRow++ {
		if q.row[q.nullRow] >= 0 {
			continue
		}
		for j := uint32(0); j < q.size; j++ {
			out, e := m.read(m.stateView(v), j)
			if e != nil {
				return false, e
			}
			if q.row[j] >= 0 {
				a, e := q.get(q.nullRow, uint32(q.row[j]))
				if e != nil {
					return false, e
				}
				if e = m.modular.Copy(out, a); e != nil {
					return false, e
				}
			} else {
				n := int32(0)
				if j == q.nullRow {
					n = 1
				}
				if e = m.modular.SetInt32(out, n); e != nil {
					return false, e
				}
			}
		}
		q.nullRow++
		return true, nil
	}
	return false, nil
}
func (m *UPolynomialManager) FactorZPSquareFreeBerlekamp(f UPolynomialView, fs *UPolynomialFactors, randomized bool) (bool, error) {
	if e := m.checkFactors(fs); e != nil {
		return false, e
	}
	return m.factorZPBerlekamp(f, fs, randomized)
}
func (m *UPolynomialManager) FactorZPSquareFree(f UPolynomialView, fs *UPolynomialFactors) (bool, error) {
	return m.FactorZPSquareFreeBerlekamp(f, fs, false)
}
func (m *UPolynomialManager) factorZPBerlekamp(f UPolynomialView, fs *UPolynomialFactors, randomized bool) (answer bool, cause error) {
	// The actual source accepts randomized but implements the same deterministic
	// splitting body for either value; no RNG is substituted.
	enabled, e := factorHeaderAssertionsEnabled()
	if e != nil {
		return false, e
	}
	if enabled && !(upolyViewDegree(f) > 1) {
		if e := m.factorizationAssertionFailure(409, "upm.degree(f) > 1"); e != nil {
			return false, e
		}
	}

	p, e := m.factorPrime()
	if e != nil {
		return false, e
	}
	var matrix upolyBerlekampMatrix
	matrix.m = m
	defer func() {
		matrix.close()
		if m.pendingBerlekamp == &matrix {
			cause = errors.Join(cause, matrix.closeCause, ErrUPolynomialCleanupPending)
		}
	}()
	if e = matrix.traceFactorEntry(f, p); e != nil {
		return false, e
	}
	enabled, e = factorHeaderAssertionsEnabled()
	if e != nil {
		return false, e
	}
	if enabled {
		top, e := m.read(f, f.count-1)
		if e != nil {
			return false, e
		}
		one, e := m.modular.IsOne(top)
		if e != nil {
			return false, e
		}
		if !one {
			if e := m.factorizationAssertionFailure(415, "zpm.is_one(f.back())"); e != nil {
				return false, e
			}
		}
	}
	if e = matrix.init(m, f); e != nil {
		return false, e
	}
	first := fs.count
	if e = fs.PushCopy(f, 1); e != nil {
		return false, e
	}
	rank, e := matrix.diagonalize()
	if e != nil {
		return false, e
	}
	if rank == 1 {
		if e := matrix.traceIrreducible(f, fs, p); e != nil {
			return false, e
		}
		return false, nil
	}
	if e := matrix.traceSplitRank(rank); e != nil {
		return false, e
	}
	matrix.split = upolyBerlekampSplit{declared: true, sourceRemaining: 3}
	gcd, div, v := &matrix.split.gcd, &matrix.split.div, &matrix.split.vector
	defer func() { cause = matrix.finishSplitSource(cause) }()
	for {
		has, e := matrix.next(v)
		if e != nil {
			return false, e
		}
		if !has {
			break
		}
		if e := matrix.traceNullVector(); e != nil {
			return false, e
		}
		if e = m.trim(v); e != nil {
			return false, e
		}
		end := fs.count
		for i := first; i < end; i++ {
			if fs.factors[i].size == 2 {
				continue
			}
			for s := uint32(0); s < p; s++ {
				v0, e := m.read(m.stateView(v), 0)
				if e != nil {
					return false, e
				}
				if e = matrix.numericCall(6, nil, nil, nil, v0); e != nil {
					return false, e
				}
				current := &fs.factors[i]
				if e = matrix.vectorNumericCall(false, m.stateView(v), m.stateView(current), gcd); e != nil {
					return false, e
				}
				if gcd.size != 1 && gcd.size != current.size {
					if e = matrix.vectorNumericCall(true, m.stateView(current), m.stateView(gcd), div); e != nil {
						return false, e
					}
					if e := matrix.traceSplitOperands(m.stateView(current)); e != nil {
						return false, e
					}
					degree, e := fs.multiplicity(i)
					if e != nil {
						return false, e
					}
					fs.totalDegree -= upolyStateDegree(current) * degree
					fs.totalDegree += upolyStateDegree(div) * degree
					*current, *div = *div, *current
					if e = fs.PushCopy(m.stateView(gcd), 1); e != nil {
						return false, e
					}
				}
				if fs.count-first == rank {
					if e := matrix.traceSplitOutcome(f, fs, p); e != nil {
						return false, e
					}
					return true, nil
				}
			}
		}
	}
	enabled, e = factorHeaderAssertionsEnabled()
	if e != nil {
		return false, e
	}
	if enabled {
		if e := m.factorizationAssertionFailure(506, "false"); e != nil {
			return false, e
		}
	}
	return true, nil
}
func (m *UPolynomialManager) SquareFreeFactorZP(f UPolynomialView, fs *UPolynomialFactors) error {
	if e := m.checkFactors(fs); e != nil {
		return e
	}
	return m.squareFreeFactorZP(f, fs)
}
func (m *UPolynomialManager) squareFreeFactorZP(f UPolynomialView, fs *UPolynomialFactors) (failure error) {
	p, e := m.factorPrime()
	if e != nil {
		return e
	}
	if p == 0 {
		return ErrUPolynomialDomain
	}
	q, e := m.beginFactorField(factorFieldSquare)
	if e != nil {
		return e
	}
	defer func() { failure = q.finish(failure) }()
	if e = m.traceFactorFieldEntry(q, f, p, 256); e != nil {
		return e
	}
	t0 := &q.vectors[1]
	q.constructed = 1
	enabled, e := factorHeaderAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled && !(f.count > 1) {
		if e := m.factorizationAssertionFailure(262, "f.size() > 1"); e != nil {
			return e
		}
	}

	q.constructed = 2
	if e = m.setView(t0, f); e != nil {
		return e
	}
	q.constructed = 3
	constant := &q.constant
	// mk_monic(lc) has its own scoped inverse destroyed before output constant.
	if e = m.factorMonic(m.stateView(t0), constant); e != nil {
		return e
	}
	if e = fs.SetConstant(constant); e != nil {
		return e
	}
	if e = m.traceSquareFreeInitial(q, fs, m.stateView(t0), p); e != nil {
		return e
	}
	exponent := uint32(1)
	t0d, t, v, w, a := &q.vectors[2], &q.vectors[3], &q.vectors[4], &q.vectors[5], &q.vectors[6]
	q.constructed = 8
	for t0.size > 1 {
		k := uint32(0)
		if e = m.traceSquareFreeKZero(); e != nil {
			return e
		}
		if e = m.derivativeState(m.stateView(t0), t0d); e != nil {
			return e
		}
		if e = m.traceSquareFreeState(q, 289, "T_0_d = T_0.derivative(x)", "T_0_d == ", m.stateView(t0d)); e != nil {
			return e
		}
		if e = m.gcd(m.stateView(t0), m.stateView(t0d), t); e != nil {
			return e
		}
		if e = m.traceSquareFreeState(q, 296, "T = T_0.gcd(T_0_d)", "T == ", m.stateView(t)); e != nil {
			return e
		}
		if e = m.divState(m.stateView(t0), m.stateView(t), v); e != nil {
			return e
		}
		if e = m.traceSquareFreeState(q, 303, "V = T_0.quo_rem(T)[0]", "V == ", m.stateView(v)); e != nil {
			return e
		}
		for v.size > 1 {
			k++
			if k%p == 0 {
				k++
				if e = m.divState(m.stateView(t), m.stateView(v), t); e != nil {
					return e
				}
				if e = m.traceSquareFreeState(q, 314, "T = T.quo_rem(V)[0]", "T == ", m.stateView(t)); e != nil {
					return e
				}
			}
			if e = m.gcd(m.stateView(t), m.stateView(v), w); e != nil {
				return e
			}
			if e = m.traceSquareFreeState(q, 324, "W = T.gcd(V)", "", m.stateView(w)); e != nil {
				return e
			}
			if e = m.divState(m.stateView(v), m.stateView(w), a); e != nil {
				return e
			}
			if e = m.traceSquareFreeState(q, 331, "A_ek = V.quo_rem(W)[0]", "A_ek == ", m.stateView(a)); e != nil {
				return e
			}
			*v, *w = *w, *v
			if e = m.traceSquareFreeState(q, 338, "V = W", "V == ", m.stateView(v)); e != nil {
				return e
			}
			if e = m.divState(m.stateView(t), m.stateView(v), t); e != nil {
				return e
			}
			if e = m.traceSquareFreeState(q, 345, "T = T.quo_rem(V)[0]", "T == ", m.stateView(t)); e != nil {
				return e
			}
			if a.size > 1 {
				if e = m.traceSquareFreeFactor(q, m.stateView(a), exponent*k); e != nil {
					return e
				}
				if e = fs.PushCopy(m.stateView(a), exponent*k); e != nil {
					return e
				}
			}
		}
		exponent *= p
		if e = m.reset(t0); e != nil {
			return e
		}
		for deg := uint32(0); deg < t.size; {
			zero := unumeral.NewInt(0)
			if e = m.scopedPush(t0, &zero); e != nil {
				return e
			}
			input, e := m.read(m.stateView(t), deg)
			if e != nil {
				return e
			}
			out, e := m.read(m.stateView(t0), t0.size-1)
			if e != nil {
				return e
			}
			if e = m.modular.Copy(out, input); e != nil {
				return e
			}
			if deg > math.MaxUint32-p {
				return ErrUPolynomialNontermination
			}
			deg += p
		}
		if e = m.traceSquareFreeState(q, 364, "", "T_0 = ", m.stateView(t0)); e != nil {
			return e
		}
	}
	return m.traceFactorFieldOutcome(q, f, fs, 369)
}
func (m *UPolynomialManager) factorMonic(v UPolynomialView, leading *unumeral.Int) error {
	var inverse unumeral.Int
	defer m.modular.Del(&inverse)
	return m.monic(v, leading, &inverse)
}
func (m *UPolynomialManager) FactorZP(f UPolynomialView, fs *UPolynomialFactors) (answer bool, failure error) {
	if e := m.checkFactors(fs); e != nil {
		return false, e
	}
	p, e := m.factorPrime()
	if e != nil {
		return false, e
	}
	q, e := m.beginFactorField(factorFieldWhole)
	if e != nil {
		return false, e
	}
	defer func() { failure = q.finish(failure) }()
	if e = m.traceFactorFieldEntry(q, f, p, 375); e != nil {
		return false, e
	}
	sq, e := m.NewFactors()
	if e != nil {
		return false, e
	}
	q.sq = sq
	if e = m.squareFreeFactorZP(f, sq); e != nil {
		return false, e
	}
	for i := uint32(0); i < sq.count; i++ {
		start := fs.count
		v := m.stateView(&sq.factors[i])
		if upolyViewDegree(v) > 1 {
			if _, e = m.factorZPBerlekamp(v, fs, false); e != nil {
				return false, e
			}
			for j := start; j < fs.count; j++ {
				k, e := fs.multiplicity(j)
				if e != nil {
					return false, e
				}
				mult, e := sq.multiplicity(i)
				if e != nil {
					return false, e
				}
				if e = fs.SetMultiplicity(j, mult*k); e != nil {
					return false, e
				}
			}
		} else {
			k, e := sq.multiplicity(i)
			if e != nil {
				return false, e
			}
			if e = fs.PushCopy(v, k); e != nil {
				return false, e
			}
		}
	}
	if e = fs.SetConstant(&sq.constant); e != nil {
		return false, e
	}
	if e = m.traceFactorFieldOutcome(q, f, fs, 399); e != nil {
		return false, e
	}
	return fs.totalFactors > 1, nil
}

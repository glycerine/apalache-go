package lp

// Source: src/util/lp/bound_analyzer_on_row.h, retained exact row consumers.

import "github.com/glycerine/g3/pkg/util/numeral"

// LARRowBoundAnalyzer borrows a stable iterator and propagator, and owns a copy
// of the RHS and its arithmetic temporaries. Close never closes either borrow.
// Analyze starts at the current cursor; its missing-bound counters persist.
// Retained solver callers supply zero RHS. Source all-column paths ignore RHS,
// while single-missing-column paths start at -RHS.X; RHS.Y is unused.
// Do not copy this owner. Even failed construction returns an owner to close.
type LARRowBoundAnalyzer struct {
	iterator             RationalLinearIterator
	propagator           *LARBoundPropagator
	manager              *numeral.RationalManager
	RightSide            RationalPair
	RowOrTermIndex       uint32
	ColumnOfU, ColumnOfL int32
	calls                exactEtaCalls
	before               exactEtaBefore
	tracking, stopped    bool
	work, phase          *IndexedRationalVector
	condition            *RationalPair
}

func NewLARRowBoundAnalyzer(it RationalLinearIterator, rhs *RationalPair, row uint32, bp *LARBoundPropagator) (a *LARRowBoundAnalyzer, err error) {
	a = &LARRowBoundAnalyzer{iterator: it, propagator: bp, manager: bp.manager, RightSide: NewRationalPair(), RowOrTermIndex: row, ColumnOfU: -1, ColumnOfL: -1}
	if it.NumeralManager() != bp.manager {
		a.stopped = true
		return a, ErrExactVectorOwner
	}
	before, err := a.calls.begin(a.manager)
	if err != nil {
		a.stopped = true
		return a, err
	}
	err = a.RightSide.Copy(a.manager, rhs)
	err = a.calls.after(a.manager, before, err)
	a.stopped = err != nil
	return a, err
}

func (a *LARRowBoundAnalyzer) resume() error {
	var err error
	a.before, err = a.calls.begin(a.manager)
	a.tracking = err == nil
	return err
}
func (a *LARRowBoundAnalyzer) suspend() error {
	if !a.tracking {
		return nil
	}
	a.tracking = false
	return a.calls.after(a.manager, a.before, nil)
}
func (a *LARRowBoundAnalyzer) next(out *numeral.Rational) (j uint32, ok bool, err error) {
	if err = a.suspend(); err != nil {
		return
	}
	j, ok, err = a.iterator.Next(out)
	if err == nil {
		err = a.resume()
	}
	return
}
func (a *LARRowBoundAnalyzer) reset() error {
	if err := a.suspend(); err != nil {
		return err
	}
	if err := a.iterator.Reset(); err != nil {
		return err
	}
	return a.resume()
}
func (a *LARRowBoundAnalyzer) limit(j uint32, value *numeral.Rational, positive, low, strict bool) error {
	if err := a.suspend(); err != nil {
		return err
	}
	if err := a.propagator.TryAddBound(value, j, low, positive, a.RowOrTermIndex, strict); err != nil {
		return err
	}
	return a.resume()
}
func (a *LARRowBoundAnalyzer) classify(j uint32, value *numeral.Rational) error {
	t, err := a.propagator.ColumnType(j)
	if err != nil {
		return err
	}
	advance := func(column *int32) {
		if *column == -1 {
			*column = int32(j)
		} else {
			*column = -2
		}
	}
	switch t {
	case ColumnLowBound, ColumnUpperBound:
		s, err := a.manager.Sign(value)
		if err != nil {
			return err
		}
		if t == ColumnLowBound && s > 0 || t == ColumnUpperBound && s < 0 {
			advance(&a.ColumnOfU)
		} else {
			advance(&a.ColumnOfL)
		}
	case ColumnFree:
		advance(&a.ColumnOfU)
		advance(&a.ColumnOfL)
	}
	return nil
}

// endpoint selects a monoid extremum without multiplying. Strictness is the
// nonzero infinitesimal flag, regardless of its sign, as in the source.
func (a *LARRowBoundAnalyzer) endpoint(j uint32, positive, maximum bool) (*numeral.Rational, bool, error) {
	upper := positive == maximum
	if sourceDebugEnabled {
		t, err := a.propagator.ColumnType(j)
		if err != nil {
			return nil, false, err
		}
		available := t == ColumnFixed || t == ColumnBoxed || upper && t == ColumnUpperBound || !upper && t == ColumnLowBound
		if !available {
			return nil, false, sourceAssertionFailure()
		}
	}
	var b *RationalPair
	var err error
	if upper {
		b, err = a.propagator.UpperBound(j)
	} else {
		b, err = a.propagator.LowBound(j)
	}
	if err != nil {
		return nil, false, err
	}
	zero, err := a.manager.IsZero(&b.Y)
	return &b.X, !zero, err
}
func (a *LARRowBoundAnalyzer) clear(value *numeral.Rational) error {
	if err := a.manager.Del(value); err != nil {
		return err
	}
	*value = numeral.NewRational()
	return nil
}
func (a *LARRowBoundAnalyzer) extremum(j uint32, coefficient, temporary, out *numeral.Rational, maximum bool) (bool, error) {
	sign, err := a.manager.Sign(coefficient)
	if err != nil {
		return false, err
	}
	// Source min uses is_neg rather than !is_pos for the multiplied overload.
	positive := sign > 0
	if !maximum {
		positive = sign >= 0
	}
	endpoint, strict, err := a.endpoint(j, positive, maximum)
	if err != nil {
		return false, err
	}
	return strict, etaRationalProduct(a.manager, temporary, out, coefficient, endpoint)
}
func (a *LARRowBoundAnalyzer) finishPhase() error {
	if err := a.phase.Close(); err != nil {
		return err
	}
	a.phase = nil
	return nil
}
func (a *LARRowBoundAnalyzer) all(maximum bool) error {
	a.phase = NewIndexedRationalVector(a.manager, 6)
	v := a.phase.Data
	total, coeff, tmp, product, quotient, bound := &v[0], &v[1], &v[2], &v[3], &v[4], &v[5]
	strictCount := int32(0)
	if sourceDebugEnabled {
		zero, err := a.manager.IsZero(total)
		if err != nil {
			return err
		}
		if !zero {
			return sourceAssertionFailure()
		}
	}
	if err := a.reset(); err != nil {
		return err
	}
	for {
		j, ok, err := a.next(coeff)
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		strict, err := a.extremum(j, coeff, tmp, product, maximum)
		if err != nil {
			return err
		}
		if err = a.manager.Sub(total, product, total); err != nil {
			return err
		}
		if err = a.clear(product); err != nil {
			return err
		}
		if strict {
			strictCount++
		}
	}
	if err := a.reset(); err != nil {
		return err
	}
	for {
		j, ok, err := a.next(coeff)
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		positive, err := a.manager.IsPos(coeff)
		if err != nil {
			return err
		}
		// Preserve the quotient before evaluating the source addition operand.
		if err = etaRationalQuotient(a.manager, tmp, quotient, total, coeff); err != nil {
			return err
		}
		endpoint, strict, err := a.endpoint(j, positive, maximum)
		if err != nil {
			return err
		}
		if err = a.manager.Copy(tmp, quotient); err != nil {
			return err
		}
		if err = a.manager.Add(tmp, endpoint, tmp); err != nil {
			return err
		}
		if err = a.manager.Copy(bound, tmp); err != nil {
			return err
		}
		if err = a.clear(tmp); err != nil {
			return err
		}
		if err = a.clear(quotient); err != nil {
			return err
		}
		otherStrict := strictCount
		if strict {
			otherStrict--
		}
		if err = a.limit(j, bound, positive, positive == maximum, otherStrict > 0); err != nil {
			return err
		}
		if err = a.clear(bound); err != nil {
			return err
		}
	}
	return a.finishPhase()
}
func (a *LARRowBoundAnalyzer) unique(column uint32, maximum bool) error {
	a.phase = NewIndexedRationalVector(a.manager, 6)
	v := a.phase.Data
	chosen, coeff, tmp, product, bound := &v[0], &v[1], &v[2], &v[3], &v[5]
	if err := a.manager.Copy(bound, &a.RightSide.X); err != nil {
		return err
	}
	if err := a.manager.Neg(bound); err != nil {
		return err
	}
	if err := a.reset(); err != nil {
		return err
	}
	strict := false
	for {
		j, ok, err := a.next(coeff)
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		if j == column {
			if err = a.manager.Copy(chosen, coeff); err != nil {
				return err
			}
			continue
		}
		str, err := a.extremum(j, coeff, tmp, product, maximum)
		if err != nil {
			return err
		}
		if err = a.manager.Sub(bound, product, bound); err != nil {
			return err
		}
		if err = a.clear(product); err != nil {
			return err
		}
		strict = strict || str
	}
	if err := a.manager.Div(bound, chosen, bound); err != nil {
		return err
	}
	var positive bool
	var err error
	if maximum {
		// Source numeric_traits<impq>::is_pos(u_coeff) implicitly constructs
		// a pair from the scalar: copy X, initialize Y=0, test, destroy the
		// full-expression temporary before entering limit_j.
		condition := NewRationalPair()
		a.condition = &condition
		if err = a.manager.Copy(&condition.X, chosen); err != nil {
			return err
		}
		positive, err = a.manager.IsPos(&condition.X)
		if err != nil {
			return err
		}
		if !positive {
			var zero bool
			zero, err = a.manager.IsZero(&condition.X)
			if err != nil {
				return err
			}
			if zero {
				positive, err = a.manager.IsPos(&condition.Y)
				if err != nil {
					return err
				}
			}
		}
		if err = condition.Del(a.manager); err != nil {
			return err
		}
		a.condition = nil
	} else {
		positive, err = a.manager.IsPos(chosen)
		if err != nil {
			return err
		}
	}
	if err = a.limit(column, bound, positive, positive == maximum, strict); err != nil {
		return err
	}
	return a.finishPhase()
}

func (a *LARRowBoundAnalyzer) Analyze() (err error) {
	if a.stopped || a.work != nil || a.phase != nil || a.condition != nil || a.calls.pending() {
		return ErrExactVectorPending
	}
	if err = a.resume(); err != nil {
		return err
	}
	defer func() {
		if a.tracking {
			a.tracking = false
			err = a.calls.after(a.manager, a.before, err)
		}
		if err == nil {
			err = a.work.Close()
			if err == nil {
				a.work = nil
			}
		}
		a.stopped = err != nil
	}()
	a.work = NewIndexedRationalVector(a.manager, 1)
	for a.ColumnOfL != -2 || a.ColumnOfU != -2 {
		j, ok, e := a.next(&a.work.Data[0])
		if e != nil {
			return e
		}
		if !ok {
			break
		}
		if err = a.classify(j, &a.work.Data[0]); err != nil {
			return err
		}
	}
	if a.ColumnOfU >= 0 {
		err = a.unique(uint32(a.ColumnOfU), true)
	} else if a.ColumnOfU == -1 {
		err = a.all(true)
	}
	if err != nil {
		return err
	}
	if a.ColumnOfL >= 0 {
		err = a.unique(uint32(a.ColumnOfL), false)
	} else if a.ColumnOfL == -1 {
		err = a.all(false)
	}
	return err
}
func (a *LARRowBoundAnalyzer) Close() error {
	if err := a.calls.cleanupRaw(a.manager); err != nil {
		return err
	}
	if a.condition != nil {
		if err := a.condition.Del(a.manager); err != nil {
			return err
		}
		a.condition = nil
	}
	if a.phase != nil {
		if err := a.phase.Close(); err != nil {
			return err
		}
		a.phase = nil
	}
	if a.work != nil {
		if err := a.work.Close(); err != nil {
			return err
		}
		a.work = nil
	}
	if err := a.RightSide.Del(a.manager); err != nil {
		return err
	}
	return a.calls.cleanupRoots(a.manager)
}

// AnalyzeLARRow is the source analyze_row entry point. The caller must Close
// the returned lexical owner on both success and error; partial arithmetic and
// publication remain inspectable until then. Iterator/propagator are borrowed.
func AnalyzeLARRow(it RationalLinearIterator, rhs *RationalPair, row uint32, bp *LARBoundPropagator) (*LARRowBoundAnalyzer, error) {
	a, err := NewLARRowBoundAnalyzer(it, rhs, row, bp)
	if err == nil {
		err = a.Analyze()
	}
	return a, err
}

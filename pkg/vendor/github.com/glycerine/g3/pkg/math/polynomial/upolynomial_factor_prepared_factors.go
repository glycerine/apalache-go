package polynomial

// Source: src/math/polynomial/upolynomial.cpp:33-54;
// src/math/polynomial/upolynomial_factorization.cpp:1062,1120-1139,1172-1175.

type upolyPreparedFactorCursor struct {
	stage       uint8
	row, column uint32
}

// Owning rescue of the exact constructed member never replays a source SET
// against a modulus whose failed Copy left an unreadable pointer codec.
func (c *upolyFactorSquareFreeScope) closePreparedFactors(fs *UPolynomialFactors, index int, source bool) (cause error) {
	q := &c.primePreparation
	if !q.matches() || c.service.scope != c || index < 0 || index > 1 || fs == nil || fs.self != fs || (index == 0 && (c.constructed != 4 || c.best != fs || fs.manager != q.related)) || (index == 1 && (c.constructed != 9 || c.lifted != fs || fs.manager != q.lift)) {
		return ErrUPolynomialOwner
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	if source {
		if q.sourceCause != nil {
			return q.sourceCause
		}
		q.sourceExecuting = true
		defer func() { q.sourceExecuting = false }()
		q.enterCall()
		defer q.leaveCall()
		previousActive := q.active
		q.active = true
		defer func() {
			if cause != nil && q.sourceCause == nil {
				q.sourceCause = cause
			}
			q.active = previousActive || cause != nil || q.pending()
		}()
		return fs.closeWithSourceContext(&q.factorSource[index], true, numericIntegerModularManager_primePreparationAllocationContext(fs.manager.modular, q))
	}
	q.ownExecuting = true
	defer func() { q.ownExecuting = false }()
	q.enterCall()
	defer q.leaveCall()
	defer func() {
		if cause != nil && q.ownCause == nil {
			q.ownCause = cause
			q.active = true
		}
	}()
	return fs.closeWithSourceContext(&q.factorOwn[index], false, numericIntegerModularManager_primePreparationAllocationContext(fs.manager.modular, q))
}

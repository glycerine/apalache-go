package polynomial

// Source: upolynomial.cpp:2409-2411,2453-2454; retained binary arithmetic.

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

func (q *upolySturmIsolationScope) context() integerAllocationInvocationContext {
	return integerAllocationInvocationContext{sturmIsolation: q, sturmIsolationInvocation: q.invocation}
}
func (c integerAllocationInvocationContext) sturmIsolationFrame(z *unumeral.IntegerManager) *upolySturmIsolationScope {
	q := c.sturmIsolation
	if q == nil || !q.entryValid() || q.owner.config.Integer != z || c.sturmIsolationInvocation == 0 || c.sturmIsolationInvocation != q.invocation {
		return nil
	}
	return q
}
func (q *upolySturmIsolationScope) NumericBound(executing bool) bool {
	return q.valid() && q.active && (!executing || q.entryValid())
}
func (q *upolySturmIsolationScope) NumericRoots(z *unumeral.IntegerManager, local *unumeral.NumericAllocationRoots) *unumeral.NumericAllocationRoots {
	if !q.NumericBound(true) || q.owner.config.Integer != z {
		return nil
	}
	if local != nil {
		return local
	}
	return &q.numeric
}
func (q *upolySturmIsolationScope) NumericRecord(z *unumeral.IntegerManager, _ string, r unumeral.NumericInvocationReceipt) {
	if q.NumericBound(true) && q.owner.config.Integer == z && r.Raw != nil {
		q.raw = append(q.raw, *r.Raw)
	}
}
func (q *upolySturmIsolationScope) numericPending() bool {
	z := q.owner.config.Integer
	if unumeral.NumericAllocationPending(&q.numeric) {
		return true
	}
	for i := range q.raw {
		if z.MatchesRawInvocation(&q.raw[i]) {
			return true
		}
	}
	return false
}
func (q *upolySturmIsolationScope) numericSourceCause() error {
	z := q.owner.config.Integer
	for i := len(q.raw) - 1; i >= 0; i-- {
		if e := z.RawInvocationSourceCause(&q.raw[i]); e != nil {
			return e
		}
	}
	return nil
}
func (q *upolySturmIsolationScope) cleanupNumeric(abort bool) error {
	z := q.owner.config.Integer
	if !abort {
		if e := q.numeric.Cleanup(z); e != nil {
			return e
		}
	}
	for i := len(q.raw) - 1; i >= 0; i-- {
		if e := z.CleanupRawInvocation(&q.raw[i], abort); e != nil {
			return e
		}
	}
	return nil
}

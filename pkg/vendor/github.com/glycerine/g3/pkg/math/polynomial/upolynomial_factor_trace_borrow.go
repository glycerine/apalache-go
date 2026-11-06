package polynomial

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Header, body, endl and footer callbacks borrow the actual enclosing owners.
// Accepted numerical child ownership remains with its separate receipt.
type upolyFactorTraceBorrow struct {
	owners    [4]*UPolynomialManager
	tags      *diagnostic.TraceTags
	executing bool
	previous  [4]*upolyFactorTraceBorrow
	bindings  [6]unumeral.NumericCallerBinding
	bound     uint8
}

func (q *upolyFactorTraceBorrow) NumericExecuting() bool { return q.executing }
func (q *upolyFactorTraceBorrow) callback() bool {
	return q.tags.OutputCallbackActive() || q.owners[0].config.Integer.RuntimeOwner().FailureTransportActive()
}
func (q *upolyFactorTraceBorrow) managers() [4]*UPolynomialManager {
	return q.owners
}
func (q *upolyFactorTraceBorrow) bind(c *upolyFactorSquareFreeScope, tags *diagnostic.TraceTags) error {
	return q.bindManagers([3]*UPolynomialManager{c.manager, c.zp, c.zpe}, tags)
}
func (q *upolyFactorTraceBorrow) bindManagers(owners [3]*UPolynomialManager, tags *diagnostic.TraceTags) error {
	return q.bindFamily([4]*UPolynomialManager{owners[0], owners[1], owners[2]}, tags)
}
func (q *upolyFactorTraceBorrow) bindFamily(owners [4]*UPolynomialManager, tags *diagnostic.TraceTags) error {
	if q.executing {
		return ErrUPolynomialCleanupPending
	}
	for i, m := range owners {
		if m != nil {
			for j := 0; j < i; j++ {
				if owners[j] == m {
					owners[i] = nil
					break
				}
			}
		}
	}
	*q = upolyFactorTraceBorrow{owners: owners, tags: tags, executing: true}
	for i, m := range q.managers() {
		if m != nil {
			q.previous[i] = m.factorTrace
			m.factorTrace = q
		}
	}
	z := owners[0].config.Integer
	if e := z.BindCaller(&q.bindings[0], q, 0); e != nil {
		q.unbind()
		return e
	}
	q.bound++
	if e := z.Natural.BindCaller(&q.bindings[1], q, 0); e != nil {
		q.unbind()
		return e
	}
	q.bound++
	for _, m := range q.managers() {
		if m == nil {
			continue
		}
		if e := m.modular.BindCaller(&q.bindings[q.bound], q, 0); e != nil {
			q.unbind()
			return e
		}
		q.bound++
	}
	return nil
}
func (q *upolyFactorTraceBorrow) unbind() {
	for q.bound != 0 {
		q.bound--
		if e := q.bindings[q.bound].Unbind(); e != nil {
			panic(e)
		}
	}
	for i, m := range q.managers() {
		if m != nil {
			if m.factorTrace != q {
				panic(ErrUPolynomialOwner)
			}
			m.factorTrace = q.previous[i]
			q.previous[i] = nil
		}
	}
	q.executing = false
	q.tags = nil
}

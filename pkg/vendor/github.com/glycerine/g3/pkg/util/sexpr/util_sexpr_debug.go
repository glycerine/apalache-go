//go:build g3debug

package sexpr

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

var ErrSexprBVInvariant = errors.New("g3: source BV remainder diagnostic invariant")

// Source SASSERT(rational(0) <= c && c < rational(base)): construct/destroy
// temporaries through the whole && expression and short-circuit the second test.
func (m *SexprManager) sexprBVDiagnostic(c *unumeral.OwnedRational, base int64) error {
	q := m.Config.Rationals
	zero, e := m.scalar(0)
	if e != nil {
		return e
	}
	defer q.Del(&zero) // RHS temporary is destroyed before this LHS temporary.
	lower, e := q.LessEqual(&zero, c)
	if e != nil {
		return e
	}
	if !lower {
		return ErrSexprBVInvariant
	}
	limit, e := m.scalar(base)
	if e != nil {
		return e
	}
	upper, e := q.Less(c, &limit)
	q.Del(&limit)
	if e != nil {
		return e
	}
	if !upper {
		return ErrSexprBVInvariant
	}
	return nil
}

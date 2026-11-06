package rel

// Source: src/muz/rel/dl_sieve_relation.cpp sieve_relation::display.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// Display writes the source sieve header followed by the concrete inner
// interval relation. Ignored outer columns are represented only in the mask;
// they are not materialized for diagnostics.
func (r *IntervalSieve) Display(out io.Writer) error {
	if err := r.open(); err != nil || out == nil {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	if _, err := io.WriteString(out, "Sieve relation ("); err != nil {
		return err
	}
	for i, selected := range r.innerCols {
		if i != 0 {
			if _, err := io.WriteString(out, ","); err != nil {
				return err
			}
		}
		value := 0
		if selected {
			value = 1
		}
		if _, err := fmt.Fprint(out, value); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(out, ")\n"); err != nil {
		return err
	}
	return r.inner.Display(out)
}

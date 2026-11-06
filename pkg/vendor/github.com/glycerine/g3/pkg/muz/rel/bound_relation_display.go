package rel

// Source: src/muz/rel/dl_bound_relation.cpp display_index and
// src/muz/rel/dl_vector_relation.h display.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// Display writes the source bound-relation diagnostic form.
func (r *BoundRelation) Display(out io.Writer) error {
	if r == nil || out == nil {
		return ast.ErrASTDomain
	}
	if r.empty {
		_, err := fmt.Fprintln(out, "empty")
		return err
	}
	for i := range r.columns {
		index := uint32(i)
		root := r.find(index)
		if root != index {
			if _, err := fmt.Fprintf(out, "%d = %d ", index, root); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(out, "#%d", index); err != nil {
			return err
		}
		strict, weak, err := r.Bounds(index)
		if err != nil {
			return err
		}
		if len(strict) != 0 {
			if _, err = io.WriteString(out, " < "); err != nil {
				return err
			}
			for _, column := range strict {
				if _, err = fmt.Fprintf(out, "%d ", column); err != nil {
					return err
				}
			}
		}
		if len(weak) != 0 {
			if _, err = io.WriteString(out, " <= "); err != nil {
				return err
			}
			for _, column := range weak {
				if _, err = fmt.Fprintf(out, "%d ", column); err != nil {
					return err
				}
			}
		}
		if len(strict) == 0 && len(weak) == 0 {
			if _, err = io.WriteString(out, " < oo"); err != nil {
				return err
			}
		}
		if _, err = io.WriteString(out, "\n"); err != nil {
			return err
		}
	}
	_, err := io.WriteString(out, "\n")
	return err
}

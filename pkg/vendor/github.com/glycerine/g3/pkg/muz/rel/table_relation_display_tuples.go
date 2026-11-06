package rel

// Source: src/muz/rel/dl_table_relation.cpp table_relation::display_tuples.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// DisplayTuples prints each row with the Datalog context's source argument
// names and finite-domain constant spellings.
func (r *TableRelation) DisplayTuples(context *base.DatalogContext, predicate ast.ASTHandle, out io.Writer) error {
	if err := r.open(); err != nil {
		return err
	}
	if context == nil || out == nil {
		return ErrTableRelation
	}
	manager, err := context.Manager()
	if err != nil {
		return err
	}
	if manager != r.manager.manager {
		return ErrTableRelation
	}
	sorts, _, err := r.manager.PredicateSignature(predicate)
	if err != nil {
		return err
	}
	if len(sorts) != len(r.sorts) {
		return ErrTableRelation
	}
	for index := range sorts {
		if sorts[index] != r.sorts[index] {
			return ErrTableRelation
		}
	}
	name, err := manager.GetASTName(predicate)
	if err != nil {
		return err
	}
	nameText, err := manager.Symbols.Display(name)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(out, "Tuples in %s: \n", nameText); err != nil {
		return err
	}
	facts, err := r.table.Facts()
	if err != nil {
		return err
	}
	for _, fact := range facts {
		if len(fact) != len(sorts) {
			return ErrTableRelation
		}
		if _, err = io.WriteString(out, "\t("); err != nil {
			return err
		}
		for index, value := range fact {
			if index != 0 {
				if _, err = io.WriteString(out, ","); err != nil {
					return err
				}
			}
			argument, nameErr := context.ArgumentName(predicate, uint32(index))
			if nameErr != nil {
				return nameErr
			}
			argumentText, displayErr := manager.Symbols.Display(argument)
			if displayErr != nil {
				return displayErr
			}
			if _, err = fmt.Fprintf(out, "%s=", argumentText); err != nil {
				return err
			}
			if err = context.WriteConstant(out, sorts[index], uint64(value)); err != nil {
				return err
			}
			if _, err = fmt.Fprintf(out, "(%d)", value); err != nil {
				return err
			}
		}
		if _, err = io.WriteString(out, ")\n"); err != nil {
			return err
		}
	}
	return nil
}

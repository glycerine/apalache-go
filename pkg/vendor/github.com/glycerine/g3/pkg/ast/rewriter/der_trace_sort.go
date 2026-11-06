package rewriter

// Source: src/ast/rewriter/der.cpp:344-359.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (d *DER) traceDefinitions(definitions []ast.ASTHandle) error {
	return d.emitDERTrace("top_sort", "get_elimination_order", 344, func(w io.Writer, globals *uparams.GlobalParamsService) error {
		if _, err := io.WriteString(w, "DEFINITIONS: \n"); err != nil {
			return err
		}
		for i, definition := range definitions {
			if definition == (ast.ASTHandle{}) {
				continue
			}
			if _, err := io.WriteString(w, "VAR "+strconv.Itoa(i)+" = "); err != nil {
				return err
			}
			if err := d.writeDERASTLine(w, definition, globals); err != nil {
				return err
			}
		}
		return nil
	})
}

func (d *DER) traceEliminationOrder(order []uint32) error {
	return d.emitDERTrace("der", "get_elimination_order", 356, func(w io.Writer, _ *uparams.GlobalParamsService) error {
		if _, err := io.WriteString(w, "Elimination m_order:\n"); err != nil {
			return err
		}
		for i, index := range order {
			if i != 0 {
				if _, err := io.WriteString(w, " "); err != nil {
					return err
				}
			}
			if _, err := io.WriteString(w, strconv.FormatUint(uint64(index), 10)); err != nil {
				return err
			}
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}

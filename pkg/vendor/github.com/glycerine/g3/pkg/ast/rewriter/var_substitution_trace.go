package rewriter

// Source: src/ast/rewriter/var_subst.cpp:35-40; src/util/trace.h:40-46.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (s *VarSubstitution) traceSubstitution(term ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) error {
	m := s.reducer.engine.core.manager
	tags := m.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("var_subst_bug") {
		return nil
	}
	globals := uparams.BorrowProcessGlobalParamsService()
	if globals == nil || globals.Context == nil {
		return uparams.ErrGlobalParamsOwner
	}
	result, err := m.RefGet(out)
	if err != nil {
		return err
	}
	err = tags.EmitConditional("var_subst_bug", "operator()", "src/ast/rewriter/var_subst.cpp", 35,
		func() bool { return true }, func(w io.Writer) error {
			order := "0"
			if s.standardOrder {
				order = "1"
			}
			if _, err := io.WriteString(w, "m_std_order: "+order+"\n"); err != nil {
				return err
			}
			if err := writeBVTraceSMT2(m, w, term, globals); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "\nusing\n"); err != nil {
				return err
			}
			for i := uint32(0); i < args.Count(); i++ {
				h, err := args.At(i)
				if err != nil {
					return err
				}
				if err = writeBVTraceSMT2(m, w, h, globals); err != nil {
					return err
				}
				if _, err = io.WriteString(w, "\n"); err != nil {
					return err
				}
			}
			if _, err := io.WriteString(w, "\n------>\n"); err != nil {
				return err
			}
			if err := writeBVTraceSMT2(m, w, result, globals); err != nil {
				return err
			}
			_, err := io.WriteString(w, "\n")
			return err
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

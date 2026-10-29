package model

// Source: src/model/func_interp.cpp:143-149; src/util/trace.h:40-46.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (f *FunctionInterpretation) tracePeeledEntry(term ast.ASTHandle) error {
	tags := f.manager.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("func_interp") {
		return nil
	}
	return tags.EmitConditional("func_interp", "set_else", "src/model/func_interp.cpp", 146,
		func() bool { return true }, func(w io.Writer) error {
			if _, err := io.WriteString(w, "fi entry expr: "); err != nil {
				return err
			}
			if err := f.writeTraceSMT2(w, term); err != nil {
				return err
			}
			_, err := io.WriteString(w, "\n")
			return err
		})
}

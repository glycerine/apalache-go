package smtcore

// Source: src/smt/theory_dl.cpp:114,197-198,205,244-248.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func datalogAssert(line int32, condition string, evaluate func() bool) error {
	if !diagnostic.DebugBuild {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !enabled {
		return err
	}
	if !evaluate() {
		return diagnostic.DebugAssertionFailure(d, "src/smt/theory_dl.cpp", line, condition)
	}
	return nil
}

func (*SMTDatalogTheory) Name() string { return "datalog" }

// Source display has an empty body and never touches its stream.
func (*SMTDatalogTheory) Display(io.Writer) error { return nil }

func (t *SMTDatalogTheory) datalogMaximum(sort ast.ASTHandle) (ast.ASTHandle, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	size, err := state.manager.GetSortSize(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !size.IsFinite() {
		// VERIFY evaluates in every build and ignores the assertion switch.
		// Continue leaves source sz uninitialized: stop before that physical read.
		err = diagnostic.BorrowProcessDebugContext().VerifyFailure("src/smt/theory_dl.cpp", 246, "u().try_get_size(s, sz)")
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, err
	}
	count, err := size.Size()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = datalogAssert(247, "sz > 0", func() bool { return count > 0 }); err != nil {
		return ast.ASTHandle{}, err
	}
	return state.manager.BitVectorNumeralUint64(count-1, 64)
}

package smtcore

// Source: src/smt/theory_str.cpp:12422-12424 theory_str::display.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) Display(out io.Writer) error {
	if t == nil || t.SMTTheoryBase == nil || out == nil {
		return ast.ErrASTHandle
	}
	if _, err := t.State(); err != nil {
		return err
	}
	// This literal TODO is the complete retained C++ display implementation.
	if err := smtWriteTheoryText(out, "TODO: theory_str display"); err != nil {
		return err
	}
	return smtTheoryDisplayEndl(out)
}

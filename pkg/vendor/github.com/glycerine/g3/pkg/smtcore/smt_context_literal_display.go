package smtcore

// Source: src/smt/smt_context_pp.cpp:99-105 and smt_context.h:1242.

import (
	"github.com/glycerine/g3/pkg/ast"
	"io"
)

func (s *SMTInternalizerState) DisplayLiteralVerbose(out io.Writer, literal SMTLiteral) error {
	return s.DisplayLiteralsVerbose(out, []SMTLiteral{literal})
}

func (s *SMTInternalizerState) DisplayLiteralsVerbose(out io.Writer, literals []SMTLiteral) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	return DisplaySMTLiteralsVerbose(out, s.manager, literals, s.boolExpr, "\n")
}

func (s *SMTInternalizerState) DisplayDetailedLiteral(out io.Writer, literal SMTLiteral) error {
	if err := s.open(); err != nil {
		return err
	}
	return literal.Display(out, s.manager, s.boolExpr)
}

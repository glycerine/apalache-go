package smtcore

// Source: src/smt/theory_fpa.cpp:716-720.

import (
	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTFloatingPointTheory) FinalCheck() (SMTFinalCheckStatus, error) {
	if t == nil || t.converter == nil {
		return SMTFinalCheckContinue, ast.ErrASTHandle
	}
	if err := t.traceFPAFinalCheck(); err != nil {
		return SMTFinalCheckContinue, err
	}
	if err := fpaSourceAssertBool(718, "m_converter.m_extra_assertions.empty()", len(t.converter.ExtraAssertions()) == 0); err != nil {
		return SMTFinalCheckContinue, err
	}
	return SMTFinalCheckDone, nil
}

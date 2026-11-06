package sls

// Source: src/tactic/sls/sls_tracker.h:530-536 and
// src/tactic/sls/sls_evaluator.h:798-823.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// DisplayModel emits constants in the source tracker's insertion order.
func (t *Tracker) DisplayModel(out io.Writer) error {
	if err := t.open(); err != nil {
		return err
	}
	if out == nil {
		return ErrTracker
	}
	for _, decl := range t.constants {
		name, err := t.manager.GetASTName(decl)
		if err != nil {
			return err
		}
		text, err := t.manager.Symbols.Display(name)
		if err != nil {
			return err
		}
		value, err := t.DeclValue(decl)
		if err != nil {
			return err
		}
		number, err := t.numbers.String(value)
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintf(out, "%s = %s\n", text, number); err != nil {
			return err
		}
	}
	return nil
}

// RandomizeExpression is the source expr* overload. It randomizes one
// declaration occurring below root.
func (e *Evaluator) RandomizeExpression(search *SearchTracker, root ast.ASTHandle) error {
	if err := e.open(); err != nil {
		return err
	}
	if search == nil || search.base != e.tracker {
		return ErrSLSSearch
	}
	declarations, err := search.ConstantsForRoot(root)
	if err != nil {
		return err
	}
	return e.RandomizeLocal(search, declarations)
}

// RandomizeAssertions is the source assertion-vector overload. Selection uses
// the current WalkSAT/GSAT policy before randomizing one candidate.
func (e *Evaluator) RandomizeAssertions(search *SearchTracker, assertions []ast.ASTHandle) error {
	if err := e.open(); err != nil {
		return err
	}
	if search == nil || search.base != e.tracker {
		return ErrSLSSearch
	}
	declarations, err := search.UnsatConstants(assertions)
	if err != nil {
		return err
	}
	return e.RandomizeLocal(search, declarations)
}

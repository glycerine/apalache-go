package smtcore

// Source: src/smt/smt_context.cpp get_assignments,
// get_relevant_labeled_literals, get_relevant_literals, and
// get_guessed_literals.

import (
	"errors"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (k *SMTKernel) ownLiteral(literal SMTLiteral) (ast.ASTHandle, error) {
	if literal.Var() < SMTFirstBoolVar || int(literal.Var()) >= len(k.internalizer.boolExpr) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	expression := k.internalizer.boolExpr[literal.Var()]
	var err error
	if literal.Sign() {
		expression, err = k.manager.MkNot(expression)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	if err = k.manager.IncRef(expression); err != nil {
		return ast.ASTHandle{}, err
	}
	return expression, nil
}

func (k *SMTKernel) clearLiteralResults() error {
	var result error
	for _, values := range [][]ast.ASTHandle{k.assignments, k.relevantLiterals, k.relevantLabeled, k.guessed} {
		for _, expression := range values {
			result = errors.Join(result, k.manager.DecRef(expression))
		}
	}
	if result == nil {
		k.assignments, k.relevantLiterals, k.relevantLabeled, k.guessed = nil, nil, nil, nil
	}
	return result
}

func (k *SMTKernel) captureLiteralResults(baseLevel uint32) error {
	if err := k.clearLiteralResults(); err != nil {
		return err
	}
	fail := func(cause error) error { return errors.Join(cause, k.clearLiteralResults()) }
	for _, literal := range k.internalizer.assignedLiterals {
		expression, err := k.ownLiteral(literal)
		if err != nil {
			return fail(err)
		}
		k.assignments = append(k.assignments, expression)
		owner := k.internalizer.boolExpr[literal.Var()]
		if !k.relevancy.IsRelevant(owner) {
			continue
		}
		relevant, err := k.ownLiteral(literal)
		if err != nil {
			return fail(err)
		}
		k.relevantLiterals = append(k.relevantLiterals, relevant)
	}
	for level := int(baseLevel); level < len(k.internalizer.scopes); level++ {
		index := k.internalizer.scopes[level].assignedLimit
		if index < 0 || index >= len(k.internalizer.assignedLiterals) {
			continue
		}
		literal := k.internalizer.assignedLiterals[index]
		if k.internalizer.booleanData[literal.Var()].Justification().Kind() != SMTBooleanJustificationAxiom {
			continue
		}
		expression, err := k.ownLiteral(literal)
		if err != nil {
			return fail(err)
		}
		k.guessed = append(k.guessed, expression)
	}
	for variable := int32(SMTFirstBoolVar); int(variable) < len(k.internalizer.boolExpr); variable++ {
		expression := k.internalizer.boolExpr[variable]
		if !k.relevancy.IsRelevant(expression) {
			continue
		}
		value, err := k.internalizer.Assignment(NewSMTLiteral(variable, false))
		if err != nil || value != lbool.True {
			if err != nil {
				return fail(err)
			}
			continue
		}
		var names []usymbol.Symbol
		label, err := k.manager.IsLabelLit(expression, &names)
		if err != nil {
			return fail(err)
		}
		if label {
			if err = k.manager.IncRef(expression); err != nil {
				return fail(err)
			}
			k.relevantLabeled = append(k.relevantLabeled, expression)
		}
	}
	return nil
}

func (k *SMTKernel) Assignments() ([]ast.ASTHandle, error) {
	if err := k.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), k.assignments...), nil
}

func (k *SMTKernel) RelevantLiterals() ([]ast.ASTHandle, error) {
	if err := k.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), k.relevantLiterals...), nil
}

func (k *SMTKernel) RelevantLabeledLiterals(atLabels bool) ([]ast.ASTHandle, error) {
	if err := k.open(); err != nil {
		return nil, err
	}
	if !atLabels {
		return append([]ast.ASTHandle(nil), k.relevantLabeled...), nil
	}
	result := make([]ast.ASTHandle, 0, len(k.relevantLabeled))
	for _, expression := range k.relevantLabeled {
		var names []usymbol.Symbol
		label, err := k.manager.IsLabelLit(expression, &names)
		if err != nil {
			return nil, err
		}
		include := false
		for _, name := range names {
			spelling, err := k.manager.Symbols.String(name)
			if err != nil {
				return nil, err
			}
			if strings.Contains(spelling, "@") {
				include = true
				break
			}
		}
		if label && include {
			result = append(result, expression)
		}
	}
	return result, nil
}

func (k *SMTKernel) GuessedLiterals() ([]ast.ASTHandle, error) {
	if err := k.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), k.guessed...), nil
}

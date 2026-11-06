package smtcore

// Source: src/smt/theory_arith_aux.h:1120-1143 mk_ge, instantiated for mi_ext and inf_ext.

import (
	"bytes"
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/tactic"
)

// MakeObjectiveLowerBound creates the native Boolean atom for value <= v.
// Its full infinitesimal threshold lives in the theory, not an SMT arithmetic
// comparison. The returned AST is borrowed from the Boolean internalizer.
func (t *SMTLinearArithmeticTheory) MakeObjectiveLowerBound(filter *tactic.FilterModelConverter, variable SMTTheoryVar, value simplex.Infinitesimal) (result ast.ASTHandle, resultErr error) {
	s, err := t.State()
	if err != nil {
		return result, err
	}
	if value.Infinity.Sign() != 0 && !t.infinite {
		return result, ast.ErrASTDomain
	}
	if filter == nil {
		return result, ast.ErrASTHandle
	}
	node, err := t.Enode(variable)
	if err != nil {
		return result, err
	}
	var name bytes.Buffer
	name.WriteString(smtArithmeticBoundName(value))
	name.WriteString(" <= ")
	printer, err := ast.NewSimpleASTPrinterContext(s.manager, &name, &name)
	if err != nil {
		return result, err
	}
	if err = printer.DisplayExpr(&name, node.owner, 0); err != nil {
		return result, err
	}
	symbol, err := s.manager.Symbols.Intern(name.String())
	if err != nil {
		return result, err
	}
	boolean, err := s.manager.MkBoolSort()
	if err != nil {
		return result, err
	}
	result, err = s.manager.MkConst(symbol, boolean)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if _, found := s.BoolVar(result); found {
		return result, nil // Source returns an already-internalized name unchanged.
	}
	root, err := s.manager.NewRef(result)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, s.manager.DelRef(&root)) }()
	decl, err := s.manager.GetDecl(result)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = filter.Insert(decl); err != nil {
		return ast.ASTHandle{}, err
	}
	bv, err := s.MkBoolVar(result)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = s.SetVariableTheory(bv, t.FamilyID()); err != nil {
		return ast.ASTHandle{}, err
	}
	threshold := newSMTArithmeticValue(&value.Main, &value.Epsilon)
	threshold.infinity.Set(&value.Infinity)
	native, err := newSMTArithmeticAtom(bv, variable, &threshold, smtArithmeticLowerAtom)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	form, linear, err := t.linearize(node.owner)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !linear {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	atom := smtLinearArithmeticAtom{native: native, variable: bv, left: node.owner, form: form, op: smtArithOpGE, supported: true}
	atom.nativeOffset.Set(&form.constant)
	atom.form.constant.Sub(&atom.form.constant, &value.Main)
	atom.boundAxiom = &smtArithmeticBoundAxiomAtom{axis: node.owner, literal: NewSMTLiteral(bv, false), kind: smtArithmeticLowerAtom, integral: t.integer || t.integerColumns[t.simplexVariables[variable]]}
	atom.boundAxiom.threshold.set(&threshold)
	index := len(t.atoms)
	t.atomByBool[bv] = index
	t.atoms = append(t.atoms, atom)
	// addBoundAxioms excludes index, preserving native pre-insertion neighbor
	// lookup. Retaining the registered atom also permits Go allocation-error
	// recovery without leaving a theory-owned Boolean with no numeric bound.
	if err = t.addBoundAxioms(index, atom.boundAxiom); err != nil {
		t.newBoundAtoms = append(t.newBoundAtoms, index)
		return ast.ASTHandle{}, err
	}
	return result, nil
}

// Source inf_eps_rational<inf_rational>::to_string and inf_rational::to_string.
func smtArithmeticBoundName(value simplex.Infinitesimal) string {
	finite := value.Main.RatString()
	if value.Epsilon.Sign() != 0 {
		sign := " +e*"
		if value.Epsilon.Sign() < 0 {
			sign = " -e*"
		}
		finite = "(" + finite + sign + new(big.Rat).Abs(&value.Epsilon).RatString() + ")"
	}
	if value.Infinity.Sign() == 0 {
		return finite
	}
	infinity := value.Infinity.RatString() + "*oo"
	if value.Infinity.Cmp(big.NewRat(1, 1)) == 0 {
		infinity = "oo"
	} else if value.Infinity.Cmp(big.NewRat(-1, 1)) == 0 {
		infinity = "-oo"
	}
	if value.Main.Sign() == 0 && value.Epsilon.Sign() == 0 {
		return infinity
	}
	return "(" + infinity + " + " + finite + ")"
}

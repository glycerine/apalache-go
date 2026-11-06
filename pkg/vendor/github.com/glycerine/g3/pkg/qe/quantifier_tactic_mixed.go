package qe

// Source: src/qe/qe.cpp, same-polarity variable block elimination.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// eliminateQuantifierMixed separates array, datatype, Boolean, arithmetic,
// bit-vector, and finite-domain declarations of one quantifier. Every group must be fully
// eliminated by its concrete solver; unsupported declarations report incomplete.
func eliminateQuantifierMixed(m *ast.ASTManager, quantifier ast.ASTHandle, config Config) (result ast.ASTRef, complete bool, err error) {
	if m == nil || config.Cooperation == nil {
		return result, false, ast.ErrASTHandle
	}
	defer func() {
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
		}
	}()
	kind, err := m.GetKind(quantifier)
	if err != nil || kind != ast.ASTQuantifier {
		return result, false, err
	}
	free, err := rewriter.NewExprFreeVars(m)
	if err != nil {
		return result, false, err
	}
	if err = free.Collect(quantifier); err != nil || !free.Empty() {
		return result, false, err
	}
	body, err := m.GetQuantifierBody(quantifier)
	if err != nil {
		return result, false, err
	}
	nested, err := m.HasQuantifiers(body)
	if err != nil || nested {
		return result, false, err
	}
	extracted, err := extractExprQuantifier(m, quantifier)
	if err != nil {
		return result, false, closeFailedExprQuantifierExtract(extracted, err)
	}
	defer func() { err = errors.Join(err, extracted.Close()) }()
	boolean, err := m.MkBoolSort()
	if err != nil {
		return result, false, err
	}
	arithmetic, err := m.ArithmeticFamilyID()
	if err != nil {
		return result, false, err
	}
	array, err := m.RegisterArrayPlugin()
	if err != nil {
		return result, false, err
	}
	datatype, err := m.RegisterDatatypePlugin()
	if err != nil {
		return result, false, err
	}
	datalog, err := m.RegisterDatalogPlugin()
	if err != nil {
		return result, false, err
	}
	var arrayVars, datatypeVars, boolVars, arithmeticVars, bitVectorVars, datalogVars, unsupportedVars []ast.ASTHandle
	for _, ref := range extracted.variables {
		variable, e := m.RefGet(ref)
		if e != nil {
			return result, false, e
		}
		sort, e := m.GetSort(variable)
		if e != nil {
			return result, false, e
		}
		if sort == boolean {
			boolVars = append(boolVars, variable)
			continue
		}
		isBitVector, e := m.IsBitVectorSort(sort)
		if e != nil {
			return result, false, e
		}
		if isBitVector {
			bitVectorVars = append(bitVectorVars, variable)
			continue
		}
		family, e := m.GetFamilyID(sort)
		if e != nil {
			return result, false, e
		}
		switch family {
		case array:
			arrayVars = append(arrayVars, variable)
		case datatype:
			datatypeVars = append(datatypeVars, variable)
		case arithmetic:
			arithmeticVars = append(arithmeticVars, variable)
		case datalog:
			datalogVars = append(datalogVars, variable)
		default:
			// quant_elim_plugin::check leaves declarations without a theory
			// plugin in m_free_vars while continuing with supported variables.
			unsupportedVars = append(unsupportedVars, variable)
		}
	}
	groups := [][]ast.ASTHandle{arrayVars, datatypeVars, boolVars, arithmeticVars, bitVectorVars, datalogVars}
	nonempty := 0
	for _, group := range groups {
		if len(group) > 0 {
			nonempty++
		}
	}
	if nonempty == 0 || (nonempty < 2 && len(unsupportedVars) == 0) {
		return result, false, nil
	}
	body, err = m.RefGet(extracted.body)
	if err != nil {
		return result, false, err
	}
	forall, err := m.IsForall(quantifier)
	if err != nil {
		return result, false, err
	}
	if forall {
		body, err = m.MkNot(body)
		if err != nil {
			return result, false, err
		}
	}
	formula, err := m.NewRef(body)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&formula)) }()
	for i, group := range groups {
		body, e := m.RefGet(formula)
		if e != nil {
			return result, false, e
		}
		bound, e := bindExprQuantifierVariables(m, group, body)
		if e != nil {
			return result, false, e
		}
		q, e := m.RefGet(bound)
		if e != nil {
			return result, false, errors.Join(e, m.DelRef(&bound))
		}
		kind, e := m.GetKind(q)
		if e != nil {
			return result, false, errors.Join(e, m.DelRef(&bound))
		}
		if kind != ast.ASTQuantifier {
			if e = m.DelRef(&bound); e != nil {
				return result, false, e
			}
			continue
		}
		var eliminated ast.ASTRef
		var done bool
		switch i {
		case 0:
			eliminated, done, e = eliminateQuantifierArrayEquations(m, q, config)
		case 1:
			eliminated, done, e = eliminateQuantifierDatatypeBlock(m, q, config)
		case 2:
			eliminated, done, e = eliminateQuantifierBoolUnits(m, q, config)
		case 3:
			if !config.nonlinearQuantifierElimination {
				eliminated, done, e = eliminateQuantifierArithmeticEquations(m, q, config)
				if e == nil && !done {
					eliminated, done, e = eliminateQuantifierArithmeticClosed(m, q, config)
				}
			}
			if e == nil && !done {
				eliminated, done, e = eliminateQuantifierArithmeticSearch(m, q, config)
			}
		case 4:
			eliminated, done, e = eliminateQuantifierBitVectorBlock(m, q, config)
		case 5:
			eliminated, done, e = eliminateQuantifierDatalogBlock(m, q, config)
		}
		if e != nil || !done {
			return result, false, errors.Join(e, m.DelRef(&bound))
		}
		value, e := m.RefGet(eliminated)
		if e == nil {
			e = m.SetRef(formula, value)
		}
		e = errors.Join(e, m.DelRef(&eliminated), m.DelRef(&bound))
		if e != nil {
			return result, false, e
		}
	}
	body, err = m.RefGet(formula)
	if err != nil {
		return result, false, err
	}
	if len(unsupportedVars) > 0 {
		bound, e := bindExprQuantifierVariables(m, unsupportedVars, body)
		if e != nil {
			return result, false, e
		}
		body, e = m.RefGet(bound)
		err = errors.Join(e, m.DelRef(&bound))
		if err != nil {
			return result, false, err
		}
	}
	if forall {
		body, err = m.MkNotSimplified(body)
		if err != nil {
			return result, false, err
		}
	}
	result, err = m.NewRef(body)
	return result, err == nil, err
}

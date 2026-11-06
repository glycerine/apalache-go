package qe

// Source: src/qe/qe.cpp, elimination of variables introduced by plugins.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// projectArrayAuxiliary eliminates one scalar variable introduced while
// solving array equations. Other variables remain as free constants, so the
// scalar projection can be performed before continuing with the array plugin.
func projectArrayAuxiliary(m *ast.ASTManager, context *SolverContext, formula ast.ASTRef, config Config) (changed bool, err error) {
	boolean, err := m.MkBoolSort()
	if err != nil {
		return false, err
	}
	arithmetic, err := m.ArithmeticFamilyID()
	if err != nil {
		return false, err
	}
	datatype, err := m.RegisterDatatypePlugin()
	if err != nil {
		return false, err
	}
	for index, variable := range context.variables {
		sort, e := m.GetSort(variable)
		if e != nil {
			return false, e
		}
		family, e := m.GetFamilyID(sort)
		if e != nil {
			return false, e
		}
		bitVector, e := m.IsBitVectorSort(sort)
		if e != nil {
			return false, e
		}
		if sort != boolean && family != arithmetic && family != datatype && !bitVector {
			continue
		}
		body, e := m.RefGet(formula)
		if e != nil {
			return false, e
		}
		bound, e := bindExprQuantifierVariables(m, []ast.ASTHandle{variable}, body)
		if e != nil {
			return false, e
		}
		q, e := m.RefGet(bound)
		if e != nil {
			return false, errors.Join(e, m.DelRef(&bound))
		}
		var part ast.ASTRef
		var done bool
		switch {
		case sort == boolean:
			part, done, e = eliminateQuantifierBoolUnits(m, q, config)
		case family == arithmetic:
			part, done, e = eliminateQuantifierArithmeticEquations(m, q, config)
			if e == nil && !done {
				part, done, e = eliminateQuantifierArithmeticClosed(m, q, config)
			}
			if e == nil && !done {
				part, done, e = eliminateQuantifierArithmeticSearch(m, q, config)
			}
		case bitVector:
			part, done, e = eliminateQuantifierBitVectorBlock(m, q, config)
		case family == datatype:
			part, done, e = eliminateQuantifierDatatypeBlock(m, q, config)
		}
		e = errors.Join(e, m.DelRef(&bound))
		if e != nil || !done {
			if part != (ast.ASTRef{}) {
				e = errors.Join(e, m.DelRef(&part))
			}
			if e != nil {
				return false, e
			}
			continue
		}
		value, e := m.RefGet(part)
		if e == nil {
			e = context.EliminateVariable(uint32(index), value, ast.ASTHandle{})
		}
		if e == nil {
			e = m.SetRef(formula, value)
		}
		e = errors.Join(e, m.DelRef(&part))
		return e == nil, e
	}
	return false, nil
}

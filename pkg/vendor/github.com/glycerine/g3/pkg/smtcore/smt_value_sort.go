package smtcore

// Source: src/smt/smt_value_sort.h and src/smt/smt_value_sort.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// SMTIsValueSort reports whether every element of sort has a ground value
// representation supported by the SMT model construction path.
func SMTIsValueSort(m *ast.ASTManager, sort ast.ASTHandle) (bool, error) {
	if m == nil {
		return false, ast.ErrASTDomain
	}
	pending := []ast.ASTHandle{sort}
	marked := make(map[ast.ASTHandle]struct{})
	boolean, err := m.MkBoolSort()
	if err != nil {
		return false, err
	}
	arithmetic, err := m.ArithmeticFamilyID()
	if err != nil {
		return false, err
	}
	datatype, datatypeReady := m.DatatypeFamilyID()
	if datatypeReady != nil && !errors.Is(datatypeReady, ast.ErrASTDomain) {
		return false, datatypeReady
	}
	for len(pending) != 0 {
		last := len(pending) - 1
		current := pending[last]
		pending = pending[:last]
		if _, found := marked[current]; found {
			continue
		}
		kind, err := m.GetKind(current)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTSort {
			return false, ast.ErrASTDomain
		}
		marked[current] = struct{}{}
		if current == boolean {
			continue
		}
		family, err := m.GetFamilyID(current)
		if err != nil {
			return false, err
		}
		if family == arithmetic {
			sortKind, err := m.GetDeclKind(current)
			if err != nil {
				return false, err
			}
			if sortKind == 0 || sortKind == 1 {
				continue
			}
		}
		bitVector, err := m.IsBitVectorSort(current)
		if err != nil {
			return false, err
		}
		if bitVector {
			continue
		}
		if datatypeReady != nil || family != datatype {
			return false, nil
		}
		constructors, err := m.DatatypeConstructors(current)
		if err != nil {
			return false, err
		}
		for _, constructor := range constructors {
			arity, err := m.GetArity(constructor)
			if err != nil {
				return false, err
			}
			for i := uint32(0); i < arity; i++ {
				domain, err := m.GetDomain(constructor, i)
				if err != nil {
					return false, err
				}
				pending = append(pending, domain)
			}
		}
	}
	return true, nil
}

func SMTExprHasValueSort(m *ast.ASTManager, expression ast.ASTHandle) (bool, error) {
	if m == nil {
		return false, ast.ErrASTDomain
	}
	sort, err := m.GetSort(expression)
	if err != nil {
		return false, err
	}
	return SMTIsValueSort(m, sort)
}

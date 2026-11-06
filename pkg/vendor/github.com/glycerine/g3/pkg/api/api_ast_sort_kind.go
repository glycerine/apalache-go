package api

// Source: src/api/api_ast.cpp:605-654.

import "github.com/glycerine/g3/pkg/ast"

type APISortKind uint8

const (
	APISortUnknown APISortKind = iota
	APISortUninterpreted
	APISortBool
	APISortInt
	APISortReal
	APISortBitVector
	APISortArray
	APISortDatatype
	APISortRelation
	APISortFiniteDomain
	APISortFloatingPoint
	APISortRoundingMode
	APISortSequence
	APISortRegex
)

func (c *SMTContext) SortKind(sort ast.ASTHandle) (APISortKind, error) {
	m, err := c.Manager()
	if err != nil {
		return APISortUnknown, err
	}
	if kind, e := m.GetKind(sort); e != nil || kind != ast.ASTSort {
		if e != nil {
			return APISortUnknown, e
		}
		return APISortUnknown, ast.ErrASTDomain
	}
	if yes, e := m.IsUninterpretedSort(sort); e != nil {
		return APISortUnknown, e
	} else if yes {
		return APISortUninterpreted, nil
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return APISortUnknown, err
	}
	if sort == boolean {
		return APISortBool, nil
	}
	family, err := m.GetFamilyID(sort)
	if err != nil {
		return APISortUnknown, err
	}
	declKind, err := m.GetDeclKind(sort)
	if err != nil {
		return APISortUnknown, err
	}
	arithmetic, err := m.ArithmeticFamilyID()
	if err != nil {
		return APISortUnknown, err
	}
	if family == arithmetic {
		if declKind == 1 {
			return APISortInt, nil
		}
		if declKind == 0 {
			return APISortReal, nil
		}
	}
	if yes, e := m.IsBitVectorSort(sort); e != nil {
		return APISortUnknown, e
	} else if yes {
		return APISortBitVector, nil
	}
	array, err := m.ArrayFamilyID()
	if err != nil {
		return APISortUnknown, err
	}
	if family == array && declKind == 0 {
		return APISortArray, nil
	}
	if datatype, e := m.DatatypeFamilyID(); e == nil && family == datatype && declKind == ast.ASTDatatypeSort {
		return APISortDatatype, nil
	}
	if yes, e := m.IsDatalogRelationSort(sort); e != nil {
		return APISortUnknown, e
	} else if yes {
		return APISortRelation, nil
	}
	if yes, e := m.IsDatalogFiniteSort(sort); e != nil {
		return APISortUnknown, e
	} else if yes {
		return APISortFiniteDomain, nil
	}
	if _, _, yes, e := m.FloatingPointSortShape(sort); e != nil {
		return APISortUnknown, e
	} else if yes {
		return APISortFloatingPoint, nil
	}
	if yes, e := m.IsRoundingModeSort(sort); e != nil {
		return APISortUnknown, e
	} else if yes {
		return APISortRoundingMode, nil
	}
	if _, yes, e := m.SequenceSortElement(sort); e != nil {
		return APISortUnknown, e
	} else if yes {
		return APISortSequence, nil
	}
	if _, yes, e := m.RegexSequenceSort(sort); e != nil {
		return APISortUnknown, e
	} else if yes {
		return APISortRegex, nil
	}
	return APISortUnknown, nil
}

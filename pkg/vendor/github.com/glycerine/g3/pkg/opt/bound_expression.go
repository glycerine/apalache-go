package opt

// Source: src/opt/opt_context.cpp:1352-1381, context::to_expr.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// BoundExpression builds the retained opt expression for one bound. The
// returned AST reference is owned by the caller.
func BoundExpression(m *ast.ASTManager, bound *numeral.OwnedInfEpsInfRational) (_ ast.ASTRef, result error) {
	if m == nil {
		return ast.ASTRef{}, arena.ErrConfig
	}
	s := m.NumeralService()
	inf, err := s.InfEpsInfGetInfinity(bound)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { result = errors.Join(result, s.Del(&inf)) }()
	r, err := s.InfEpsInfGetRational(bound)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { result = errors.Join(result, s.Del(&r)) }()
	eps, err := s.InfEpsInfGetInfinitesimal(bound)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { result = errors.Join(result, s.Del(&eps)) }()
	var args []ast.ASTHandle
	var roots []ast.ASTRef
	defer func() {
		for i := range roots {
			result = errors.Join(result, m.DelRef(&roots[i]))
		}
	}()
	appendArg := func(h ast.ASTHandle) error {
		ref, err := m.NewRef(h)
		if err != nil {
			return err
		}
		roots = append(roots, ref)
		args = append(args, h)
		return nil
	}
	zeroEps, err := s.IsZero(&eps)
	if err != nil {
		return ast.ASTRef{}, err
	}
	isInt := false
	if zeroEps {
		isInt, err = s.IsInt(&r)
		if err != nil {
			return ast.ASTRef{}, err
		}
	}
	arith, err := m.ArithmeticFamilyID()
	if err != nil {
		return ast.ASTRef{}, err
	}
	zeroInf, err := s.IsZero(&inf)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if !zeroInf {
		sortKind := int32(0) // REAL_SORT
		if isInt {
			sortKind = 1 // INT_SORT
		}
		sort, err := m.MkFamilySort(arith, sortKind, nil)
		if err != nil {
			return ast.ASTRef{}, err
		}
		name, err := m.Symbols.Intern("oo")
		if err != nil {
			return ast.ASTRef{}, err
		}
		oo, err := m.MkConst(name, sort)
		if err != nil {
			return ast.ASTRef{}, err
		}
		one, err := s.IsOne(&inf)
		if err != nil {
			return ast.ASTRef{}, err
		}
		if one {
			if err := appendArg(oo); err != nil {
				return ast.ASTRef{}, err
			}
		} else {
			coefficient, err := m.ArithmeticNumeral(&inf, isInt)
			if err != nil {
				return ast.ASTRef{}, err
			}
			product, err := m.MkFamilyApp(arith, 9, nil, []ast.ASTHandle{coefficient, oo}) // OP_MUL
			if err != nil {
				return ast.ASTRef{}, err
			}
			if err := appendArg(product); err != nil {
				return ast.ASTRef{}, err
			}
		}
	}
	zeroR, err := s.IsZero(&r)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if !zeroR {
		h, err := m.ArithmeticNumeral(&r, isInt)
		if err != nil {
			return ast.ASTRef{}, err
		}
		if err := appendArg(h); err != nil {
			return ast.ASTRef{}, err
		}
	}
	if !zeroEps {
		sort, err := m.MkFamilySort(arith, 0, nil) // REAL_SORT
		if err != nil {
			return ast.ASTRef{}, err
		}
		name, err := m.Symbols.Intern("epsilon")
		if err != nil {
			return ast.ASTRef{}, err
		}
		ep, err := m.MkConst(name, sort)
		if err != nil {
			return ast.ASTRef{}, err
		}
		one, err := s.IsOne(&eps)
		if err != nil {
			return ast.ASTRef{}, err
		}
		if one {
			if err := appendArg(ep); err != nil {
				return ast.ASTRef{}, err
			}
		} else {
			coefficient, err := m.ArithmeticNumeral(&eps, isInt)
			if err != nil {
				return ast.ASTRef{}, err
			}
			product, err := m.MkFamilyApp(arith, 9, nil, []ast.ASTHandle{coefficient, ep}) // OP_MUL
			if err != nil {
				return ast.ASTRef{}, err
			}
			if err := appendArg(product); err != nil {
				return ast.ASTRef{}, err
			}
		}
	}
	var h ast.ASTHandle
	switch len(args) {
	case 0:
		zero, err := s.Zero()
		if err != nil {
			return ast.ASTRef{}, err
		}
		h, err = m.ArithmeticNumeral(&zero, true)
		result = errors.Join(err, s.Del(&zero))
		if result != nil {
			return ast.ASTRef{}, result
		}
	case 1:
		h = args[0]
	default:
		h, err = m.MkFamilyApp(arith, 6, nil, args) // OP_ADD
		if err != nil {
			return ast.ASTRef{}, err
		}
	}
	return m.NewRef(h)
}

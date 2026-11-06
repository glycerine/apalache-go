package duality

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// UnderapproxFullFormula records all resolved atoms in the source Boolean
// skeleton, independent of a connective's polarity. False array equalities are
// omitted; without extensionality, equal arrays must also have identical model
// values. This is the model cube consumed by the case-interpolation solver.
// Source: duality_rpfp.cpp:1822-1851,1965-1979.
func (g *RPFP) UnderapproxFullFormula(formula *Expr, extensional bool) (out *Expr, result error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if err := formula.open(); err != nil {
		return nil, err
	}
	if formula.value.object.context != g.context {
		return nil, ErrContextMismatch
	}
	m := g.context.manager
	root, err := formula.Raw()
	if err != nil {
		return nil, err
	}
	done := make(map[ast.ASTHandle]bool)
	truth := make(map[ast.ASTHandle]int)
	var pins []*Expr
	var literals []ast.ASTHandle
	defer func() {
		for _, pin := range pins {
			result = errors.Join(result, pin.Close())
		}
		if result != nil && out != nil {
			result = errors.Join(result, out.Close())
			out = nil
		}
	}()
	keep := func(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		pin, err := NewExpr(g.context, h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		pins = append(pins, pin)
		return h, nil
	}
	evaluate := func(h ast.ASTHandle) (ast.ASTHandle, error) {
		input, err := NewExpr(g.context, h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		pins = append(pins, input)
		value, err := g.evaluate(input)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		pins = append(pins, value)
		return value.Raw()
	}
	var visit func(ast.ASTHandle) error
	visit = func(h ast.ASTHandle) error {
		if done[h] {
			return nil
		}
		kind, err := m.GetKind(h)
		if err != nil {
			return err
		}
		equality := false
		if kind == ast.ASTApp {
			op, err := m.GetDeclKind(h)
			if err != nil {
				return err
			}
			switch op {
			case ast.ASTOpImplies, ast.ASTOpIff, ast.ASTOpAnd, ast.ASTOpOr, ast.ASTOpNot:
				count, err := m.GetNumArgs(h)
				if err != nil {
					return err
				}
				for i := uint32(0); i < count; i++ {
					child, err := m.GetArg(h, i)
					if err != nil {
						return err
					}
					if err = visit(child); err != nil {
						return err
					}
				}
				done[h] = true
				return nil
			case ast.ASTOpEq:
				equality = true
			}
		}
		value, err := g.subtermTruth(truth, h)
		if err != nil {
			return err
		}
		if value != 0 && value != 1 {
			done[h] = true
			return nil
		}
		if equality {
			left, err := m.GetArg(h, 0)
			if err != nil {
				return err
			}
			sort, err := m.GetSort(left)
			if err != nil {
				return err
			}
			family, err := m.GetFamilyID(sort)
			if err != nil {
				return err
			}
			array, err := m.ArrayFamilyID()
			if err != nil {
				return err
			}
			if family == array {
				if value == 1 && !extensional {
					right, err := m.GetArg(h, 1)
					if err != nil {
						return err
					}
					x, err := evaluate(left)
					if err != nil {
						return err
					}
					y, err := evaluate(right)
					if err != nil {
						return err
					}
					if x != y {
						value = 0
					}
				}
				if value == 0 {
					done[h] = true
					return nil
				}
			}
		}
		literal := h
		if value == 0 {
			literal, err = keep(m.MkNot(h))
			if err != nil {
				return err
			}
		}
		literals = append(literals, literal)
		done[h] = true
		return nil
	}
	timer, err := startProfile(g.context, "UnderapproxFormula")
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	if err = visit(root); err != nil {
		return nil, err
	}
	if err = timer.Close(); err != nil {
		return nil, err
	}
	var combined ast.ASTHandle
	switch len(literals) {
	case 0:
		combined, err = m.MkTrue()
	case 1:
		combined = literals[0]
	default:
		combined, err = m.MkAndArgs(literals)
	}
	if err != nil {
		return nil, err
	}
	return NewExpr(g.context, combined)
}

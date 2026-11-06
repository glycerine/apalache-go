package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// SubtermTruth returns source values 0=false, 1=true and 2=unresolved in the
// current model. Its memo borrows only subterms of the live input expression.
// Source: duality_rpfp.cpp:1584-1653.
func (g *RPFP) SubtermTruth(term *Expr) (int, error) {
	if err := g.open(); err != nil {
		return 2, err
	}
	if err := term.open(); err != nil {
		return 2, err
	}
	if term.value.object.context != g.context {
		return 2, ErrContextMismatch
	}
	h, err := term.Raw()
	if err != nil {
		return 2, err
	}
	return g.subtermTruth(make(map[ast.ASTHandle]int), h)
}

// timedSubtermTruth is used at the source's explicitly timed call sites;
// recursive and other calls to subtermTruth are intentionally untimed.
func (g *RPFP) timedSubtermTruth(memo map[ast.ASTHandle]int, h ast.ASTHandle) (_ int, result error) {
	timer, err := startProfile(g.context, "SubtermTruth")
	if err != nil {
		return 2, err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	return g.subtermTruth(memo, h)
}

func (g *RPFP) subtermTruth(memo map[ast.ASTHandle]int, h ast.ASTHandle) (out int, result error) {
	if value, found := memo[h]; found {
		return value, nil
	}
	out = 2
	defer func() {
		if result == nil {
			memo[h] = out
		}
	}()
	m := g.context.manager
	kind, err := m.GetKind(h)
	if err != nil {
		return 2, err
	}
	if kind == ast.ASTApp {
		family, err := m.GetFamilyID(h)
		if err != nil {
			return 2, err
		}
		if family == m.BasicFamilyID() {
			op, err := m.GetDeclKind(h)
			if err != nil {
				return 2, err
			}
			switch op {
			case ast.ASTOpAnd, ast.ASTOpOr, ast.ASTOpImplies, ast.ASTOpNot:
				n, err := m.GetNumArgs(h)
				if err != nil {
					return 2, err
				}
				out = 1
				if op == ast.ASTOpOr || op == ast.ASTOpImplies {
					out = 0
				}
				for i := uint32(0); i < n; i++ {
					child, err := m.GetArg(h, i)
					if err != nil {
						return 2, err
					}
					value, err := g.subtermTruth(memo, child)
					if err != nil {
						return 2, err
					}
					if op == ast.ASTOpNot || (op == ast.ASTOpImplies && i == 0) {
						if value != 2 {
							value = 1 - value
						}
					}
					if op == ast.ASTOpNot {
						return value, nil
					}
					if op == ast.ASTOpAnd && value == 0 {
						return 0, nil
					}
					if op != ast.ASTOpAnd && value == 1 {
						return 1, nil
					}
					if value == 2 {
						out = 2
					}
				}
				return out, nil
			}
		}
	}
	var polarity bool
	var names []usymbol.Symbol
	label, err := m.IsLabel(h, &polarity, &names)
	if err != nil {
		return 2, err
	}
	if label {
		child, err := m.GetArg(h, 0)
		if err != nil {
			return 2, err
		}
		return g.subtermTruth(memo, child)
	}
	term, err := NewExpr(g.context, h)
	if err != nil {
		return 2, err
	}
	defer func() { result = errors.Join(result, term.Close()) }()
	value, err := g.evaluate(term)
	if err != nil {
		return 2, err
	}
	defer func() { result = errors.Join(result, value.Close()) }()
	raw, err := value.Raw()
	if err != nil {
		return 2, err
	}
	negative, child, err := m.IsNot(raw)
	if err != nil {
		return 2, err
	}
	if negative {
		raw = child
	}
	equality := false
	valueKind, err := m.GetKind(raw)
	if err != nil {
		return 2, err
	}
	if valueKind == ast.ASTApp {
		family, err := m.GetFamilyID(raw)
		if err != nil {
			return 2, err
		}
		op, err := m.GetDeclKind(raw)
		if err != nil {
			return 2, err
		}
		equality = family == m.BasicFamilyID() && op == ast.ASTOpEq
	}
	if equality {
		left, err := m.GetArg(raw, 0)
		if err != nil {
			return 2, err
		}
		sort, err := m.GetSort(left)
		if err != nil {
			return 2, err
		}
		family, err := m.GetFamilyID(sort)
		if err != nil {
			return 2, err
		}
		array, err := m.ArrayFamilyID()
		if err != nil {
			return 2, err
		}
		if family == array {
			eq, err := NewExpr(g.context, raw)
			if err != nil {
				return 2, err
			}
			defer func() { result = errors.Join(result, eq.Close()) }()
			reduced, err := g.EvalArrayEquality(eq)
			if err != nil {
				return 2, err
			}
			defer func() { result = errors.Join(result, reduced.Close()) }()
			raw, err = reduced.Raw()
			if err != nil {
				return 2, err
			}
			if negative {
				neg, err := reduced.Not()
				if err != nil {
					return 2, err
				}
				defer func() { result = errors.Join(result, neg.Close()) }()
				evaluated, err := g.evaluate(neg)
				if err != nil {
					return 2, err
				}
				defer func() { result = errors.Join(result, evaluated.Close()) }()
				raw, err = evaluated.Raw()
				if err != nil {
					return 2, err
				}
			}
		} else if negative {
			return 2, nil
		}
	} else if negative {
		return 2, nil
	}
	if m.IsTrue(raw) {
		return 1, nil
	}
	if m.IsFalse(raw) {
		return 0, nil
	}
	return 2, nil
}

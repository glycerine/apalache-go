package arith

// Source: src/tactic/arith/lia2pb_tactic.cpp is_target_core and visitor/check.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func lia2pbBits(n *big.Rat) (uint32, error) {
	if n.Sign() < 0 || !n.IsInt() {
		return 0, ast.ErrASTDomain
	}
	bits := n.Num().BitLen()
	if bits == 0 {
		bits = 1
	}
	return uint32(bits), nil
}

func (t *LIA2PBTactic) target(variable ast.ASTHandle) (upper big.Rat, found bool, err error) {
	constant, err := t.bounds.constant(variable)
	if err != nil || !constant {
		return upper, false, err
	}
	lower, _, found, err := t.bounds.Lower(variable)
	if err != nil || !found {
		return upper, false, err
	}
	upper, _, found, err = t.bounds.Upper(variable)
	if err != nil || !found || lower.Sign() != 0 || upper.Sign() < 0 {
		return upper, false, err
	}
	bits, err := lia2pbBits(&upper)
	return upper, bits <= t.maxBits, err
}

// The live visitor instantiation marks every expression and ignores quantifier
// annotations. Children are visited in source order before their parent.
func (t *LIA2PBTactic) check(goal *tactic.Goal) (bool, error) {
	type frame struct {
		term ast.ASTHandle
		next uint32
	}
	visited := make(map[ast.ASTHandle]bool)
	size, err := goal.Size()
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < size; i++ {
		root, err := goal.Form(i)
		if err != nil {
			return false, err
		}
		if visited[root] {
			continue
		}
		visited[root] = true
		stack := []frame{{term: root}}
		for len(stack) != 0 {
			f := &stack[len(stack)-1]
			kind, err := t.manager.GetKind(f.term)
			if err != nil {
				return false, err
			}
			var child ast.ASTHandle
			switch kind {
			case ast.ASTApp:
				n, e := t.manager.GetNumArgs(f.term)
				if e != nil {
					return false, e
				}
				if f.next < n {
					child, err = t.manager.GetArg(f.term, f.next)
					f.next++
				}
			case ast.ASTQuantifier:
				if f.next == 0 {
					child, err = t.manager.GetQuantifierBody(f.term)
					f.next++
				}
			case ast.ASTVar:
			default:
				return false, ast.ErrASTDomain
			}
			if err != nil {
				return false, err
			}
			if !child.IsNull() {
				if !visited[child] {
					visited[child] = true
					stack = append(stack, frame{term: child})
				}
				continue
			}
			term := f.term
			stack = stack[:len(stack)-1]
			accepted, err := t.checkNode(term, kind)
			if err != nil {
				return false, err
			}
			if !accepted {
				return false, t.traceFailed(term)
			}
		}
	}
	return true, nil
}

func (t *LIA2PBTactic) checkNode(term ast.ASTHandle, kind ast.ASTKind) (bool, error) {
	if kind != ast.ASTApp {
		return false, nil
	}
	family, op, args, _, err := t.bounds.app(term)
	if err != nil {
		return false, err
	}
	if family == t.manager.BasicFamilyID() {
		return true, nil
	}
	if family == t.bounds.family {
		switch op {
		case 2, 3, 4, 5, 6, 0: // comparisons, addition, numeral
			return true, nil
		case 9: // a numeral times a single expression
			if len(args) != 2 {
				return false, nil
			}
			_, _, numeral, err := t.manager.ArithmeticNumeralText(args[0])
			return numeral, err
		default:
			return false, nil
		}
	}
	constant, err := t.bounds.constant(term)
	if err != nil {
		return false, err
	}
	sort, err := t.manager.GetSort(term)
	if err != nil {
		return false, err
	}
	sortFamily, err := t.manager.GetFamilyID(sort)
	if err != nil || sortFamily != t.bounds.family {
		return err == nil, err
	}
	if !constant {
		return false, nil
	}
	if t.partial {
		return true, nil
	}
	sortKind, err := t.manager.GetDeclKind(sort)
	if err != nil || sortKind != 1 {
		return false, err
	}
	_, bounded, err := t.target(term)
	return bounded, err
}

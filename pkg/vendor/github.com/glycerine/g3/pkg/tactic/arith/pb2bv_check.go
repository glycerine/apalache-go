package arith

// Source: src/tactic/arith/pb2bv_tactic.cpp only_01_visitor/quick_pb_check.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type pbNonPB struct{ term ast.ASTHandle }

func (e pbNonPB) Error() string { return "not pseudo-Boolean" }

// The live visitor instantiation marks every expression and ignores quantifier
// annotations. Children are visited in source order before their parent.
func (t *PB2BVTactic) check(goal *tactic.Goal) (bool, error) {
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
				return false, t.nonPBAt(term, 51)
			}
		}
	}
	return true, nil
}

func (t *PB2BVTactic) checkNode(term ast.ASTHandle, kind ast.ASTKind) (bool, error) {
	if kind != ast.ASTApp {
		return false, nil
	}
	family, op, args, _, err := t.bounds.app(term)
	if err != nil {
		return false, err
	}
	if family == t.manager.BasicFamilyID() {
		if op == ast.ASTOpDistinct {
			return false, nil
		}
		if op == ast.ASTOpIte {
			s, e := t.manager.GetSort(term)
			if e != nil {
				return false, e
			}
			b, e := t.manager.MkBoolSort()
			return s == b, e
		}
		return true, nil
	}
	if family == t.bounds.family {
		switch op {
		case 0, 2, 3, 4, 5, 6:
			return true, nil
		case 9:
			if len(args) != 2 {
				return false, nil
			}
			_, _, number, e := t.manager.ArithmeticNumeralText(args[0])
			return number, e
		default:
			return false, nil
		}
	}
	constant, err := t.bounds.constant(term)
	if err != nil || !constant {
		return false, err
	}
	sort, err := t.manager.GetSort(term)
	if err != nil {
		return false, err
	}
	boolean, err := t.manager.MkBoolSort()
	if err != nil {
		return false, err
	}
	if sort == boolean {
		return true, nil
	}
	sf, err := t.manager.GetFamilyID(sort)
	if err != nil || sf != t.bounds.family {
		return false, err
	}
	sk, err := t.manager.GetDeclKind(sort)
	if err != nil || sk != 1 {
		return false, err
	}
	l, _, found, err := t.bounds.Lower(term)
	if err != nil || !found {
		return false, err
	}
	u, _, found, err := t.bounds.Upper(term)
	if err != nil || !found {
		return false, err
	}
	return (l.Sign() == 0 || l.RatString() == "1") && (u.Sign() == 0 || u.RatString() == "1"), nil
}
func (t *PB2BVTactic) constraintCore(term ast.ASTHandle) (bool, error) {
	f, op, args, app, err := t.bounds.app(term)
	if err != nil || !app {
		return false, err
	}
	if f == t.bounds.family && (op == 2 || op == 3) {
		return true, nil
	}
	if f != t.manager.BasicFamilyID() || op != ast.ASTOpEq || len(args) != 2 {
		return false, nil
	}
	s, err := t.manager.GetSort(args[0])
	if err != nil {
		return false, err
	}
	f, err = t.manager.GetFamilyID(s)
	if err != nil || f != t.bounds.family {
		return false, err
	}
	k, err := t.manager.GetDeclKind(s)
	return k == 1, err
}

package arith

// Source: src/tactic/arith/fix_dl_var_tactic.cpp is_target.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

var errNotDifferenceLogic = errors.New("not the source difference-logic target")

type fixDLTarget struct {
	owner              *FixDLVarTactic
	visited            map[ast.ASTHandle]bool
	todo               []ast.ASTHandle
	occurrences, units map[ast.ASTHandle]uint32
	order              []ast.ASTHandle
}

func (t *FixDLVarTactic) dlApp(term ast.ASTHandle) (family ast.FamilyID, op int32, args []ast.ASTHandle, app bool, err error) {
	m := t.manager
	kind, err := m.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return family, op, nil, false, err
	}
	family, err = m.GetFamilyID(term)
	if err != nil {
		return family, op, nil, false, err
	}
	op, err = m.GetDeclKind(term)
	if err != nil {
		return family, op, nil, false, err
	}
	count, err := m.GetNumArgs(term)
	if err != nil {
		return family, op, nil, false, err
	}
	args = make([]ast.ASTHandle, count)
	for i := range args {
		args[i], err = m.GetArg(term, uint32(i))
		if err != nil {
			return family, op, nil, false, err
		}
	}
	return family, op, args, true, nil
}
func (s *fixDLTarget) isArithmetic(term ast.ASTHandle) (bool, error) {
	sort, err := s.owner.manager.GetSort(term)
	if err != nil {
		return false, err
	}
	family, err := s.owner.manager.GetFamilyID(sort)
	return family == s.owner.family, err
}
func (s *fixDLTarget) uninterpreted(term ast.ASTHandle) (bool, error) {
	family, _, _, app, err := s.owner.dlApp(term)
	return app && family != s.owner.family, err
}
func (s *fixDLTarget) numeral(term ast.ASTHandle) (bool, error) {
	family, op, _, app, err := s.owner.dlApp(term)
	return app && family == s.owner.family && op == 0, err
}
func (s *fixDLTarget) visit(term ast.ASTHandle, nested bool) error {
	family, _, args, app, err := s.owner.dlApp(term)
	if err != nil {
		return err
	}
	if app && family == ast.NullFamilyID && len(args) == 0 {
		arithmetic, err := s.isArithmetic(term)
		if err != nil {
			return err
		}
		if arithmetic {
			if _, found := s.occurrences[term]; !found {
				s.order = append(s.order, term)
			}
			s.occurrences[term]++
			if !nested {
				s.units[term]++
			}
		}
	}
	if !s.visited[term] {
		s.visited[term] = true
		s.todo = append(s.todo, term)
	}
	return nil
}
func (s *fixDLTarget) fail(first, second ast.ASTHandle) error {
	if err := s.owner.traceDLFailure(first, second); err != nil {
		return err
	}
	return errNotDifferenceLogic
}
func (s *fixDLTarget) timesMinusOne(term ast.ASTHandle) (ast.ASTHandle, bool, error) {
	family, op, args, app, err := s.owner.dlApp(term)
	if err != nil || !app || family != s.owner.family || op != 9 || len(args) != 2 {
		return ast.ASTHandle{}, false, err
	}
	text, _, matched, err := s.owner.manager.ArithmeticNumeralText(args[0])
	if err != nil || !matched || text != "-1" {
		return ast.ASTHandle{}, false, err
	}
	return args[1], true, nil
}
func (s *fixDLTarget) arithmeticAtom(lhs, rhs ast.ASTHandle, nested bool) error {
	a, err := s.uninterpreted(lhs)
	if err != nil {
		return err
	}
	b, err := s.uninterpreted(rhs)
	if err != nil {
		return err
	}
	if a && b {
		if err = s.visit(lhs, nested); err != nil {
			return err
		}
		return s.visit(rhs, nested)
	}
	num, err := s.numeral(lhs)
	if err != nil {
		return err
	}
	if num {
		lhs, rhs = rhs, lhs
	}
	num, err = s.numeral(rhs)
	if err != nil {
		return err
	}
	if !num {
		return s.fail(lhs, rhs)
	}
	family, op, args, app, err := s.owner.dlApp(lhs)
	if err != nil {
		return err
	}
	if app && family == s.owner.family && op == 6 && len(args) == 2 {
		negative, minus, e := s.timesMinusOne(args[1])
		if e != nil {
			return e
		}
		left, e := s.uninterpreted(args[0])
		if e != nil {
			return e
		}
		right := false
		if minus {
			right, e = s.uninterpreted(negative)
			if e != nil {
				return e
			}
		}
		if minus && left && right {
			if err = s.visit(args[0], nested); err != nil {
				return err
			}
			return s.visit(negative, nested)
		}
		if err = s.owner.traceDLAtomShape(args[0], negative, minus, left, right); err != nil {
			return err
		}
	}
	return s.fail(lhs, rhs)
}
func (s *fixDLTarget) arithmetic(term ast.ASTHandle, nested bool) error {
	m := s.owner.manager
	_, op, args, _, err := s.owner.dlApp(term)
	if err != nil {
		return err
	}
	sort, err := m.GetSort(term)
	if err != nil {
		return err
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return err
	}
	if sort == boolean {
		if len(args) < 2 {
			return ast.ErrASTDomain
		} // source arithmetic atom precondition
		return s.arithmeticAtom(args[0], args[1], nested)
	}
	if op == 6 && len(args) == 2 {
		num, err := s.numeral(args[0])
		if err != nil {
			return err
		}
		c, err := s.uninterpreted(args[1])
		if err != nil {
			return err
		}
		if num && c {
			return s.visit(args[1], nested)
		}
	}
	return s.fail(term, ast.ASTHandle{})
}
func (s *fixDLTarget) processApp(args []ast.ASTHandle) error {
	// Source resets nested=false for arguments of foreign-family apps,
	// even when the parent occurs inside Boolean structure.
	for _, arg := range args {
		if err := s.visit(arg, false); err != nil {
			return err
		}
	}
	return nil
}
func (s *fixDLTarget) process(root ast.ASTHandle) error {
	if s.visited[root] {
		return nil
	}
	for {
		family, op, args, app, err := s.owner.dlApp(root)
		if err != nil {
			return err
		}
		if app && family == s.owner.manager.BasicFamilyID() && op == ast.ASTOpNot && len(args) == 1 {
			root = args[0]
			continue
		}
		if app && family == s.owner.family {
			return s.arithmetic(root, false)
		}
		break
	}
	s.todo = append(s.todo, root)
	s.visited[root] = true
	for len(s.todo) > 0 {
		term := s.todo[len(s.todo)-1]
		s.todo = s.todo[:len(s.todo)-1]
		family, op, args, app, err := s.owner.dlApp(term)
		if err != nil {
			return err
		}
		if !app {
			return s.fail(term, ast.ASTHandle{})
		}
		if family == s.owner.manager.BasicFamilyID() && op == ast.ASTOpEq {
			if len(args) != 2 {
				return ast.ErrASTDomain
			}
			arithmetic, err := s.isArithmetic(args[0])
			if err != nil {
				return err
			}
			if arithmetic {
				if err = s.arithmeticAtom(args[0], args[1], true); err != nil {
					return err
				}
			} else if err = s.processApp(args); err != nil {
				return err
			}
		} else if family == s.owner.family {
			if err = s.arithmetic(term, true); err != nil {
				return err
			}
		} else if err = s.processApp(args); err != nil {
			return err
		}
	}
	return nil
}
func (t *FixDLVarTactic) selectDLVariable(goal *tactic.Goal) (ast.ASTHandle, error) {
	s := fixDLTarget{owner: t, visited: make(map[ast.ASTHandle]bool), occurrences: make(map[ast.ASTHandle]uint32), units: make(map[ast.ASTHandle]uint32)}
	count, err := goal.Size()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for i := uint32(0); i < count; i++ {
		term, err := goal.Form(i)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if err = s.process(term); err != nil {
			if errors.Is(err, errNotDifferenceLogic) {
				return ast.ASTHandle{}, nil
			}
			return ast.ASTHandle{}, err
		}
	}
	var unit, total ast.ASTHandle
	var bestUnit, bestTotal uint32
	for _, term := range s.order {
		if n := s.units[term]; n > bestUnit {
			unit, bestUnit = term, n
		}
		if n := s.occurrences[term]; n > bestTotal {
			total, bestTotal = term, n
		}
	}
	if err = t.traceDLChoice(unit, total, bestUnit, bestTotal); err != nil {
		return ast.ASTHandle{}, err
	}
	if bestTotal > 10*bestUnit {
		return total, nil
	}
	return unit, nil
}

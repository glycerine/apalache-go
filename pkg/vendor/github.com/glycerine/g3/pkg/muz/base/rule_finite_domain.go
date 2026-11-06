package base

import "github.com/glycerine/g3/pkg/ast"

func finiteDomainExpression(manager *ast.ASTManager, roots []ast.ASTHandle) (bool, error) {
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return false, err
	}
	bitVector, err := manager.BitVectorUtilityFamilyID()
	if err != nil {
		return false, err
	}
	stack := append([]ast.ASTHandle(nil), roots...)
	seen := make(map[ast.ASTHandle]bool)
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if seen[current] {
			continue
		}
		seen[current] = true
		kind, e := manager.GetKind(current)
		if e != nil {
			return false, e
		}
		if kind == ast.ASTQuantifier {
			return false, nil
		}
		sort, e := manager.GetSort(current)
		if e != nil {
			return false, e
		}
		if sort != boolean {
			family, x := manager.GetFamilyID(sort)
			if x != nil {
				return false, x
			}
			if family != bitVector {
				return false, nil
			}
		}
		if kind == ast.ASTApp {
			count, x := manager.GetNumArgs(current)
			if x != nil {
				return false, x
			}
			for i := count; i > 0; i-- {
				argument, y := manager.GetArg(current, i-1)
				if y != nil {
					return false, y
				}
				stack = append(stack, argument)
			}
		}
	}
	return true, nil
}

func (r *Rule) IsFiniteDomain() (bool, error) {
	state, err := r.open()
	if err != nil {
		return false, err
	}
	roots := make([]ast.ASTHandle, 0, len(state.tail)+4)
	for i, tail := range state.tail {
		if i >= state.uninterpreted {
			roots = append(roots, tail.handle)
			continue
		}
		count, e := state.manager.GetNumArgs(tail.handle)
		if e != nil {
			return false, e
		}
		for j := uint32(0); j < count; j++ {
			argument, x := state.manager.GetArg(tail.handle, j)
			if x != nil {
				return false, x
			}
			roots = append(roots, argument)
		}
	}
	count, err := state.manager.GetNumArgs(state.head)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < count; i++ {
		argument, e := state.manager.GetArg(state.head, i)
		if e != nil {
			return false, e
		}
		roots = append(roots, argument)
	}
	return finiteDomainExpression(state.manager, roots)
}

func (s *RuleSet) IsFiniteDomain() (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	for _, rule := range s.rules {
		finite, err := rule.IsFiniteDomain()
		if err != nil || !finite {
			return false, err
		}
	}
	return true, nil
}

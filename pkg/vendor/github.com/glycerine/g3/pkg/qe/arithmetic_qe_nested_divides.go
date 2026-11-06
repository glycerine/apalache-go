package qe

// Source: src/qe/qe_arith_plugin.cpp bounds_proc::get_nested_divs.

import "github.com/glycerine/g3/pkg/ast"

func (p *ArithmeticBoundsProcessor) addNestedDivides(variable *ast.ContainsApp, expression ast.ASTHandle) (bool, error) {
	target, err := variable.Target()
	if err != nil {
		return false, err
	}
	todo := []ast.ASTHandle{expression}
	visited := make(map[ast.ASTHandle]bool)
	for len(todo) != 0 {
		current := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if visited[current] {
			continue
		}
		visited[current] = true
		contains, cause := variable.Check(current)
		if cause != nil {
			return false, cause
		}
		if !contains {
			continue
		}
		if current == target {
			return false, nil
		}
		if kind, cause := p.manager.GetKind(current); cause != nil || kind != ast.ASTApp {
			return false, cause
		}
		args, modular, cause := p.util.operation(current, p.util.family, 13)
		if cause != nil {
			return false, cause
		}
		if modular && len(args) == 2 {
			divisor, numeral, cause := p.util.numeral(args[1])
			if cause != nil {
				return false, cause
			}
			if numeral {
				coefficient, rest, found, cause := p.util.Coefficient(variable, args[0])
				if cause != nil {
					return false, cause
				}
				if found {
					if cause = p.appendDivisibility(current, rest, coefficient, divisor, false, true); cause != nil {
						return false, cause
					}
					continue
				}
				if rest != (ast.ASTRef{}) {
					if cause = p.manager.DelRef(&rest); cause != nil {
						return false, cause
					}
				}
			}
		}
		count, cause := p.manager.GetNumArgs(current)
		if cause != nil {
			return false, cause
		}
		for i := uint32(0); i < count; i++ {
			argument, cause := p.manager.GetArg(current, i)
			if cause != nil {
				return false, cause
			}
			todo = append(todo, argument)
		}
	}
	return true, nil
}

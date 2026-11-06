package base

// Source: src/muz/base/bind_variables.cpp bind_variables::abstract.

import "github.com/glycerine/g3/pkg/ast"

// usedVariables returns referenced implicit constants in quantifier declaration
// order. The source visits application arguments from right to left and then
// reverses its discovered variables when constructing the quantifier.
func (b *VariableBinder) usedVariables(root ast.ASTHandle) ([]ast.ASTHandle, error) {
	candidates := make(map[ast.ASTHandle]bool, len(b.variables))
	for _, v := range b.variables {
		candidates[v] = true
	}
	visited := make(map[ast.ASTHandle]bool)
	stack := []ast.ASTHandle{root}
	var found []ast.ASTHandle
	for len(stack) > 0 {
		n := len(stack) - 1
		current := stack[n]
		stack = stack[:n]
		if visited[current] {
			continue
		}
		visited[current] = true
		if candidates[current] {
			found = append(found, current)
			continue
		}
		kind, err := b.manager.GetKind(current)
		if err != nil {
			return nil, err
		}
		switch kind {
		case ast.ASTApp:
			count, e := b.manager.GetNumArgs(current)
			if e != nil {
				return nil, e
			}
			for i := uint32(0); i < count; i++ {
				v, e := b.manager.GetArg(current, i)
				if e != nil {
					return nil, e
				}
				stack = append(stack, v)
			}
		case ast.ASTQuantifier:
			body, e := b.manager.GetQuantifierBody(current)
			if e != nil {
				return nil, e
			}
			stack = append(stack, body)
			count, e := b.manager.GetNumPatterns(current)
			if e != nil {
				return nil, e
			}
			for i := count; i > 0; i-- {
				v, e := b.manager.GetPattern(current, i-1)
				if e != nil {
					return nil, e
				}
				stack = append(stack, v)
			}
		}
	}
	for i, j := 0, len(found)-1; i < j; i, j = i+1, j-1 {
		found[i], found[j] = found[j], found[i]
	}
	return found, nil
}

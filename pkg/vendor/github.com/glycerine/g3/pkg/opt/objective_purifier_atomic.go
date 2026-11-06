package opt

import "github.com/glycerine/g3/pkg/ast"

func (p *ObjectivePurifier) propositional(root ast.ASTHandle) (bool, error) {
	stack := []ast.ASTHandle{root}
	seen := make(map[ast.ASTHandle]bool)
	for len(stack) != 0 {
		h := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[h] {
			continue
		}
		seen[h] = true
		kind, err := p.manager.GetKind(h)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp {
			return false, nil
		}
		family, err := p.manager.GetFamilyID(h)
		if err != nil {
			return false, err
		}
		n, err := p.manager.GetNumArgs(h)
		if err != nil {
			return false, err
		}
		if family != p.manager.BasicFamilyID() && !(family == ast.NullFamilyID && n == 0) {
			return false, nil
		}
		for i := uint32(0); i < n; i++ {
			arg, e := p.manager.GetArg(h, i)
			if e != nil {
				return false, e
			}
			stack = append(stack, arg)
		}
	}
	return true, nil
}

func (p *ObjectivePurifier) atomic(term ast.ASTHandle, hard *[]ast.ASTHandle) (ast.ASTHandle, error) {
	yes, err := p.propositional(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if yes {
		return term, nil
	}
	return p.definition(term, hard, true)
}

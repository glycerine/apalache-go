package opt

import "github.com/glycerine/g3/pkg/ast"

func (p *ObjectivePurifier) arithmeticTerm(term ast.ASTHandle, hard *[]ast.ASTHandle) (ast.ASTHandle, error) {
	if add, err := p.appKind(term, p.arith, 6); err != nil {
		return ast.ASTHandle{}, err
	} else if add {
		n, err := p.manager.GetNumArgs(term)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		args := make([]ast.ASTHandle, n)
		changed := false
		for i := uint32(0); i < n; i++ {
			arg, e := p.manager.GetArg(term, i)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			linear, e := p.mulConst(arg)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if linear {
				args[i] = arg
				continue
			}
			args[i], e = p.definition(arg, hard, false)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			changed = true
		}
		if !changed {
			return term, nil
		}
		result, err := p.manager.MkFamilyApp(p.arith, 6, nil, args)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		p.stats.RebuiltSums++
		return p.hold(result)
	}
	sort, err := p.manager.GetSort(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	family, err := p.manager.GetFamilyID(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	linear, err := p.mulConst(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ite, err := p.appKind(term, p.manager.BasicFamilyID(), ast.ASTOpIte)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if family == p.arith && !linear || ite {
		return p.definition(term, hard, false)
	}
	return term, nil
}

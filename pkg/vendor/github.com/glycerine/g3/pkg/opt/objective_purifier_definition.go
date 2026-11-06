package opt

import "github.com/glycerine/g3/pkg/ast"

func (p *ObjectivePurifier) definition(term ast.ASTHandle, hard *[]ast.ASTHandle, soft bool) (ast.ASTHandle, error) {
	prefix, err := p.definitionPrefix(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := p.manager.GetSort(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	fresh, err := p.manager.MkFreshConst(prefix, sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if _, err = p.hold(fresh); err != nil {
		return ast.ASTHandle{}, err
	}
	declaration, err := p.manager.GetDecl(fresh)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	equality, err := p.manager.MkEq(fresh, term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if _, err = p.hold(equality); err != nil {
		return ast.ASTHandle{}, err
	}
	*hard = append(*hard, equality)
	if err = p.filter.Insert(declaration); err != nil {
		return ast.ASTHandle{}, err
	}
	p.stats.FreshDeclarations++
	if soft {
		p.stats.SoftDefinitions++
	} else {
		p.stats.ArithmeticDefinitions++
	}
	return fresh, nil
}

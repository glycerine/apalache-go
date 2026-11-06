package opt

import "github.com/glycerine/g3/pkg/ast"

func (p *ObjectivePurifier) appKind(h ast.ASTHandle, family ast.FamilyID, kind int32) (bool, error) {
	k, err := p.manager.GetKind(h)
	if err != nil || k != ast.ASTApp {
		return false, err
	}
	f, err := p.manager.GetFamilyID(h)
	if err != nil || f != family {
		return false, err
	}
	op, err := p.manager.GetDeclKind(h)
	return op == kind, err
}

func (p *ObjectivePurifier) uninterpretedConstant(h ast.ASTHandle) (bool, error) {
	k, err := p.manager.GetKind(h)
	if err != nil || k != ast.ASTApp {
		return false, err
	}
	n, err := p.manager.GetNumArgs(h)
	if err != nil || n != 0 {
		return false, err
	}
	family, err := p.manager.GetFamilyID(h)
	return family == ast.NullFamilyID, err
}

func (p *ObjectivePurifier) numeral(h ast.ASTHandle) (bool, error) {
	_, _, matched, err := p.manager.ArithmeticNumeralText(h)
	return matched, err
}

func (p *ObjectivePurifier) mulConst(h ast.ASTHandle) (bool, error) {
	if yes, err := p.uninterpretedConstant(h); err != nil || yes {
		return yes, err
	}
	if yes, err := p.numeral(h); err != nil || yes {
		return yes, err
	}
	yes, err := p.appKind(h, p.arith, 9)
	if err != nil || !yes {
		return false, err
	}
	n, err := p.manager.GetNumArgs(h)
	if err != nil || n != 2 {
		return false, err
	}
	a, err := p.manager.GetArg(h, 0)
	if err != nil {
		return false, err
	}
	b, err := p.manager.GetArg(h, 1)
	if err != nil {
		return false, err
	}
	an, err := p.numeral(a)
	if err != nil {
		return false, err
	}
	bc, err := p.uninterpretedConstant(b)
	if err != nil {
		return false, err
	}
	if an && bc {
		return true, nil
	}
	bn, err := p.numeral(b)
	if err != nil {
		return false, err
	}
	ac, err := p.uninterpretedConstant(a)
	if err != nil {
		return false, err
	}
	return bn && ac, nil
}

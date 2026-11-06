package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

func (p *NonlinearArithmeticPlugin) IsUninterpreted(application ast.ASTHandle) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if kind, err := p.manager.GetKind(application); err != nil || kind != ast.ASTApp {
		return false, errors.Join(ast.ErrASTDomain, err)
	}
	if p.produceModels {
		return true, nil
	}
	family, err := p.manager.GetFamilyID(application)
	if err != nil {
		return false, err
	}
	if family != p.family {
		return true, nil
	}
	kind, err := p.manager.GetDeclKind(application)
	if err != nil {
		return false, err
	}
	switch kind {
	case 0, 2, 3, 4, 5, 6, 7, 8:
		return false, nil
	case 9:
		n, err := p.manager.GetNumArgs(application)
		if err != nil {
			return false, err
		}
		for i := uint32(0); i < n; i++ {
			a, cause := p.manager.GetArg(application, i)
			if cause != nil {
				return false, cause
			}
			_, numeral, cause := p.util.term.numeral(a)
			if cause != nil {
				return false, cause
			}
			if numeral {
				return false, nil
			}
		}
		return true, nil
	default:
		return true, nil
	}
}

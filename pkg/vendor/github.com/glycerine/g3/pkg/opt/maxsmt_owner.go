package opt

// Source: src/opt/sortmax.cpp m_trail ownership.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type maxSMTASTOwner struct {
	manager *ast.ASTManager
	refs    []ast.ASTRef
}

func (o *maxSMTASTOwner) hold(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := o.manager.NewRef(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	o.refs = append(o.refs, ref)
	return value, nil
}

func (o *maxSMTASTOwner) close() (result error) {
	for i := len(o.refs) - 1; i >= 0; i-- {
		result = errors.Join(result, o.manager.DelRef(&o.refs[i]))
	}
	o.refs = nil
	return result
}

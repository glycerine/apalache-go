package api

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (v *ASTVector) Translate(target *SMTContext) (result *ASTVector, resultErr error) {
	if err := v.check(); err != nil {
		return nil, err
	}
	targetManager, err := target.Manager()
	if err != nil {
		return nil, err
	}
	if targetManager == v.owner.manager {
		return nil, ast.ErrASTDomain
	}
	owner, err := newAPIContextResults(targetManager, true)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, owner.Close())
			result = nil
		}
	}()
	object, err := v.object.translate(owner)
	if err != nil {
		return nil, err
	}
	return &ASTVector{owner: owner, object: object}, nil
}

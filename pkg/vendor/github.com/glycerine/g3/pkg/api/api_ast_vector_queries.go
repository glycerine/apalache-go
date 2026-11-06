package api

import "github.com/glycerine/g3/pkg/ast"

func (v *ASTVector) Len() (uint32, error) {
	if err := v.check(); err != nil {
		return 0, err
	}
	return v.object.size()
}

func (v *ASTVector) At(index uint32) (ast.ASTHandle, error) {
	if err := v.check(); err != nil {
		return ast.ASTHandle{}, err
	}
	return v.object.get(index)
}

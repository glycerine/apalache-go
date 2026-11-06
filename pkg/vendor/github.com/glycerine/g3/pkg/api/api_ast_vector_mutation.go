package api

import "github.com/glycerine/g3/pkg/ast"

func (v *ASTVector) Set(index uint32, value ast.ASTHandle) error {
	if err := v.check(); err != nil {
		return err
	}
	return v.object.set(index, value)
}

func (v *ASTVector) Resize(size uint32) error {
	if err := v.check(); err != nil {
		return err
	}
	return v.object.resize(size)
}

func (v *ASTVector) Push(value ast.ASTHandle) error {
	if err := v.check(); err != nil {
		return err
	}
	return v.object.push(value)
}

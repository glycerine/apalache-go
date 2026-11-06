package qe

// Source: src/qe/qe.h qe_solver_plugin::project concrete BV specialization.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

type BitVectorModelProjector interface {
	ProjectModel(*ast.ContainsApp, *model.Model, ast.ASTRef, Config) (bool, error)
}

var _ BitVectorModelProjector = (*BitVectorPlugin)(nil)

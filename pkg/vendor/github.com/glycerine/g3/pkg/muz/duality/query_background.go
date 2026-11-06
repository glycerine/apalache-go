package duality

// Source: duality_dl_interface.cpp, query background-axiom installation.

import (
	"github.com/glycerine/g3/pkg/ast"
	graph "github.com/glycerine/g3/pkg/duality"
)

func (q *PreparedQuery) AddBackground(formula ast.ASTHandle) error {
	if q == nil || q.manager == nil || q.self != q || q.closing {
		return graph.ErrWrapperOwner
	}
	ref, err := q.manager.NewRef(formula)
	if err != nil {
		return err
	}
	q.background = append(q.background, ref)
	return nil
}

package clp

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *Engine) Query(query ast.ASTHandle) (status lbool.LBool, resultErr error) {
	if err := e.open(); err != nil {
		return lbool.Undef, err
	}
	g, err := e.ground(query)
	if err != nil {
		return lbool.Undef, err
	}
	owner := &clpRefOwner{manager: e.manager}
	owner.take(g)
	defer func() {
		resultErr = owner.finish(resultErr)
		if resultErr != nil {
			status = lbool.Undef
		}
	}()
	h, err := e.manager.RefGet(owner.refs[0])
	if err != nil {
		return lbool.Undef, err
	}
	status, err = e.search([]ast.ASTHandle{h}, e.depth, 0)
	return status, err
}

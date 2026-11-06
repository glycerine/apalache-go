package base

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (e *EngineBase) QueryRelations(relations []ast.ASTHandle) (result lbool.LBool, resultErr error) {
	if err := e.open(); err != nil {
		return lbool.Undef, err
	}
	if len(relations) != 1 {
		return lbool.Undef, nil
	}
	declaration := relations[0]
	arity, err := e.manager.GetArity(declaration)
	if err != nil {
		return lbool.Undef, err
	}
	args := make([]ast.ASTHandle, arity)
	sorts := make([]ast.ASTHandle, arity)
	names := make([]usymbol.Symbol, arity)
	for i := uint32(0); i < arity; i++ {
		sorts[i], err = e.manager.GetDomain(declaration, i)
		if err != nil {
			return lbool.Undef, err
		}
		args[i], err = e.manager.MkVar(i, sorts[i])
		if err != nil {
			return lbool.Undef, err
		}
		names[i] = usymbol.Numerical(i)
	}
	query, err := e.manager.MkApp(declaration, args)
	if err != nil {
		return lbool.Undef, err
	}
	if arity != 0 {
		for i, j := 0, len(sorts)-1; i < j; i, j = i+1, j-1 {
			sorts[i], sorts[j] = sorts[j], sorts[i]
			names[i], names[j] = names[j], names[i]
		}
		query, err = e.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: sorts, Names: names, Body: query})
		if err != nil {
			return lbool.Undef, err
		}
	}
	pin, err := e.manager.NewRef(query)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&pin)) }()
	return e.core.Query(query)
}

func (e *EngineBase) QueryFromLevel(q ast.ASTHandle, level uint32) (lbool.LBool, error) {
	if x := e.open(); x != nil {
		return lbool.Undef, x
	}
	if capability, ok := e.core.(EngineLevelQuery); ok {
		return capability.QueryFromLevel(q, level)
	}
	return lbool.Undef, e.unsupported("query from level")
}

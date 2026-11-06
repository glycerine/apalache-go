package api

// Source: src/api/api_datalog.cpp:277-327.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (f *APIFixedpoint) Query(declaration ast.ASTHandle) (lbool.LBool, error) {
	if err := f.open(); err != nil {
		return lbool.Undef, err
	}
	f.hasLastQuery = false
	f.hasLastExpression = false
	f.hasLastRelations = false
	status, err := f.executeQuery(true, func(p *uparams.ParamsRef) (lbool.LBool, error) {
		return f.command.QueryFixedpointWithParams(declaration, p)
	})
	if err == nil {
		f.lastQuery, f.hasLastQuery = declaration, true
	}
	return status, err
}

// QueryExpression evaluates a Boolean fixedpoint query.
func (f *APIFixedpoint) QueryExpression(query ast.ASTHandle) (lbool.LBool, error) {
	if err := f.open(); err != nil {
		return lbool.Undef, err
	}
	f.hasLastQuery = false
	f.hasLastExpression = false
	f.hasLastRelations = false
	status, err := f.executeQuery(true, func(p *uparams.ParamsRef) (lbool.LBool, error) {
		return f.command.QueryFixedpointExpressionWithParams(query, p)
	})
	if err != nil {
		return status, err
	}
	decl, found, err := f.nullaryRelation(query)
	if err != nil {
		return status, err
	}
	if found {
		f.lastQuery, f.hasLastQuery = decl, true
	} else {
		f.hasLastExpression = true
	}
	return status, nil
}

func (f *APIFixedpoint) nullaryRelation(query ast.ASTHandle) (ast.ASTHandle, bool, error) {
	manager, err := f.command.Manager()
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	kind, err := manager.GetKind(query)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if kind != ast.ASTApp {
		return ast.ASTHandle{}, false, nil
	}
	arity, err := manager.GetNumArgs(query)
	if err != nil || arity != 0 {
		return ast.ASTHandle{}, false, err
	}
	decl, err := manager.GetDecl(query)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	predicates, err := f.command.FixedpointPredicates()
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	for _, predicate := range predicates {
		if predicate == decl {
			return decl, true, nil
		}
	}
	return ast.ASTHandle{}, false, nil
}

// QueryExpressionFromLevel runs a Boolean query at a Spacer proof level.
func (f *APIFixedpoint) QueryExpressionFromLevel(query ast.ASTHandle, level uint32) (lbool.LBool, error) {
	if err := f.open(); err != nil {
		return lbool.Undef, err
	}
	f.hasLastQuery = false
	f.hasLastExpression = false
	f.hasLastRelations = false
	declaration, found, err := f.nullaryRelation(query)
	if err != nil {
		return lbool.Undef, err
	}
	if !found {
		status, err := f.executeQuery(true, func(p *uparams.ParamsRef) (lbool.LBool, error) {
			return f.command.QueryFixedpointExpressionFromLevelWithParams(query, level, p)
		})
		if err == nil {
			f.hasLastExpression = true
		}
		return status, err
	}
	status, err := f.executeQuery(true, func(p *uparams.ParamsRef) (lbool.LBool, error) {
		return f.command.QueryFixedpointFromLevelWithParams(declaration, level, p)
	})
	if err == nil {
		f.lastQuery, f.hasLastQuery = declaration, true
	}
	return status, err
}

func (f *APIFixedpoint) QueryRelations(declarations []ast.ASTHandle) (lbool.LBool, error) {
	if err := f.open(); err != nil {
		return lbool.Undef, err
	}
	f.hasLastQuery = false
	f.hasLastExpression = false
	f.hasLastRelations = false
	status, err := f.executeQuery(false, func(p *uparams.ParamsRef) (lbool.LBool, error) {
		return f.command.QueryFixedpointRelationsWithParams(declarations, p)
	})
	if err == nil {
		if len(declarations) == 1 {
			f.lastQuery, f.hasLastQuery = declarations[0], true
		} else {
			f.hasLastRelations = true
		}
	}
	return status, err
}

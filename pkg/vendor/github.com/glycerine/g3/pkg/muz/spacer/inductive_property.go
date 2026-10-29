package spacer

// Source: inductive_property in src/muz/spacer/spacer_manager.h and
// src/muz/spacer/spacer_manager.cpp.

import (
	"bytes"
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// RelationInfo is one predicate interpretation in an inductive certificate.
// Variables are the nullary signature declarations occurring in Body.
type RelationInfo struct {
	Predicate ast.ASTHandle
	Variables []ast.ASTHandle
	Body      ast.ASTHandle
}

// InductiveProperty owns the relation roots used to materialize a Spacer
// certificate as either a model or a formula.
type InductiveProperty struct {
	manager   *ast.ASTManager
	relations []RelationInfo
	roots     []ast.ASTHandle
	closed    bool
}

func NewInductiveProperty(manager *ast.ASTManager, relations []RelationInfo) (_ *InductiveProperty, resultErr error) {
	if manager == nil {
		return nil, ErrManagerOwner
	}
	property := &InductiveProperty{manager: manager}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, property.Close())
		}
	}()
	for _, relation := range relations {
		arity, err := manager.GetArity(relation.Predicate)
		if err != nil || int(arity) != len(relation.Variables) {
			if err == nil {
				err = ast.ErrASTDomain
			}
			return nil, err
		}
		copyRelation := RelationInfo{Predicate: relation.Predicate, Body: relation.Body, Variables: append([]ast.ASTHandle(nil), relation.Variables...)}
		toRoot := append([]ast.ASTHandle{copyRelation.Predicate}, copyRelation.Variables...)
		toRoot = append(toRoot, copyRelation.Body)
		for _, handle := range toRoot {
			if err = manager.IncRef(handle); err != nil {
				return nil, err
			}
			property.roots = append(property.roots, handle)
		}
		property.relations = append(property.relations, copyRelation)
	}
	return property, nil
}

func (p *InductiveProperty) open() error {
	if p == nil || p.closed || p.manager == nil {
		return ErrManagerOwner
	}
	return nil
}

func (p *InductiveProperty) fixupClauses(body ast.ASTHandle) (_ ast.ASTRef, resultErr error) {
	conjuncts, err := p.manager.FlattenAnd([]ast.ASTHandle{body})
	if err != nil {
		return ast.ASTRef{}, err
	}
	clauses := make([]ast.ASTRef, 0, len(conjuncts))
	defer func() {
		for index := len(clauses) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&clauses[index]))
		}
	}()
	for _, conjunct := range conjuncts {
		disjuncts, err := p.manager.FlattenOr([]ast.ASTHandle{conjunct})
		if err != nil {
			return ast.ASTRef{}, err
		}
		clause, err := p.manager.MkOrArray(disjuncts)
		if err != nil {
			return ast.ASTRef{}, err
		}
		clauseRef, err := p.manager.NewRef(clause)
		if err != nil {
			return ast.ASTRef{}, err
		}
		clauses = append(clauses, clauseRef)
	}
	handles := make([]ast.ASTHandle, len(clauses))
	for index := range clauses {
		handles[index], err = p.manager.RefGet(clauses[index])
		if err != nil {
			return ast.ASTRef{}, err
		}
	}
	result, err := p.manager.MkAndArray(handles)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return p.manager.NewRef(result)
}

// Model materializes the source predicate interpretations. The caller closes
// the returned model.
func (p *InductiveProperty) Model() (result *model.Model, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	result, err := model.NewModel(p.manager)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	abstractor, err := ast.NewExprAbstractor(p.manager)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, abstractor.Close()) }()
	for _, relation := range p.relations {
		property, err := p.fixupClauses(relation.Body)
		if err != nil {
			return nil, err
		}
		constants := make([]ast.ASTRef, 0, len(relation.Variables))
		cleanup := func() (cleanupErr error) {
			for index := len(constants) - 1; index >= 0; index-- {
				cleanupErr = errors.Join(cleanupErr, p.manager.DelRef(&constants[index]))
			}
			cleanupErr = errors.Join(cleanupErr, p.manager.DelRef(&property))
			return cleanupErr
		}
		arguments := make([]ast.ASTHandle, len(relation.Variables))
		for index := range relation.Variables {
			constant, x := p.manager.MkApp(relation.Variables[len(relation.Variables)-index-1], nil)
			if x == nil {
				var reference ast.ASTRef
				reference, x = p.manager.NewRef(constant)
				if x == nil {
					constants = append(constants, reference)
					arguments[index] = constant
				}
			}
			if x != nil {
				return nil, errors.Join(x, cleanup())
			}
		}
		body, x := p.manager.RefGet(property)
		var abstracted ast.ASTRef
		if x == nil {
			abstracted, x = p.manager.NewRef(ast.ASTHandle{})
		}
		if x == nil {
			x = abstractor.Abstract(0, arguments, body, abstracted)
		}
		var interpretation *model.FunctionInterpretation
		if x == nil {
			body, x = p.manager.RefGet(abstracted)
		}
		if x == nil && len(arguments) == 0 {
			x = result.Core().RegisterConstant(relation.Predicate, body)
		} else if x == nil {
			interpretation, x = model.NewFunctionInterpretation(p.manager, uint32(len(arguments)))
			if x == nil {
				x = interpretation.SetElse(body)
			}
			if x == nil {
				x = result.Core().RegisterFunction(relation.Predicate, interpretation)
				interpretation = nil
			}
		}
		if interpretation != nil {
			x = errors.Join(x, interpretation.Close())
		}
		x = errors.Join(x, p.manager.DelRef(&abstracted), cleanup())
		if x != nil {
			return nil, x
		}
	}
	return result, nil
}

// Expr exports the model as the conjunction of predicate definitions.
func (p *InductiveProperty) Expr() (result ast.ASTRef, resultErr error) {
	md, err := p.Model()
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, md.Close()) }()
	builder, err := model.NewModel2ExprBuilder(md.Core())
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, builder.Close()) }()
	result, err = p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	if err = builder.Build(result); err != nil {
		return result, errors.Join(err, p.manager.DelRef(&result))
	}
	return result, nil
}

func (p *InductiveProperty) WriteSMT2(out io.Writer, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (resultErr error) {
	md, err := p.Model()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, md.Close()) }()
	return md.Core().WriteSMT2(out, context, globals, 0)
}

func (p *InductiveProperty) SMT2(context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (string, error) {
	var out bytes.Buffer
	if err := p.WriteSMT2(&out, context, globals); err != nil {
		return "", err
	}
	return out.String(), nil
}

func (p *InductiveProperty) Close() (result error) {
	if p == nil || p.manager == nil {
		return ErrManagerOwner
	}
	if p.closed {
		return nil
	}
	for index := len(p.roots) - 1; index >= 0; index-- {
		result = errors.Join(result, p.manager.DecRef(p.roots[index]))
	}
	if result == nil {
		clear(p.roots)
		p.roots = nil
		clear(p.relations)
		p.relations = nil
		p.closed = true
	}
	return result
}

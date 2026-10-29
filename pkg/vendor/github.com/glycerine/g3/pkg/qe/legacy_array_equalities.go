package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// array_project_eqs_util::find_arr_eqs.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type legacyArrayProjection struct {
	syntax *ArrayProjection
	theory *rewriter.TheoryRewriter
}

type legacyArrayEqualities struct {
	manager    *ast.ASTManager
	equalities []ast.ASTRef
	stores     map[ast.ASTHandle]bool
}

func newLegacyArrayProjection(manager *ast.ASTManager, config Config) (*legacyArrayProjection, error) {
	if _, err := manager.RegisterArrayPlugin(); err != nil {
		return nil, err
	}
	syntax, err := NewArrayProjection(manager, config)
	if err != nil {
		return nil, err
	}
	theory, err := rewriter.NewTheoryRewriter(manager, config.Context, config.Globals, nil)
	if err != nil {
		return nil, errors.Join(err, syntax.Close())
	}
	return &legacyArrayProjection{syntax: syntax, theory: theory}, nil
}

func (p *legacyArrayProjection) Close() error {
	if p == nil || p.syntax == nil {
		return ast.ErrASTHandle
	}
	err := p.theory.Close()
	if err == nil {
		p.theory = nil
		err = p.syntax.Close()
	}
	if err == nil {
		p.syntax = nil
	}
	return err
}

func (r *legacyArrayEqualities) Close() (result error) {
	if r == nil || r.manager == nil {
		return ast.ErrASTHandle
	}
	for index := len(r.equalities) - 1; index >= 0; index-- {
		result = errors.Join(result, r.manager.DelRef(&r.equalities[index]))
	}
	r.equalities = nil
	if result == nil {
		r.manager = nil
		r.stores = nil
	}
	return result
}

func (p *legacyArrayProjection) arrayExpression(expression ast.ASTHandle) (bool, error) {
	sort, err := p.syntax.manager.GetSort(expression)
	if err != nil {
		return false, err
	}
	family, err := p.syntax.manager.GetFamilyID(sort)
	return family == p.syntax.family, err
}

func (p *legacyArrayProjection) findEqualities(variable, formula ast.ASTHandle) (_ *legacyArrayEqualities, resultErr error) {
	if p == nil || p.syntax == nil {
		return nil, ast.ErrASTHandle
	}
	result := &legacyArrayEqualities{manager: p.syntax.manager, stores: make(map[ast.ASTHandle]bool)}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	done := make(map[ast.ASTHandle]bool)
	var visit func(ast.ASTHandle) error
	visit = func(current ast.ASTHandle) error {
		if done[current] {
			return nil
		}
		kind, err := p.syntax.manager.GetKind(current)
		if err != nil {
			return err
		}
		if kind != ast.ASTApp {
			done[current] = true
			return nil
		}
		count, err := p.syntax.manager.GetNumArgs(current)
		if err != nil {
			return err
		}
		arguments := make([]ast.ASTHandle, count)
		argumentsHaveStores := false
		for index := range arguments {
			arguments[index], err = p.syntax.manager.GetArg(current, uint32(index))
			if err != nil {
				return err
			}
			if err = visit(arguments[index]); err != nil {
				return err
			}
			argumentsHaveStores = argumentsHaveStores || result.stores[arguments[index]]
		}
		_, selectTerm, err := p.syntax.operation(current, p.syntax.family, ast.ASTArrayOpSelect)
		if err != nil {
			return err
		}
		storeArguments, storeTerm, err := p.syntax.operation(current, p.syntax.family, ast.ASTArrayOpStore)
		if err != nil {
			return err
		}
		if (!selectTerm && argumentsHaveStores) || (storeTerm && len(storeArguments) != 0 && storeArguments[0] == variable) {
			result.stores[current] = true
		}
		equalityArguments, equality, err := p.syntax.operation(current, p.syntax.manager.BasicFamilyID(), ast.ASTOpEq)
		if err != nil {
			return err
		}
		if equality && len(equalityArguments) == 2 {
			array, err := p.arrayExpression(equalityArguments[0])
			if err != nil {
				return err
			}
			if equalityArguments[0] == variable || equalityArguments[1] == variable || (array && result.stores[current]) {
				reference, err := p.syntax.manager.NewRef(current)
				if err != nil {
					return err
				}
				result.equalities = append(result.equalities, reference)
			}
		}
		done[current] = true
		return nil
	}
	if err := visit(formula); err != nil {
		return nil, err
	}
	return result, nil
}

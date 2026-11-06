package smtcore

// Source: src/smt/smt_model_finder.cpp collect_partial_funcs,
// complete_partial_funcs and auf_solver::fix_model.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func (g *smtModelFinderAUFGraph) collectPartialFunctions() ([]ast.ASTHandle, error) {
	seen := make(map[ast.ASTHandle]bool)
	var functions []ast.ASTHandle
	for key := range g.arguments {
		function := key.owner
		if seen[function] {
			continue
		}
		seen[function] = true
		interpretation, err := g.model.Core().FunctionInterpretation(function)
		if err != nil {
			return nil, err
		}
		if interpretation == nil {
			arity, err := g.finder.manager.GetArity(function)
			if err != nil {
				return nil, err
			}
			interpretation, err = model.NewFunctionInterpretation(g.finder.manager, arity)
			if err != nil {
				return nil, err
			}
			if err = g.model.Core().RegisterFunction(function, interpretation); err != nil {
				return nil, errors.Join(err, interpretation.Close())
			}
		}
		partial, err := interpretation.IsPartial()
		if err != nil {
			return nil, err
		}
		if partial {
			functions = append(functions, function)
		}
	}
	return functions, nil
}

func (g *smtModelFinderAUFGraph) completeProjectedFunctions(functions []ast.ASTHandle) error {
	m := g.finder.manager
	for _, function := range functions {
		if err := g.model.CompletePartialFunction(function); err != nil {
			return err
		}
		interpretation, err := g.model.Core().FunctionInterpretation(function)
		if err != nil {
			return err
		}
		constant, err := interpretation.IsConstant()
		if err != nil {
			return err
		}
		if constant {
			continue
		}
		arity, err := m.GetArity(function)
		if err != nil {
			return err
		}
		domain := make([]ast.ASTHandle, arity)
		arguments := make([]ast.ASTHandle, arity)
		hasProjection := false
		for i := uint32(0); i < arity; i++ {
			domain[i], err = m.GetDomain(function, i)
			if err != nil {
				return err
			}
			variable, err := m.MkVar(i, domain[i])
			if err != nil {
				return err
			}
			arguments[i] = variable
			if node := g.arguments[smtModelFinderAUFKey{function, i}]; node != nil {
				projection := node.root().projection
				if !projection.IsNull() {
					arguments[i], err = m.MkApp(projection, []ast.ASTHandle{variable})
					if err != nil {
						return err
					}
					hasProjection = true
				}
			}
		}
		if !hasProjection {
			continue
		}
		name, err := m.GetASTName(function)
		if err != nil {
			return err
		}
		rangeSort, err := m.GetRange(function)
		if err != nil {
			return err
		}
		auxiliary, err := m.MkFreshFuncDecl(name, symbol.Symbol{}, domain, rangeSort)
		if err != nil {
			return err
		}
		definition, err := m.MkApp(auxiliary, arguments)
		if err != nil {
			return err
		}
		replacement, err := model.NewFunctionInterpretation(m, arity)
		if err != nil {
			return err
		}
		if err = replacement.SetElse(definition); err != nil {
			return errors.Join(err, replacement.Close())
		}
		if err = g.model.ReregisterFunction(function, replacement, auxiliary); err != nil {
			return errors.Join(err, replacement.Close())
		}
	}
	return nil
}

func (g *smtModelFinderAUFGraph) extendModel() error {
	if err := g.cleanInstantiationSets(); err != nil {
		return err
	}
	partial, err := g.collectPartialFunctions()
	if err != nil {
		return err
	}
	if err = g.fillEmptyInstantiationSets(); err != nil {
		return err
	}
	if err = g.freezeUniverses(); err != nil {
		return err
	}
	if err = g.addMonotoneExceptions(); err != nil {
		return err
	}
	for _, node := range g.nodes {
		if err := g.finder.checkpoint(); err != nil {
			return err
		}
		if node.parent != nil {
			continue
		}
		if node.mono {
			err = g.makeMonotoneProjection(node)
		} else {
			err = g.makeSimpleProjection(node)
		}
		if err != nil {
			return err
		}
	}
	for _, node := range g.nodes {
		if node.parent == nil {
			if err = node.set.MakeInverse(g.model); err != nil {
				return err
			}
		}
	}
	return g.completeProjectedFunctions(partial)
}

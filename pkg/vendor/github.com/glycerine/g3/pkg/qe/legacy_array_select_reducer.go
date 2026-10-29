package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// array_select_reducer.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

type legacyArrayFormula struct {
	manager *ast.ASTManager
	formula ast.ASTRef
}

func (r *legacyArrayFormula) Close() error {
	if r == nil || r.manager == nil {
		return ast.ErrASTHandle
	}
	err := r.manager.DelRef(&r.formula)
	if err == nil {
		r.manager = nil
	}
	return err
}

func (p *legacyArrayProjection) rewrite(expression ast.ASTHandle) (_ ast.ASTRef, resultErr error) {
	result, err := p.syntax.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	if err = p.theory.Rewrite(expression, result); err != nil {
		return ast.ASTRef{}, errors.Join(err, p.syntax.manager.DelRef(&result))
	}
	return result, nil
}

func (p *legacyArrayProjection) reduceSelects(md *model.ModelCore, variables []ast.ASTHandle, formula ast.ASTHandle, reduceAll bool) (result *legacyArrayFormula, resultErr error) {
	if p == nil || p.syntax == nil || p.theory == nil || md == nil || md.Manager() != p.syntax.manager {
		return nil, ast.ErrASTHandle
	}
	defer func() {
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	if !reduceAll && len(variables) == 0 {
		ref, err := p.syntax.manager.NewRef(formula)
		if err != nil {
			return nil, err
		}
		return &legacyArrayFormula{manager: p.syntax.manager, formula: ref}, nil
	}
	evaluator, err := model.NewModelEvaluator(md, model.ModelEvaluatorConfig{
		Context: p.syntax.config.Context, Globals: p.syntax.config.Globals,
		Memory: p.syntax.manager.NumeralService().RuntimeOwner(), Cooperation: p.syntax.config.Cooperation, Trace: p.syntax.config.Trace,
	})
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	if err = evaluator.SetModelCompletion(true); err != nil {
		return nil, err
	}
	markedVariables := make(map[ast.ASTHandle]bool)
	for _, variable := range variables {
		markedVariables[variable] = true
	}
	type reducedNode struct {
		handle    ast.ASTHandle
		hasStores bool
	}
	cache := make(map[ast.ASTHandle]reducedNode)
	conditions := make([]ast.ASTRef, 0)
	defer func() {
		for index := len(conditions) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, p.syntax.manager.DelRef(&conditions[index]))
		}
	}()
	equalInModel := func(left, right ast.ASTHandle) (_ bool, evalErr error) {
		if left == right {
			return true, nil
		}
		leftValue, err := evaluator.EvaluateValue(left)
		if err != nil {
			return false, err
		}
		defer func() { evalErr = errors.Join(evalErr, p.syntax.manager.DelRef(&leftValue)) }()
		rightValue, err := evaluator.EvaluateValue(right)
		if err != nil {
			return false, err
		}
		defer func() { evalErr = errors.Join(evalErr, p.syntax.manager.DelRef(&rightValue)) }()
		leftHandle, err := p.syntax.manager.RefGet(leftValue)
		if err != nil {
			return false, err
		}
		rightHandle, err := p.syntax.manager.RefGet(rightValue)
		if err != nil {
			return false, err
		}
		return p.syntax.manager.AreEqual(leftHandle, rightHandle)
	}
	addCondition := func(condition ast.ASTHandle) error {
		rewritten, err := p.rewrite(condition)
		if err != nil {
			return err
		}
		handle, err := p.syntax.manager.RefGet(rewritten)
		if err != nil {
			return errors.Join(err, p.syntax.manager.DelRef(&rewritten))
		}
		if !p.syntax.manager.IsTrue(handle) {
			conditions = append(conditions, rewritten)
		} else {
			err = p.syntax.manager.DelRef(&rewritten)
		}
		return err
	}
	var reduce func(ast.ASTHandle) (reducedNode, error)
	reduce = func(current ast.ASTHandle) (reducedNode, error) {
		if cached, ok := cache[current]; ok {
			return cached, nil
		}
		kind, err := p.syntax.manager.GetKind(current)
		if err != nil || kind != ast.ASTApp {
			return reducedNode{handle: current}, err
		}
		count, err := p.syntax.manager.GetNumArgs(current)
		if err != nil {
			return reducedNode{}, err
		}
		children := make([]ast.ASTHandle, count)
		argumentsHaveStores := false
		changed := false
		for index := range children {
			original, err := p.syntax.manager.GetArg(current, uint32(index))
			if err != nil {
				return reducedNode{}, err
			}
			child, err := reduce(original)
			if err != nil {
				return reducedNode{}, err
			}
			children[index] = child.handle
			changed = changed || child.handle != original
			argumentsHaveStores = argumentsHaveStores || child.hasStores
		}
		rebuilt := current
		if changed {
			decl, err := p.syntax.manager.GetDecl(current)
			if err != nil {
				return reducedNode{}, err
			}
			rebuilt, err = p.syntax.manager.MkApp(decl, children)
			if err != nil {
				return reducedNode{}, err
			}
		}
		selectArgs, selectTerm, err := p.syntax.operation(rebuilt, p.syntax.family, ast.ASTArrayOpSelect)
		if err != nil {
			return reducedNode{}, err
		}
		if selectTerm && len(selectArgs) == 2 && (reduceAll || argumentsHaveStores) {
			array, index := selectArgs[0], selectArgs[1]
			for {
				storeArgs, storeTerm, err := p.syntax.operation(array, p.syntax.family, ast.ASTArrayOpStore)
				if err != nil {
					return reducedNode{}, err
				}
				if !storeTerm {
					break
				}
				if len(storeArgs) != 3 {
					return reducedNode{}, ast.ErrASTDomain
				}
				equal, err := equalInModel(storeArgs[1], index)
				if err != nil {
					return reducedNode{}, err
				}
				condition, err := p.syntax.manager.MkEq(storeArgs[1], index)
				if err != nil {
					return reducedNode{}, err
				}
				if equal {
					if err = addCondition(condition); err != nil {
						return reducedNode{}, err
					}
					node := reducedNode{handle: storeArgs[2]}
					cache[current] = node
					return node, nil
				}
				condition, err = p.syntax.manager.MkNot(condition)
				if err != nil {
					return reducedNode{}, err
				}
				if err = addCondition(condition); err != nil {
					return reducedNode{}, err
				}
				array = storeArgs[0]
			}
			rebuilt, err = p.syntax.manager.MkFamilyApp(p.syntax.family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{array, index})
			if err != nil {
				return reducedNode{}, err
			}
		}
		_, storeTerm, err := p.syntax.operation(rebuilt, p.syntax.family, ast.ASTArrayOpStore)
		if err != nil {
			return reducedNode{}, err
		}
		node := reducedNode{handle: rebuilt, hasStores: reduceAll || argumentsHaveStores || (storeTerm && len(children) != 0 && markedVariables[children[0]])}
		cache[current] = node
		return node, nil
	}
	reduced, err := reduce(formula)
	if err != nil {
		return nil, err
	}
	conjuncts := make([]ast.ASTHandle, 0, len(conditions)+1)
	for _, condition := range conditions {
		handle, err := p.syntax.manager.RefGet(condition)
		if err != nil {
			return nil, err
		}
		conjuncts = append(conjuncts, handle)
	}
	conjuncts = append(conjuncts, reduced.handle)
	combined, err := p.syntax.manager.MkAndArray(conjuncts)
	if err != nil {
		return nil, err
	}
	final, err := p.rewrite(combined)
	if err != nil {
		return nil, err
	}
	result = &legacyArrayFormula{manager: p.syntax.manager, formula: final}
	return result, nil
}

package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// array_project_eqs_util::find_subst_term/convert_peq_to_eq store path.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

func (p *legacyArrayProjection) projectStoreEquality(md *model.ModelCore, variable, formula ast.ASTHandle) (result *legacyArraySelectProjection, eliminated bool, resultErr error) {
	if p == nil || p.syntax == nil || md == nil || md.Manager() != p.syntax.manager {
		return nil, false, ast.ErrASTHandle
	}
	result = &legacyArraySelectProjection{manager: p.syntax.manager}
	defer func() {
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
			eliminated = false
		}
	}()
	equalities, err := p.findEqualities(variable, formula)
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, equalities.Close()) }()
	evaluator, err := model.NewModelEvaluator(md, model.ModelEvaluatorConfig{
		Context: p.syntax.config.Context, Globals: p.syntax.config.Globals,
		Memory: p.syntax.manager.NumeralService().RuntimeOwner(), Cooperation: p.syntax.config.Cooperation, Trace: p.syntax.config.Trace,
	})
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	if err = evaluator.SetModelCompletion(true); err != nil {
		return nil, false, err
	}
	selected := ast.ASTHandle{}
	selectedDepth := int(^uint(0) >> 1)
	falseEqualities := make([]ast.ASTHandle, 0)
	for _, equalityRef := range equalities.equalities {
		equality, err := p.syntax.manager.RefGet(equalityRef)
		if err != nil {
			return nil, false, err
		}
		value, err := evaluator.EvaluateValue(equality)
		if err != nil {
			return nil, false, err
		}
		valueHandle, err := p.syntax.manager.RefGet(value)
		if err != nil {
			_ = p.syntax.manager.DelRef(&value)
			return nil, false, err
		}
		isFalse := p.syntax.manager.IsFalse(valueHandle)
		if err = p.syntax.manager.DelRef(&value); err != nil {
			return nil, false, err
		}
		if isFalse {
			falseEqualities = append(falseEqualities, equality)
			continue
		}
		args, matched, err := p.syntax.operation(equality, p.syntax.manager.BasicFamilyID(), ast.ASTOpEq)
		if err != nil || !matched || len(args) != 2 {
			continue
		}
		depth := func(root ast.ASTHandle) int {
			if !equalities.stores[root] {
				return 0
			}
			depth := 1
			for {
				args, store, _ := p.syntax.operation(root, p.syntax.family, ast.ASTArrayOpStore)
				if !store || len(args) != 3 {
					break
				}
				root = args[0]
				if root == variable {
					return depth + 1
				}
				depth++
			}
			return 0
		}
		leftDepth, rightDepth := depth(args[0]), depth(args[1])
		candidateDepth := leftDepth
		if candidateDepth == 0 || rightDepth != 0 && rightDepth < candidateDepth {
			candidateDepth = rightDepth
		}
		if candidateDepth > 1 && candidateDepth < selectedDepth {
			selected, selectedDepth = equality, candidateDepth
		}
	}
	if selected == (ast.ASTHandle{}) {
		_ = result.Close()
		result = nil
		return nil, false, nil
	}
	args, _, err := p.syntax.operation(selected, p.syntax.manager.BasicFamilyID(), ast.ASTOpEq)
	if err != nil {
		return nil, false, err
	}
	lhs, rhs := args[0], args[1]
	if !equalities.stores[lhs] {
		lhs, rhs = rhs, lhs
	}
	indices := make([]ast.ASTHandle, 0)
	indexConditions := make([]ast.ASTHandle, 0)
	auxiliary := make([]ast.ASTHandle, 0)
	evaluate := func(expression ast.ASTHandle) (_ ast.ASTRef, evalErr error) {
		return evaluator.EvaluateValue(expression)
	}
	for equalities.stores[lhs] {
		store, matched, err := p.syntax.operation(lhs, p.syntax.family, ast.ASTArrayOpStore)
		if err != nil || !matched || len(store) != 3 {
			return nil, false, errors.Join(err, ast.ErrASTDomain)
		}
		base, index, storedValue := store[0], store[1], store[2]
		indexValue, err := evaluate(index)
		if err != nil {
			return nil, false, err
		}
		indexValueHandle, err := p.syntax.manager.RefGet(indexValue)
		if err != nil {
			_ = p.syntax.manager.DelRef(&indexValue)
			return nil, false, err
		}
		matchedIndex := -1
		for candidate, previous := range indices {
			previousValue, err := evaluate(previous)
			if err != nil {
				_ = p.syntax.manager.DelRef(&indexValue)
				return nil, false, err
			}
			previousHandle, err := p.syntax.manager.RefGet(previousValue)
			if err == nil {
				var equal bool
				equal, err = p.syntax.manager.AreEqual(indexValueHandle, previousHandle)
				if equal {
					matchedIndex = candidate
				}
			}
			err = errors.Join(err, p.syntax.manager.DelRef(&previousValue))
			if err != nil {
				_ = p.syntax.manager.DelRef(&indexValue)
				return nil, false, err
			}
			if matchedIndex >= 0 {
				break
			}
		}
		if err = p.syntax.manager.DelRef(&indexValue); err != nil {
			return nil, false, err
		}
		if matchedIndex >= 0 {
			condition, err := p.syntax.manager.MkEq(index, indices[matchedIndex])
			if err != nil {
				return nil, false, err
			}
			indexConditions = append(indexConditions, condition)
		} else {
			for _, previous := range indices {
				condition, err := p.syntax.manager.MkEq(index, previous)
				if err == nil {
					condition, err = p.syntax.manager.MkNot(condition)
				}
				if err != nil {
					return nil, false, err
				}
				indexConditions = append(indexConditions, condition)
			}
			indices = append(indices, index)
			selectTerm, err := p.syntax.manager.MkFamilyApp(p.syntax.family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{rhs, index})
			if err != nil {
				return nil, false, err
			}
			valueEquality, err := p.syntax.manager.MkEq(selectTerm, storedValue)
			if err != nil {
				return nil, false, err
			}
			auxiliary = append(auxiliary, valueEquality)
		}
		lhs = base
	}
	if lhs != variable && rhs != variable {
		_ = result.Close()
		result = nil
		return nil, false, nil
	}
	partial, err := newLegacyPartialEquality(p.syntax.manager, p.syntax.family, lhs, rhs, indices)
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, partial.Close()) }()
	factored, err := p.factorPartialSelects(md, evaluator, variable, partial)
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, factored.Close()) }()
	for _, variableRef := range factored.variables {
		handle, err := p.syntax.manager.RefGet(variableRef)
		if err != nil {
			return nil, false, err
		}
		copy, err := p.syntax.manager.NewRef(handle)
		if err != nil {
			return nil, false, err
		}
		result.variables = append(result.variables, copy)
	}
	for _, equalityRef := range factored.equalities {
		handle, err := p.syntax.manager.RefGet(equalityRef)
		if err != nil {
			return nil, false, err
		}
		auxiliary = append(auxiliary, handle)
	}
	factoredLHS, err := p.syntax.manager.RefGet(factored.partial.lhs)
	if err != nil {
		return nil, false, err
	}
	factoredRHS, err := p.syntax.manager.RefGet(factored.partial.rhs)
	if err != nil {
		return nil, false, err
	}
	factoredIndices := make([]ast.ASTHandle, 0, len(factored.partial.indices))
	for _, indexRef := range factored.partial.indices {
		handle, err := p.syntax.manager.RefGet(indexRef)
		if err != nil {
			return nil, false, err
		}
		factoredIndices = append(factoredIndices, handle)
	}
	expansion, err := factored.partial.Expand(factoredRHS != variable)
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, expansion.Close()) }()
	for index, constantRef := range expansion.constants {
		constant, err := p.syntax.manager.RefGet(constantRef)
		if err != nil {
			return nil, false, err
		}
		constantCopy, err := p.syntax.manager.NewRef(constant)
		if err != nil {
			return nil, false, err
		}
		result.variables = append(result.variables, constantCopy)
		selectTerm, err := p.syntax.manager.MkFamilyApp(p.syntax.family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{factoredLHS, factoredIndices[index]})
		if err != nil {
			return nil, false, err
		}
		value, err := evaluator.EvaluateValue(selectTerm)
		if err != nil {
			return nil, false, err
		}
		valueHandle, err := p.syntax.manager.RefGet(value)
		if err == nil {
			var decl ast.ASTHandle
			decl, err = p.syntax.manager.GetDecl(constant)
			if err == nil {
				err = md.RegisterConstant(decl, valueHandle)
			}
		}
		err = errors.Join(err, p.syntax.manager.DelRef(&value))
		if err != nil {
			return nil, false, err
		}
	}
	expandedEquality, err := p.syntax.manager.RefGet(expansion.equality)
	if err != nil {
		return nil, false, err
	}
	expandedArgs, matched, err := p.syntax.operation(expandedEquality, p.syntax.manager.BasicFamilyID(), ast.ASTOpEq)
	if err != nil || !matched || len(expandedArgs) != 2 || expandedArgs[0] != variable {
		return nil, false, errors.Join(err, ast.ErrASTDomain)
	}
	replace, err := rewriter.NewExprSafeReplace(p.syntax.manager)
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, replace.Close()) }()
	truth, err := p.syntax.manager.MkTrue()
	if err != nil {
		return nil, false, err
	}
	falsity, err := p.syntax.manager.MkFalse()
	if err != nil {
		return nil, false, err
	}
	for _, equality := range falseEqualities {
		if err = replace.Insert(equality, falsity); err != nil {
			return nil, false, err
		}
	}
	if err = replace.Insert(selected, truth); err != nil {
		return nil, false, err
	}
	if err = replace.Insert(variable, expandedArgs[1]); err != nil {
		return nil, false, err
	}
	conjuncts := append(indexConditions, auxiliary...)
	conjuncts = append(conjuncts, formula)
	combined, err := p.syntax.manager.MkAndArray(conjuncts)
	if err != nil {
		return nil, false, err
	}
	replaced, err := p.syntax.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, false, err
	}
	if err = replace.Apply(combined, replaced); err != nil {
		return nil, false, errors.Join(err, p.syntax.manager.DelRef(&replaced))
	}
	replacedHandle, err := p.syntax.manager.RefGet(replaced)
	if err != nil {
		return nil, false, errors.Join(err, p.syntax.manager.DelRef(&replaced))
	}
	result.formula, err = p.rewrite(replacedHandle)
	err = errors.Join(err, p.syntax.manager.DelRef(&replaced))
	if err != nil {
		return nil, false, err
	}
	return result, true, nil
}

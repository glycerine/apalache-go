package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// array_project_selects_util.

import (
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

type legacyArraySelectProjection struct {
	manager   *ast.ASTManager
	formula   ast.ASTRef
	variables []ast.ASTRef
}

func (r *legacyArraySelectProjection) Close() (result error) {
	if r == nil || r.manager == nil {
		return ast.ErrASTHandle
	}
	// Refuse cleanup before discarding any roots when manager entry is pending.
	if _, err := r.manager.ProofMode(); err != nil {
		return err
	}
	for len(r.variables) != 0 {
		index := len(r.variables) - 1
		if r.variables[index] == (ast.ASTRef{}) {
			r.variables = r.variables[:index]
			continue
		}
		err := r.manager.DelRef(&r.variables[index])
		if r.variables[index] == (ast.ASTRef{}) {
			r.variables = r.variables[:index]
		}
		if err != nil {
			return err
		}
	}
	if r.formula != (ast.ASTRef{}) {
		if err := r.manager.DelRef(&r.formula); err != nil {
			return err
		}
	}
	r.manager = nil
	return result
}

func (p *legacyArrayProjection) projectSelects(md *model.ModelCore, variables []ast.ASTHandle, formula ast.ASTHandle) (result *legacyArraySelectProjection, resultErr error) {
	if p == nil || p.syntax == nil || md == nil || md.Manager() != p.syntax.manager {
		return nil, ast.ErrASTHandle
	}
	result = &legacyArraySelectProjection{manager: p.syntax.manager}
	ownedResult := result
	defer func() {
		if resultErr != nil {
			resultErr = closeFailedLegacyArraySelectProjection(ownedResult, resultErr)
			result = nil
		}
	}()
	evaluator, err := newLegacyModelEvaluator(md, model.ModelEvaluatorConfig{
		Context: p.syntax.config.Context, Globals: p.syntax.config.Globals,
		Memory: p.syntax.manager.NumeralService().RuntimeOwner(), Cooperation: p.syntax.config.Cooperation, Trace: p.syntax.config.Trace,
	})
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = closeFailedLegacyArrayEvaluator(p.syntax.manager, evaluator, resultErr) }()
	if err = evaluator.SetModelCompletion(true); err != nil {
		return nil, err
	}
	selected := make(map[ast.ASTHandle][]ast.ASTHandle)
	for _, variable := range variables {
		selected[variable] = nil
	}
	done := make(map[ast.ASTHandle]bool)
	var todo []ast.ASTHandle
	rootKind, err := p.syntax.manager.GetKind(formula)
	if err != nil {
		return nil, err
	}
	if rootKind == ast.ASTApp {
		todo = append(todo, formula)
	}
	for len(todo) != 0 {
		current := todo[len(todo)-1]
		if done[current] {
			todo = todo[:len(todo)-1]
			continue
		}
		count, err := p.syntax.manager.GetNumArgs(current)
		if err != nil {
			return nil, err
		}
		allDone := true
		// Source pushes pending arguments in increasing index order. The
		// last pending application is collected first by the LIFO stack.
		for index := uint32(0); index < count; index++ {
			child, err := p.syntax.manager.GetArg(current, index)
			if err != nil {
				return nil, err
			}
			kind, err := p.syntax.manager.GetKind(child)
			if err != nil {
				return nil, err
			}
			if !done[child] && kind == ast.ASTApp {
				todo = append(todo, child)
				allDone = false
			}
		}
		if !allDone {
			continue
		}
		todo = todo[:len(todo)-1]
		args, selectTerm, err := p.syntax.operation(current, p.syntax.family, ast.ASTArrayOpSelect)
		if err != nil {
			return nil, err
		}
		if selectTerm && len(args) == 2 {
			if _, ok := selected[args[0]]; ok {
				selected[args[0]] = append(selected[args[0]], current)
			}
		}
		done[current] = true
	}
	replace, err := rewriter.NewExprSafeReplace(p.syntax.manager)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = closeFailedLegacyArrayReplacer(p.syntax.manager, replace, resultErr) }()
	conditions := make([]ast.ASTRef, 0)
	values := make([]ast.ASTRef, 0)
	defer func() {
		for index := len(values) - 1; index >= 0; index-- {
			resultErr = closeFailedLegacyArrayRewrite(p.syntax.manager, values[index], resultErr)
		}
		for index := len(conditions) - 1; index >= 0; index-- {
			resultErr = closeFailedLegacyArrayRewrite(p.syntax.manager, conditions[index], resultErr)
		}
	}()
	type indexClass struct {
		representative ast.ASTHandle
		value          ast.ASTRef
		constant       ast.ASTHandle
		number         *big.Rat
	}
	for _, variable := range variables {
		terms := selected[variable]
		if len(terms) == 0 {
			continue
		}
		arraySort, err := p.syntax.manager.GetSort(variable)
		if err != nil {
			return nil, err
		}
		rangeSort, err := p.syntax.manager.ArrayRange(arraySort)
		if err != nil {
			return nil, err
		}
		classes := make([]indexClass, 0)
		for _, term := range terms {
			args, _, err := p.syntax.operation(term, p.syntax.family, ast.ASTArrayOpSelect)
			if err != nil {
				return nil, err
			}
			index := args[1]
			value, err := evaluator.EvaluateValue(index)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
			valueHandle, err := p.syntax.manager.RefGet(value)
			if err != nil {
				return nil, err
			}
			classIndex := -1
			for candidate := range classes {
				other, err := p.syntax.manager.RefGet(classes[candidate].value)
				if err != nil {
					return nil, err
				}
				equal, err := p.syntax.manager.AreEqual(valueHandle, other)
				if err != nil {
					return nil, err
				}
				if equal {
					classIndex = candidate
					break
				}
			}
			if classIndex >= 0 {
				if err = replace.Insert(term, classes[classIndex].constant); err != nil {
					return nil, err
				}
				equality, err := p.syntax.manager.MkEq(index, classes[classIndex].representative)
				if err != nil {
					return nil, err
				}
				owned, err := p.syntax.manager.NewRef(equality)
				if err != nil {
					return nil, err
				}
				conditions = append(conditions, owned)
				continue
			}
			constant, err := p.syntax.manager.MkFreshConst("sel", rangeSort)
			if err != nil {
				return nil, err
			}
			constantRef, err := p.syntax.manager.NewRef(constant)
			if err != nil {
				return nil, err
			}
			result.variables = append(result.variables, constantRef)
			if err = replace.Insert(term, constant); err != nil {
				return nil, err
			}
			termValue, err := evaluator.EvaluateValue(term)
			if err != nil {
				return nil, err
			}
			termValueHandle, err := p.syntax.manager.RefGet(termValue)
			if err == nil {
				var decl ast.ASTHandle
				decl, err = p.syntax.manager.GetDecl(constant)
				if err == nil {
					err = md.RegisterConstant(decl, termValueHandle)
				}
			}
			err = closeFailedLegacyArrayRewrite(p.syntax.manager, termValue, err)
			if err != nil {
				return nil, err
			}
			classes = append(classes, indexClass{representative: index, value: value, constant: constant})
		}
		// The source orders arithmetic indices. For other index sorts,
		// pairwise disequalities express the same model partition without
		// requiring an ordering operation on that sort.
		numeric := true
		if len(classes) > 1 {
			for i := range classes {
				value, err := p.syntax.manager.RefGet(classes[i].value)
				if err != nil {
					return nil, err
				}
				text, _, numeral, err := p.syntax.manager.ArithmeticNumeralText(value)
				if err != nil {
					return nil, err
				}
				if !numeral {
					numeric = false
					break
				}
				classes[i].number, numeral = new(big.Rat).SetString(text)
				if !numeral {
					return nil, ast.ErrASTDomain
				}
			}
		}
		if !numeric {
			for i := range classes {
				for j := 0; j < i; j++ {
					equality, err := p.syntax.manager.MkEq(classes[i].representative, classes[j].representative)
					if err != nil {
						return nil, err
					}
					inequality, err := p.syntax.manager.MkNot(equality)
					if err != nil {
						return nil, err
					}
					owned, err := p.syntax.manager.NewRef(inequality)
					if err != nil {
						return nil, err
					}
					conditions = append(conditions, owned)
				}
			}
			continue
		}
		sort.SliceStable(classes, func(left, right int) bool { return classes[left].number.Cmp(classes[right].number) < 0 })
		arithmetic, err := p.syntax.manager.ArithmeticFamilyID()
		if err != nil {
			return nil, err
		}
		for index := 0; index+1 < len(classes); index++ {
			less, err := p.syntax.manager.MkFamilyApp(arithmetic, 4, nil, []ast.ASTHandle{classes[index].representative, classes[index+1].representative})
			if err != nil {
				return nil, err
			}
			owned, err := p.syntax.manager.NewRef(less)
			if err != nil {
				return nil, err
			}
			conditions = append(conditions, owned)
		}
	}
	conjuncts := make([]ast.ASTHandle, 0, len(conditions)+1)
	for _, condition := range conditions {
		handle, err := p.syntax.manager.RefGet(condition)
		if err != nil {
			return nil, err
		}
		conjuncts = append(conjuncts, handle)
	}
	conjuncts = append(conjuncts, formula)
	combined, err := p.syntax.manager.MkAndArray(conjuncts)
	if err != nil {
		return nil, err
	}
	result.formula, err = p.syntax.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	if err = replace.Apply(combined, result.formula); err != nil {
		return nil, err
	}
	return result, nil
}

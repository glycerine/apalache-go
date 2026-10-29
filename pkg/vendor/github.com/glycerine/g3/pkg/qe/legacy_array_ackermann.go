package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// array_project_selects_util.

import (
	"errors"
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
	for index := len(r.variables) - 1; index >= 0; index-- {
		result = errors.Join(result, r.manager.DelRef(&r.variables[index]))
	}
	r.variables = nil
	if r.formula != (ast.ASTRef{}) {
		result = errors.Join(result, r.manager.DelRef(&r.formula))
	}
	if result == nil {
		r.manager = nil
	}
	return result
}

func (p *legacyArrayProjection) projectSelects(md *model.ModelCore, variables []ast.ASTHandle, formula ast.ASTHandle) (result *legacyArraySelectProjection, resultErr error) {
	if p == nil || p.syntax == nil || md == nil || md.Manager() != p.syntax.manager {
		return nil, ast.ErrASTHandle
	}
	result = &legacyArraySelectProjection{manager: p.syntax.manager}
	defer func() {
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
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
	selected := make(map[ast.ASTHandle][]ast.ASTHandle)
	for _, variable := range variables {
		selected[variable] = nil
	}
	done := make(map[ast.ASTHandle]bool)
	var collect func(ast.ASTHandle) error
	collect = func(current ast.ASTHandle) error {
		if done[current] {
			return nil
		}
		done[current] = true
		kind, err := p.syntax.manager.GetKind(current)
		if err != nil || kind != ast.ASTApp {
			return err
		}
		count, err := p.syntax.manager.GetNumArgs(current)
		if err != nil {
			return err
		}
		for index := uint32(0); index < count; index++ {
			child, err := p.syntax.manager.GetArg(current, index)
			if err != nil {
				return err
			}
			if err = collect(child); err != nil {
				return err
			}
		}
		args, selectTerm, err := p.syntax.operation(current, p.syntax.family, ast.ASTArrayOpSelect)
		if err != nil {
			return err
		}
		if selectTerm && len(args) == 2 {
			if _, ok := selected[args[0]]; ok {
				selected[args[0]] = append(selected[args[0]], current)
			}
		}
		return nil
	}
	if err = collect(formula); err != nil {
		return nil, err
	}
	replace, err := rewriter.NewExprSafeReplace(p.syntax.manager)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, replace.Close()) }()
	conditions := make([]ast.ASTHandle, 0)
	values := make([]ast.ASTRef, 0)
	defer func() {
		for index := len(values) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, p.syntax.manager.DelRef(&values[index]))
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
				conditions = append(conditions, equality)
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
			err = errors.Join(err, p.syntax.manager.DelRef(&termValue))
			if err != nil {
				return nil, err
			}
			text, _, numeral, err := p.syntax.manager.ArithmeticNumeralText(valueHandle)
			if err != nil || !numeral {
				return nil, errors.Join(err, ast.ErrASTDomain)
			}
			number, ok := new(big.Rat).SetString(text)
			if !ok {
				return nil, ast.ErrASTDomain
			}
			classes = append(classes, indexClass{representative: index, value: value, constant: constant, number: number})
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
			conditions = append(conditions, less)
		}
	}
	conjuncts := append(conditions, formula)
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

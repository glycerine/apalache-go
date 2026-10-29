package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// arith_project_util::collect_lits, substitute and operator().

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

type legacyFormulaProjection struct {
	manager   *ast.ASTManager
	formula   ast.ASTRef
	variables []ast.ASTRef
}

func (r *legacyFormulaProjection) Close() (result error) {
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

func (p *legacyArithmeticProjection) collectLegacyLiterals(formula ast.ASTHandle) ([]ast.ASTHandle, error) {
	seen := make(map[ast.ASTHandle]bool)
	work := []ast.ASTHandle{formula}
	result := make([]ast.ASTHandle, 0)
	for len(work) != 0 {
		last := len(work) - 1
		current := work[last]
		work = work[:last]
		if seen[current] {
			continue
		}
		seen[current] = true
		kind, err := p.syntax.manager.GetKind(current)
		if err != nil {
			return nil, err
		}
		if kind != ast.ASTApp {
			continue
		}
		family, err := p.syntax.manager.GetFamilyID(current)
		if err != nil {
			return nil, err
		}
		op, err := p.syntax.manager.GetDeclKind(current)
		if err != nil {
			return nil, err
		}
		if family == p.syntax.manager.BasicFamilyID() && (op == ast.ASTOpAnd || op == ast.ASTOpOr) {
			count, err := p.syntax.manager.GetNumArgs(current)
			if err != nil {
				return nil, err
			}
			for index := uint32(0); index < count; index++ {
				child, err := p.syntax.manager.GetArg(current, index)
				if err != nil {
					return nil, err
				}
				work = append(work, child)
			}
		} else {
			result = append(result, current)
		}
	}
	return result, nil
}

func (p *legacyArithmeticProjection) projectFormula(md *model.ModelCore, variables []ast.ASTHandle, formula ast.ASTHandle) (_ *legacyFormulaProjection, resultErr error) {
	if p == nil || p.syntax == nil || md == nil || md.Manager() != p.syntax.manager {
		return nil, ast.ErrASTHandle
	}
	result := &legacyFormulaProjection{manager: p.syntax.manager}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	factored, err := p.factorModTerms(md, formula)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, factored.Close()) }()
	allVariables := append([]ast.ASTHandle(nil), variables...)
	for _, variableRef := range factored.variables {
		variable, err := p.syntax.manager.RefGet(variableRef)
		if err != nil {
			return nil, err
		}
		allVariables = append(allVariables, variable)
	}
	initial, err := p.syntax.manager.RefGet(factored.formula)
	if err != nil {
		return nil, err
	}
	current, err := p.syntax.manager.NewRef(initial)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.syntax.manager.DelRef(&current)) }()
	for _, variable := range allVariables {
		integer, arithmetic, err := p.syntax.arithmetic(variable)
		if err != nil {
			return nil, err
		}
		if !arithmetic {
			variableRef, err := p.syntax.manager.NewRef(variable)
			if err != nil {
				return nil, err
			}
			result.variables = append(result.variables, variableRef)
			continue
		}
		currentHandle, err := p.syntax.manager.RefGet(current)
		if err != nil {
			return nil, err
		}
		if integer {
			converted, err := p.modToDivisibility(currentHandle)
			if err != nil {
				return nil, err
			}
			if err = p.syntax.manager.DelRef(&current); err != nil {
				_ = p.syntax.manager.DelRef(&converted)
				return nil, err
			}
			current = converted
			currentHandle, err = p.syntax.manager.RefGet(current)
			if err != nil {
				return nil, err
			}
		}
		literals, err := p.collectLegacyLiterals(currentHandle)
		if err != nil {
			return nil, err
		}
		var projection *legacyRealProjection
		var projected bool
		if integer {
			projection, projected, err = p.projectInteger(md, variable, literals)
		} else {
			projection, projected, err = p.projectReal(md, variable, literals)
		}
		if err != nil {
			return nil, err
		}
		if !projected {
			variableRef, err := p.syntax.manager.NewRef(variable)
			if err != nil {
				return nil, err
			}
			result.variables = append(result.variables, variableRef)
			continue
		}
		replace, err := rewriter.NewExprSafeReplace(p.syntax.manager)
		if err != nil {
			_ = projection.Close()
			return nil, err
		}
		for _, replacement := range projection.replacements {
			destination, e := p.syntax.manager.RefGet(replacement.replacement)
			if e == nil {
				e = replace.Insert(replacement.literal, destination)
			}
			if e != nil {
				err = e
				break
			}
		}
		if err == nil && projection.substitution != (ast.ASTRef{}) {
			destination, e := p.syntax.manager.RefGet(projection.substitution)
			if e == nil {
				e = replace.Insert(variable, destination)
			}
			err = e
		}
		next := ast.ASTRef{}
		if err == nil {
			next, err = p.syntax.manager.NewRef(ast.ASTHandle{})
		}
		if err == nil {
			err = replace.Apply(currentHandle, next)
		}
		if err == nil && projection.divisibility != (ast.ASTRef{}) {
			rewritten, e := p.syntax.manager.RefGet(next)
			if e == nil {
				constraint, x := p.syntax.manager.RefGet(projection.divisibility)
				if x == nil {
					var conjunction ast.ASTHandle
					conjunction, x = p.syntax.manager.MkAnd(rewritten, constraint)
					if x == nil {
						x = p.syntax.manager.SetRef(next, conjunction)
					}
				}
				e = x
			}
			err = e
		}
		err = errors.Join(err, replace.Close(), projection.Close())
		if err != nil {
			if next != (ast.ASTRef{}) {
				err = errors.Join(err, p.syntax.manager.DelRef(&next))
			}
			return nil, err
		}
		if err = p.syntax.manager.DelRef(&current); err != nil {
			_ = p.syntax.manager.DelRef(&next)
			return nil, err
		}
		current = next
	}
	currentHandle, err := p.syntax.manager.RefGet(current)
	if err != nil {
		return nil, err
	}
	result.formula, err = p.rewriteLegacy(currentHandle)
	if err != nil {
		return nil, err
	}
	return result, nil
}

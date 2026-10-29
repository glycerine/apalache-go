package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// array_project_eqs_util::project/mk_result no-substitution path.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

func (p *legacyArrayProjection) replaceEqualityPolarities(md *model.ModelCore, variable, formula ast.ASTHandle) (result *legacyArrayFormula, resultErr error) {
	if p == nil || p.syntax == nil || md == nil || md.Manager() != p.syntax.manager {
		return nil, ast.ErrASTHandle
	}
	defer func() {
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	equalities, err := p.findEqualities(variable, formula)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, equalities.Close()) }()
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
	replace, err := rewriter.NewExprSafeReplace(p.syntax.manager)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, replace.Close()) }()
	truth, err := p.syntax.manager.MkTrue()
	if err != nil {
		return nil, err
	}
	falsity, err := p.syntax.manager.MkFalse()
	if err != nil {
		return nil, err
	}
	for _, equalityRef := range equalities.equalities {
		equality, err := p.syntax.manager.RefGet(equalityRef)
		if err != nil {
			return nil, err
		}
		value, err := evaluator.EvaluateValue(equality)
		if err != nil {
			return nil, err
		}
		valueHandle, err := p.syntax.manager.RefGet(value)
		if err != nil {
			_ = p.syntax.manager.DelRef(&value)
			return nil, err
		}
		replacement := truth
		if p.syntax.manager.IsFalse(valueHandle) {
			replacement = falsity
		}
		err = errors.Join(replace.Insert(equality, replacement), p.syntax.manager.DelRef(&value))
		if err != nil {
			return nil, err
		}
	}
	replaced, err := p.syntax.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	if err = replace.Apply(formula, replaced); err != nil {
		return nil, errors.Join(err, p.syntax.manager.DelRef(&replaced))
	}
	handle, err := p.syntax.manager.RefGet(replaced)
	if err != nil {
		return nil, errors.Join(err, p.syntax.manager.DelRef(&replaced))
	}
	final, err := p.rewrite(handle)
	err = errors.Join(err, p.syntax.manager.DelRef(&replaced))
	if err != nil {
		return nil, err
	}
	result = &legacyArrayFormula{manager: p.syntax.manager, formula: final}
	return result, nil
}

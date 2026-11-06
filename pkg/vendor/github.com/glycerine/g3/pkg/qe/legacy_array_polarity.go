package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// array_project_eqs_util::project/mk_result no-substitution path.

import (
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
			resultErr = closeFailedLegacyArrayRewrite(result.manager, result.formula, resultErr)
			result = nil
		}
	}()
	equalities, err := p.findEqualities(variable, formula)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = closeFailedLegacyArrayEqualities(equalities, resultErr) }()
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
	replace, err := rewriter.NewExprSafeReplace(p.syntax.manager)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = closeFailedLegacyArrayReplacer(p.syntax.manager, replace, resultErr) }()
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
			return nil, closeFailedLegacyArrayRewrite(p.syntax.manager, value, err)
		}
		replacement := truth
		if p.syntax.manager.IsFalse(valueHandle) {
			replacement = falsity
		}
		err = replace.Insert(equality, replacement)
		err = closeFailedLegacyArrayRewrite(p.syntax.manager, value, err)
		if err != nil {
			return nil, err
		}
	}
	replaced, err := p.syntax.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	if err = replace.Apply(formula, replaced); err != nil {
		return nil, closeFailedLegacyArrayRewrite(p.syntax.manager, replaced, err)
	}
	handle, err := p.syntax.manager.RefGet(replaced)
	if err != nil {
		return nil, closeFailedLegacyArrayRewrite(p.syntax.manager, replaced, err)
	}
	final, err := p.rewrite(handle)
	err = finishLegacyPolarityRewrite(p.syntax.manager, final, replaced, err)
	if err != nil {
		return nil, err
	}
	result = &legacyArrayFormula{manager: p.syntax.manager, formula: final}
	return result, nil
}

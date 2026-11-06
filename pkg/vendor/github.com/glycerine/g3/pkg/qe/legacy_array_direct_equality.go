package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// array_project_eqs_util::project/mk_result direct-equality path.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

func (p *legacyArrayProjection) projectDirectEquality(md *model.ModelCore, variable, formula ast.ASTHandle) (result *legacyArrayFormula, eliminated bool, resultErr error) {
	if p == nil || p.syntax == nil || md == nil || md.Manager() != p.syntax.manager {
		return nil, false, ast.ErrASTHandle
	}
	defer func() {
		if resultErr != nil && result != nil {
			resultErr = closeFailedLegacyArrayRewrite(result.manager, result.formula, resultErr)
			result = nil
			eliminated = false
		}
	}()
	equalities, err := p.findEqualities(variable, formula)
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = closeFailedLegacyArrayEqualities(equalities, resultErr) }()
	evaluator, err := newLegacyModelEvaluator(md, model.ModelEvaluatorConfig{
		Context: p.syntax.config.Context, Globals: p.syntax.config.Globals,
		Memory: p.syntax.manager.NumeralService().RuntimeOwner(), Cooperation: p.syntax.config.Cooperation, Trace: p.syntax.config.Trace,
	})
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = closeFailedLegacyArrayEvaluator(p.syntax.manager, evaluator, resultErr) }()
	if err = evaluator.SetModelCompletion(true); err != nil {
		return nil, false, err
	}
	selectedEquality := ast.ASTHandle{}
	substitution := ast.ASTHandle{}
	contains, err := ast.NewContainsApp(p.syntax.manager, variable)
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = closeFailedLegacyArrayContains(p.syntax.manager, contains, resultErr) }()
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
			return nil, false, closeFailedLegacyArrayRewrite(p.syntax.manager, value, err)
		}
		isFalse := p.syntax.manager.IsFalse(valueHandle)
		if err = p.syntax.manager.DelRef(&value); err != nil {
			return nil, false, closeFailedLegacyArrayRewrite(p.syntax.manager, value, err)
		}
		if isFalse {
			continue
		}
		args, matched, err := p.syntax.operation(equality, p.syntax.manager.BasicFamilyID(), ast.ASTOpEq)
		if err != nil || !matched || len(args) != 2 {
			if err != nil {
				return nil, false, err
			}
			continue
		}
		if args[0] == variable {
			dependent, err := contains.Check(args[1])
			if err != nil {
				return nil, false, err
			}
			if !dependent {
				selectedEquality, substitution = equality, args[1]
				break
			}
		}
		if args[1] == variable {
			dependent, err := contains.Check(args[0])
			if err != nil {
				return nil, false, err
			}
			if !dependent {
				selectedEquality, substitution = equality, args[0]
				break
			}
		}
	}
	if selectedEquality == (ast.ASTHandle{}) {
		return nil, false, nil
	}
	replace, err := rewriter.NewExprSafeReplace(p.syntax.manager)
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = closeFailedLegacyArrayReplacer(p.syntax.manager, replace, resultErr) }()
	truth, err := p.syntax.manager.MkTrue()
	if err != nil {
		return nil, false, err
	}
	if err = replace.Insert(selectedEquality, truth); err != nil {
		return nil, false, err
	}
	if err = replace.Insert(variable, substitution); err != nil {
		return nil, false, err
	}
	replaced, err := p.syntax.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, false, err
	}
	if err = replace.Apply(formula, replaced); err != nil {
		return nil, false, closeFailedLegacyArrayRewrite(p.syntax.manager, replaced, err)
	}
	replacedHandle, err := p.syntax.manager.RefGet(replaced)
	if err != nil {
		return nil, false, closeFailedLegacyArrayRewrite(p.syntax.manager, replaced, err)
	}
	final, err := p.rewrite(replacedHandle)
	err = finishLegacyDirectEqualityRewrite(p.syntax.manager, final, replaced, err)
	if err != nil {
		return nil, false, err
	}
	result = &legacyArrayFormula{manager: p.syntax.manager, formula: final}
	return result, true, nil
}

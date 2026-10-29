package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// array_project_eqs_util::project/mk_result direct-equality path.

import (
	"errors"

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
	falseEqualities := make([]ast.ASTHandle, 0)
	selectedEquality := ast.ASTHandle{}
	substitution := ast.ASTHandle{}
	contains, err := ast.NewContainsApp(p.syntax.manager, variable)
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, contains.Close()) }()
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
		return nil, false, errors.Join(err, p.syntax.manager.DelRef(&replaced))
	}
	replacedHandle, err := p.syntax.manager.RefGet(replaced)
	if err != nil {
		return nil, false, errors.Join(err, p.syntax.manager.DelRef(&replaced))
	}
	final, err := p.rewrite(replacedHandle)
	err = errors.Join(err, p.syntax.manager.DelRef(&replaced))
	if err != nil {
		return nil, false, err
	}
	result = &legacyArrayFormula{manager: p.syntax.manager, formula: final}
	return result, true, nil
}

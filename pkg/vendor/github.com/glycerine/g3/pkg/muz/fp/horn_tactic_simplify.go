package fp

// Source: horn_tactic::simplify and mk_horn_simplify_tactic.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func NewHornSimplifyTactic(manager *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (*HornTactic, error) {
	return newHornTactic(manager, context, globals, true)
}

func (t *HornTactic) simplifyGoal(goal *tactic.Goal, out *tactic.TacticResult, engine *PropositionalEngine, query, queryDeclaration ast.ASTHandle) (resultErr error) {
	if err := engine.set.SetOutputPredicate(queryDeclaration); err != nil {
		return err
	}
	if _, err := engine.set.Finalize(); err != nil {
		return err
	}
	// horn_tactic::simplify uses the default Horn pipeline followed by
	// optional slicing, without the solver engines' coalescing/unfolding.
	prepared, err := engine.preprocessHornRulesWithQueryRule(queryDeclaration, nil, true, true, false, false)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, prepared.Close()) }()
	parameters, err := engine.fixedpointParamView()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, parameters.Close()) }()
	slice, err := parameters.Bool("xform.slice", true)
	if err != nil {
		return err
	}
	if slice {
		transformer := base.NewRuleTransformer()
		plugin := transforms.NewRuleSlicePlugin(t.manager, engine.rules, 1)
		generateProof, err := parameters.Bool("generate_proof_trace", false)
		if err != nil {
			return errors.Join(err, plugin.Close(), transformer.Close())
		}
		plugin.SetGenerateProofTrace(generateProof || engine.forceProofTrace)
		if err = transformer.Register(plugin); err != nil {
			return errors.Join(err, plugin.Close(), transformer.Close())
		}
		_, err = transformer.Apply(prepared.rules)
		if err = errors.Join(err, transformer.Close()); err != nil {
			return err
		}
	}
	falsity, err := t.manager.MkFalse()
	if err != nil {
		return err
	}
	replacer, err := rewriter.NewExprSafeReplace(t.manager)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, replacer.Close()) }()
	if err = replacer.Insert(query, falsity); err != nil {
		return err
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	if err = goal.Reset(); err != nil {
		return err
	}
	count, err := prepared.rules.Len()
	if err != nil {
		return err
	}
	for index := 0; index < count; index++ {
		rule, x := prepared.rules.Rule(index)
		if x != nil {
			return x
		}
		formula, x := rule.ToFormula()
		x = errors.Join(x, rule.Close())
		if x != nil {
			return x
		}
		formulaHandle, x := t.manager.RefGet(formula)
		if x != nil {
			return errors.Join(x, t.manager.DelRef(&formula))
		}
		output, x := t.manager.NewRef(ast.ASTHandle{})
		if x != nil {
			return errors.Join(x, t.manager.DelRef(&formula))
		}
		x = replacer.Apply(formulaHandle, output)
		handle, y := t.manager.RefGet(output)
		if x == nil {
			x = y
		}
		if x == nil {
			x = goal.Assert(handle)
		}
		x = errors.Join(x, t.manager.DelRef(&output), t.manager.DelRef(&formula))
		if x != nil {
			return x
		}
	}
	if t.progress != nil {
		t.progress(1)
	}
	return out.AddGoal(goal)
}

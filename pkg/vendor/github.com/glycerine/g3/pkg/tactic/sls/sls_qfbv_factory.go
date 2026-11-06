package sls

// Source: src/tactic/sls/sls_tactic.cpp:100-153.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/bv"
	"github.com/glycerine/g3/pkg/tactic/core"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type QFBVConfig struct {
	Context        *uparams.ParamsContext
	Globals        *uparams.GlobalParamsService
	ModelEvaluator model.ModelEvaluatorConfig
}

// NewQFBVSLSTactic ports mk_preamble followed by mk_sls_tactic. In particular,
// main_p and ctx_p are constructed and populated but deliberately not applied:
// that is the executable behavior of this 4.7.1 fork.
func NewQFBVSLSTactic(manager *ast.ASTManager, params *uparams.ParamsRef, config QFBVConfig) (tactic.Tactic, error) {
	c, g, services := config.Context, config.Globals, config.ModelEvaluator
	if manager == nil || c == nil || g == nil || g.Context != c || services.Context != c || services.Globals != g || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(manager.Symbols, manager.NumeralService()); err != nil {
		return nil, err
	}
	b := newSLSPreambleBuilder(manager, c)

	mainP := b.parameter(nil)
	for _, key := range []string{"elim_and", "push_ite_bv", "blast_distinct", "hi_div0"} {
		b.boolean(mainP, key, true)
	}
	simp2P := b.parameter(params)
	for _, key := range []string{"som", "pull_cheap_ite", "local_ctx"} {
		b.boolean(simp2P, key, true)
	}
	b.boolean(simp2P, "push_ite_bv", false)
	b.unsigned(simp2P, "local_ctx_limit", 10000000)
	hoistP := b.parameter(nil)
	b.boolean(hoistP, "hoist_mul", true)
	b.boolean(hoistP, "som", false)
	gaussianP := b.parameter(nil)
	b.unsigned(gaussianP, "gaussian_max_occs", 2)
	ctxP := b.parameter(nil)
	b.unsigned(ctxP, "max_depth", 32)
	b.unsigned(ctxP, "max_steps", 5000000)
	if b.err != nil {
		return b.finish(nil)
	}

	simplify1, err := core.NewSimplifyTactic(manager, c, g, nil)
	first := b.own(simplify1, err)
	propagate, err := core.NewPropagateValuesTactic(manager, c, g, nil)
	second := b.own(propagate, err)
	solve, err := core.NewSolveEquationsTactic(manager, c, g, nil, services)
	third := b.using(b.own(solve, err), gaussianP)
	eliminate, err := core.NewEliminateUnconstrainedTactic(manager, c, g, nil, services)
	fourth := b.own(eliminate, err)
	reduce, err := bv.NewBVSizeReductionTactic(manager, c, g, services)
	fifth := b.own(reduce, err)
	simplify2, err := core.NewSimplifyTactic(manager, c, g, nil)
	sixth := b.using(b.own(simplify2, err), simp2P)
	inner := b.sequence(first, second, third, fourth, fifth, sixth)

	hoist, err := core.NewSimplifyTactic(manager, c, g, nil)
	hoistStage := b.using(b.own(hoist, err), hoistP)
	sharing, err := bv.NewMaxBVSharingTactic(manager, c, nil, services.Cooperation)
	sharingStage := b.own(sharing, err)
	nnf, err := core.NewNNFTactic(manager, c, g, params)
	nnfStage := b.own(nnf, err)
	preamble := b.sequence(inner, hoistStage, sharingStage, nnfStage)

	sls, err := NewSLSTactic(manager, c, g, params)
	guarded := b.own(sls, err)
	result := b.sequence(preamble, guarded)
	if b.err == nil {
		update := params
		if update == nil {
			empty := c.EmptyValue()
			update = &empty
		}
		b.err = result.UpdateParams(update)
	}
	return b.finish(result)
}

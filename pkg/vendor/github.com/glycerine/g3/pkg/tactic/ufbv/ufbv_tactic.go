package ufbv

// Source: src/tactic/ufbv/ufbv_tactic.cpp/.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/core"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type ufbvConstruction struct {
	context    *uparams.ParamsContext
	owners     []tactic.Tactic
	parameters []*uparams.ParamsRef
	err        error
}

func (b *ufbvConstruction) keep(owner tactic.Tactic, err error) tactic.Tactic {
	if err != nil {
		b.err = errors.Join(b.err, err)
		return nil
	}
	if b.err != nil || owner == nil {
		return nil
	}
	if err = owner.IncRef(); err != nil {
		b.err = errors.Join(b.err, err)
		return nil
	}
	b.owners = append(b.owners, owner)
	return owner
}
func (b *ufbvConstruction) then(children ...tactic.Tactic) tactic.Tactic {
	if b.err != nil {
		return nil
	}
	return b.keep(tactic.NewAndThenTactic(children...))
}
func (b *ufbvConstruction) repeat(child tactic.Tactic, depth uint32) tactic.Tactic {
	if b.err != nil {
		return nil
	}
	return b.keep(tactic.NewRepeatTactic(child, depth))
}
func (b *ufbvConstruction) noProofs(child tactic.Tactic) tactic.Tactic {
	if b.err != nil {
		return nil
	}
	return b.keep(tactic.NewIfNoProofsTactic(child))
}
func (b *ufbvConstruction) noCores(child tactic.Tactic) tactic.Tactic {
	if b.err != nil {
		return nil
	}
	return b.keep(tactic.NewIfNoUnsatCoresTactic(child))
}
func (b *ufbvConstruction) using(child tactic.Tactic, params *uparams.ParamsRef) tactic.Tactic {
	if b.err != nil {
		return nil
	}
	return b.keep(tactic.NewUsingParamsTactic(b.context, child, params))
}
func (b *ufbvConstruction) params() *uparams.ParamsRef {
	value, err := b.context.NewValue()
	if err != nil {
		b.err = errors.Join(b.err, err)
		return nil
	}
	b.parameters = append(b.parameters, &value)
	return &value
}
func (b *ufbvConstruction) boolean(params *uparams.ParamsRef, name string, value bool) {
	if b.err == nil {
		b.err = b.context.SetBoolName(params, &name, value)
	}
}
func (b *ufbvConstruction) unsigned(params *uparams.ParamsRef, name string, value uint32) {
	if b.err == nil {
		b.err = b.context.SetUIntName(params, &name, value)
	}
}
func (b *ufbvConstruction) close() (result error) {
	for len(b.owners) != 0 {
		i := len(b.owners) - 1
		result = errors.Join(result, b.owners[i].DecRef())
		b.owners = b.owners[:i]
	}
	for len(b.parameters) != 0 {
		i := len(b.parameters) - 1
		result = errors.Join(result, b.context.Del(b.parameters[i]))
		b.parameters = b.parameters[:i]
	}
	return result
}

// NewUFBVTactic builds the retained quantified UFBV preprocessing fixed point
// followed by the SMT backend. Source trace-only tactics are intentionally
// omitted; they do not alter goals or models.
func NewUFBVTactic(m *ast.ASTManager, params *uparams.ParamsRef, config smtcore.SMTSolverConfig) (result tactic.Tactic, err error) {
	context, globals := config.Params, config.Globals
	if m == nil || context == nil || globals == nil || globals.Context != context {
		return nil, uparams.ErrParamsOwner
	}
	if err = context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	b := ufbvConstruction{context: context}
	defer func() { err = errors.Join(err, b.close()) }()
	main := b.params()
	b.boolean(main, "mbqi", true)
	b.unsigned(main, "mbqi.max_iterations", ^uint32(0))
	b.boolean(main, "elim_and", true)
	noElimAnd := b.params()
	b.boolean(noElimAnd, "elim_and", false)
	if b.err != nil {
		return nil, b.err
	}
	simplify := func() tactic.Tactic { return b.keep(core.MakeSimplifyTactic(m, context, globals, nil)) }
	derFixedPoint := func() tactic.Tactic {
		step := b.then(b.keep(core.NewDERTactic(m)), simplify())
		return b.repeat(step, ^uint32(0))
	}
	firstMacro := b.keep(NewMacroFinderTactic(m, context, globals, nil, config.ModelEvaluator))
	firstMacro = b.using(firstMacro, noElimAnd)
	firstMacro = b.noCores(firstMacro)
	firstMacro = b.noProofs(firstMacro)
	front := b.then(
		simplify(),
		b.keep(core.NewPropagateValuesTactic(m, context, globals, nil)),
		firstMacro,
		simplify(),
		b.keep(core.NewSNFTactic(m, context, globals, nil)),
		simplify(),
		b.keep(core.MakeEliminateAndTactic(m, context, globals, nil)),
		b.keep(core.MakeSolveEquationsTactic(m, context, globals, nil, config.ModelEvaluator)),
		derFixedPoint(), simplify(),
		b.keep(core.NewDistributeForallTactic(m, nil)), simplify())
	coreFree := b.then(
		b.keep(core.MakeReduceArgsTactic(m, context, globals, nil, config.ModelEvaluator)), simplify(),
		b.keep(NewMacroFinderTactic(m, context, globals, nil, config.ModelEvaluator)), simplify(),
		b.keep(NewRewriterTacticAdapter(m, context, globals)), simplify(),
		b.keep(NewQuasiMacrosTactic(m, context, globals, nil, config.ModelEvaluator)), simplify())
	preprocessor := b.then(front, b.noCores(coreFree), derFixedPoint(), simplify(), simplify())
	preprocessor = b.repeat(preprocessor, 2)
	backend := b.keep(smtcore.NewSMTTactic(m, nil, config))
	body := b.then(preprocessor, backend)
	if b.err != nil {
		return nil, b.err
	}
	result, err = tactic.NewUsingParamsTactic(context, body, main)
	if err != nil {
		return nil, err
	}
	if params == nil {
		empty := context.EmptyValue()
		params = &empty
	}
	if err = result.UpdateParams(params); err != nil {
		if incErr := result.IncRef(); incErr == nil {
			err = errors.Join(err, result.DecRef())
		} else {
			err = errors.Join(err, incErr)
		}
		result = nil
	}
	return result, err
}

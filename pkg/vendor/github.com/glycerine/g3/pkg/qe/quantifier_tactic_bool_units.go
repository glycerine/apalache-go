package qe

// Source: src/qe/qe.cpp and qe_bool_plugin.cpp, complete Boolean unit cases.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
)

// eliminateQuantifierBoolUnits returns an owned formula only when Boolean unit
// solving and supported solver-guided branches eliminate all declarations.
func eliminateQuantifierBoolUnits(m *ast.ASTManager, quantifier ast.ASTHandle, config Config) (result ast.ASTRef, complete bool, err error) {
	defer func() {
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
		}
	}()
	kind, err := m.GetKind(quantifier)
	if err != nil || kind != ast.ASTQuantifier {
		return result, false, err
	}
	free, err := rewriter.NewExprFreeVars(m)
	if err != nil {
		return result, false, err
	}
	if err = free.Collect(quantifier); err != nil || !free.Empty() {
		return result, false, err
	}
	body, err := m.GetQuantifierBody(quantifier)
	if err != nil {
		return result, false, err
	}
	nested, err := m.HasQuantifiers(body)
	if err != nil || nested {
		return result, false, err
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return result, false, err
	}
	count, err := m.GetNumDeclarations(quantifier)
	if err != nil {
		return result, false, err
	}
	for i := uint32(0); i < count; i++ {
		sort, e := m.GetDeclarationSort(quantifier, i)
		if e != nil || sort != boolean {
			return result, false, e
		}
	}
	extracted, err := extractExprQuantifier(m, quantifier)
	if err != nil {
		return result, false, closeFailedExprQuantifierExtract(extracted, err)
	}
	defer func() { err = errors.Join(err, extracted.Close()) }()
	handles := make([]ast.ASTHandle, len(extracted.variables))
	for i, ref := range extracted.variables {
		handles[i], err = m.RefGet(ref)
		if err != nil {
			return result, false, err
		}
	}
	context, err := NewSolverContext(m, handles)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, context.Close()) }()
	plugin, err := NewBoolPlugin(context)
	if err != nil {
		return result, false, err
	}
	if err = context.AddPlugin(plugin); err != nil {
		return result, false, errors.Join(err, plugin.Close())
	}
	adapter, err := NewSolverNNFAdapter(context)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, adapter.Close()) }()
	nnf, err := NewQENNFNormalizer(m, adapter, adapter, config)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, nnf.Close()) }()
	rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, nil)
	if err != nil {
		if rw != nil {
			err = errors.Join(err, rw.Close())
		}
		return result, false, err
	}
	defer func() { err = errors.Join(err, rw.Close()) }()
	variables, err := newExprQEBranchVariables(m)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, variables.Close()) }()
	for _, variable := range handles {
		if err = variables.add(variable); err != nil {
			return result, false, err
		}
	}
	truth, err := m.MkTrue()
	if err != nil {
		return result, false, err
	}
	root, err := newExprQETree(m, config, nil, truth)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	body, err = m.RefGet(extracted.body)
	if err != nil {
		return result, false, err
	}
	forall, err := m.IsForall(quantifier)
	if err != nil {
		return result, false, err
	}
	if forall {
		body, err = m.MkNot(body)
		if err != nil {
			return result, false, err
		}
	}
	if err = root.initialize(body); err != nil {
		return result, false, err
	}
	if err = variables.consume(root); err != nil {
		return result, false, err
	}
	if err = normalizeExprQETree(root, rw, nnf); err != nil {
		return result, false, err
	}
	leaf, err := solveExprQEBoolUnits(root, variables, plugin, config, rw, nnf)
	if err != nil {
		return result, false, err
	}
	if len(leaf.variables) == 0 {
		body, err = m.RefGet(leaf.formula)
	} else {
		partitioner, e := NewConjunctionPartitioner(m)
		if e != nil {
			return result, false, e
		}
		defer func() { err = errors.Join(err, partitioner.Close()) }()
		if e = partitioner.AddPlugin(plugin); e != nil {
			return result, false, e
		}
		solver, e := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: config.Context, Globals: config.Globals, ModelEvaluator: model.ModelEvaluatorConfig{Context: config.Context, Globals: config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: config.Cooperation, Trace: config.Trace}})
		if e != nil {
			return result, false, e
		}
		defer func() { err = errors.Join(err, solver.Close()) }()
		if config.qeStatistics != nil {
			defer func() { err = errors.Join(err, solver.CollectStatistics(config.qeStatistics)) }()
		}
		if e = solver.SetProduceModels(true); e != nil {
			return result, false, e
		}
		initial, e := m.RefGet(root.formula)
		if e != nil {
			return result, false, e
		}
		if e = solver.AssertFormula(initial); e != nil {
			return result, false, e
		}
		searched, complete, e := searchExprQEScalarTree(root, variables, context, plugin, partitioner, solver, config, rw, nnf)
		if e != nil || !complete {
			return result, false, e
		}
		defer func() { err = errors.Join(err, m.DelRef(&searched)) }()
		body, err = m.RefGet(searched)
	}
	if err != nil {
		return result, false, err
	}
	if forall {
		body, err = m.MkNot(body)
		if err != nil {
			return result, false, err
		}
	}
	result, err = m.NewRef(body)
	if err != nil {
		return result, false, err
	}
	if err = rw.Rewrite(body, result); err != nil {
		return result, false, errors.Join(err, m.DelRef(&result))
	}
	return result, true, nil
}

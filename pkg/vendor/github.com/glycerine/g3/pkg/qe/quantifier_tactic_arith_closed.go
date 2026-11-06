package qe

// Source: src/qe/qe.cpp, satisfiable closed-partition case for one arithmetic variable.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// eliminateQuantifierArithmeticClosed handles a single arithmetic declaration
// when all conjuncts mentioning it are independent of uninterpreted symbols.
func eliminateQuantifierArithmeticClosed(m *ast.ASTManager, quantifier ast.ASTHandle, config Config) (result ast.ASTRef, complete bool, err error) {
	if m == nil || config.Context == nil || config.Globals == nil || config.Cooperation == nil {
		return result, false, ast.ErrASTHandle
	}
	defer func() {
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
		}
	}()
	canceled, err := m.Canceled()
	if err != nil {
		return result, false, err
	}
	if canceled {
		return result, false, tactic.NewTacticException(m.ResourceLimit().CancelMessage())
	}
	if err = config.Cooperation.Cooperate("qe"); err != nil {
		return result, false, err
	}
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
	count, err := m.GetNumDeclarations(quantifier)
	if err != nil || count != 1 {
		return result, false, err
	}
	sort, err := m.GetDeclarationSort(quantifier, 0)
	if err != nil {
		return result, false, err
	}
	family, err := m.GetFamilyID(sort)
	if err != nil {
		return result, false, err
	}
	arithmetic, err := m.ArithmeticFamilyID()
	if err != nil || family != arithmetic {
		return result, false, err
	}
	extracted, err := extractExprQuantifier(m, quantifier)
	if err != nil {
		return result, false, closeFailedExprQuantifierExtract(extracted, err)
	}
	defer func() { err = errors.Join(err, extracted.Close()) }()
	variable, err := m.RefGet(extracted.variables[0])
	if err != nil {
		return result, false, err
	}
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
	formula, err := m.NewRef(body)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&formula)) }()
	context, err := NewSolverContext(m, []ast.ASTHandle{variable})
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, context.Close()) }()
	boolPlugin, err := NewBoolPlugin(context)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, boolPlugin.Close()) }()
	arithPlugin, err := NewArithmeticPlugin(context)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, arithPlugin.Close()) }()
	partitioner, err := NewConjunctionPartitioner(m)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, partitioner.Close()) }()
	if err = partitioner.AddPlugin(boolPlugin); err != nil {
		return result, false, err
	}
	if err = partitioner.AddPlugin(arithPlugin); err != nil {
		return result, false, err
	}
	closed, mixed, open, err := partitioner.GetPartition(body, []ast.ASTHandle{variable})
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, closePartitionRefs(m, &closed, &mixed, &open)) }()
	mixedValue, err := m.RefGet(mixed)
	if err != nil || !m.IsTrue(mixedValue) {
		return result, false, err
	}
	solver, err := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: config.Context, Globals: config.Globals, ModelEvaluator: model.ModelEvaluatorConfig{Context: config.Context, Globals: config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: config.Cooperation, Trace: config.Trace}})
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, solver.Close()) }()
	if config.qeStatistics != nil {
		defer func() { err = errors.Join(err, solver.CollectStatistics(config.qeStatistics)) }()
	}
	if err = solver.AssertFormula(body); err != nil {
		return result, false, err
	}
	status, err := solver.CheckSat(nil)
	if err != nil {
		return result, false, err
	}
	if status == lbool.Undef {
		canceled, err = m.Canceled()
		if err != nil {
			return result, false, err
		}
		if canceled {
			return result, false, tactic.NewTacticException(m.ResourceLimit().CancelMessage())
		}
		return result, false, nil
	}
	if status == lbool.False {
		body, err = m.MkFalse()
	} else {
		body, err = m.RefGet(closed)
	}
	if err != nil {
		return result, false, err
	}
	if forall {
		body, err = m.MkNotSimplified(body)
		if err != nil {
			return result, false, err
		}
	}
	result, err = m.NewRef(body)
	return result, err == nil, err
}

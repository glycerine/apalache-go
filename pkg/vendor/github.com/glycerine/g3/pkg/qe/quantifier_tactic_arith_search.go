package qe

// Source: src/qe/qe.cpp, solver-guided search for one arithmetic variable.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
)

type qeScalarSearchKind uint8

const (
	qeScalarArithmetic qeScalarSearchKind = iota
	qeScalarBitVector
	qeScalarDatatype
	qeScalarDatalog
)

func eliminateQuantifierArithmeticSearch(m *ast.ASTManager, quantifier ast.ASTHandle, config Config) (ast.ASTRef, bool, error) {
	return eliminateQuantifierScalarSearch(m, quantifier, config, qeScalarArithmetic)
}

func eliminateQuantifierBitVectorSearch(m *ast.ASTManager, quantifier ast.ASTHandle, config Config) (ast.ASTRef, bool, error) {
	return eliminateQuantifierScalarSearch(m, quantifier, config, qeScalarBitVector)
}

func eliminateQuantifierDatatypeSearch(m *ast.ASTManager, quantifier ast.ASTHandle, config Config) (ast.ASTRef, bool, error) {
	return eliminateQuantifierScalarSearch(m, quantifier, config, qeScalarDatatype)
}

func eliminateQuantifierDatalogSearch(m *ast.ASTManager, quantifier ast.ASTHandle, config Config) (ast.ASTRef, bool, error) {
	return eliminateQuantifierScalarSearch(m, quantifier, config, qeScalarDatalog)
}

// eliminateQuantifierScalarSearch handles one arithmetic, bit-vector,
// datatype, or finite-domain declaration when propagation finishes without
// fresh variables.
func eliminateQuantifierScalarSearch(m *ast.ASTManager, quantifier ast.ASTHandle, config Config, searchKind qeScalarSearchKind) (result ast.ASTRef, complete bool, err error) {
	if m == nil || config.Context == nil || config.Globals == nil || config.Cooperation == nil {
		return result, false, ast.ErrASTHandle
	}
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
	var expectedFamily ast.FamilyID
	switch searchKind {
	case qeScalarArithmetic:
		expectedFamily, err = m.ArithmeticFamilyID()
	case qeScalarBitVector:
		var bitVector bool
		bitVector, err = m.IsBitVectorSort(sort)
		if err != nil || !bitVector {
			return result, false, err
		}
		expectedFamily, err = m.BitVectorUtilityFamilyID()
	case qeScalarDatatype:
		expectedFamily, err = m.RegisterDatatypePlugin()
	case qeScalarDatalog:
		expectedFamily, err = m.RegisterDatalogPlugin()
	default:
		return result, false, ast.ErrASTDomain
	}
	if err != nil || family != expectedFamily {
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
	bodyRef, err := m.NewRef(body)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&bodyRef)) }()
	context, err := NewSolverContext(m, []ast.ASTHandle{variable})
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, context.Close()) }()
	boolPlugin, err := NewBoolPlugin(context)
	if err != nil {
		return result, false, err
	}
	if err = context.AddPlugin(boolPlugin); err != nil {
		return result, false, errors.Join(err, boolPlugin.Close())
	}
	var scalarPlugin SolverPlugin
	var arithPlugin *ArithmeticPlugin
	switch searchKind {
	case qeScalarArithmetic:
		if config.nonlinearQuantifierElimination {
			var nonlinear *NonlinearArithmeticPlugin
			nonlinear, err = NewNonlinearArithmeticPlugin(context, config.produceModels)
			scalarPlugin = nonlinear
		} else {
			arithPlugin, err = NewArithmeticPlugin(context)
			scalarPlugin = arithPlugin
		}
	case qeScalarBitVector:
		var bitVectorPlugin *BitVectorPlugin
		bitVectorPlugin, err = NewBitVectorPlugin(context)
		scalarPlugin = bitVectorPlugin
	case qeScalarDatatype:
		var datatypePlugin *DatatypeSolverPlugin
		datatypePlugin, err = NewDatatypeSolverPlugin(context)
		scalarPlugin = datatypePlugin
	case qeScalarDatalog:
		var datalogPlugin *DatalogPlugin
		datalogPlugin, err = NewDatalogPlugin(context)
		scalarPlugin = datalogPlugin
	}
	if err != nil {
		return result, false, err
	}
	if err = context.AddPlugin(scalarPlugin); err != nil {
		return result, false, errors.Join(err, scalarPlugin.Close())
	}
	if searchKind == qeScalarArithmetic {
		if _, cause := m.RegisterBitVectorPlugin(); cause != nil {
			return result, false, cause
		}
		bitVectors, cause := NewBitVectorPlugin(context)
		if cause != nil {
			return result, false, cause
		}
		if cause = context.AddPlugin(bitVectors); cause != nil {
			return result, false, errors.Join(cause, bitVectors.Close())
		}
	}
	rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, nil)
	if err != nil {
		if rw != nil {
			err = errors.Join(err, rw.Close())
		}
		return result, false, err
	}
	defer func() { err = errors.Join(err, rw.Close()) }()
	if arithPlugin != nil {
		arithPlugin.util.theory = rw
		arithPlugin.util.qeConfig = &config
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
	partitioner, err := NewConjunctionPartitioner(m)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, partitioner.Close()) }()
	if err = partitioner.AddPlugin(boolPlugin); err != nil {
		return result, false, err
	}
	if err = partitioner.AddPlugin(scalarPlugin); err != nil {
		return result, false, err
	}
	if searchKind == qeScalarArithmetic {
		family, cause := m.BitVectorUtilityFamilyID()
		if cause != nil {
			return result, false, cause
		}
		plugin, found, cause := context.Plugin(family)
		if cause != nil || !found {
			return result, false, errors.Join(cause, ast.ErrASTDomain)
		}
		if cause = partitioner.AddPlugin(plugin); cause != nil {
			return result, false, cause
		}
	}
	variables, err := newExprQEBranchVariables(m)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, variables.Close()) }()
	if err = variables.add(variable); err != nil {
		return result, false, err
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
	if simplifier, ok := scalarPlugin.(exprQETreeSimplifier); ok {
		root.simplifiers = append(root.simplifiers, simplifier)
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
	solver, err := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: config.Context, Globals: config.Globals, ModelEvaluator: model.ModelEvaluatorConfig{Context: config.Context, Globals: config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: config.Cooperation, Trace: config.Trace}})
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, solver.Close()) }()
	if config.qeStatistics != nil {
		defer func() { err = errors.Join(err, solver.CollectStatistics(config.qeStatistics)) }()
	}
	if err = solver.SetProduceModels(true); err != nil {
		return result, false, err
	}
	initial, err := m.RefGet(root.formula)
	if err != nil {
		return result, false, err
	}
	if err = solver.AssertFormula(initial); err != nil {
		return result, false, err
	}
	result, complete, err = searchExprQEScalarTree(root, variables, context, boolPlugin, partitioner, solver, config, rw, nnf)
	if err != nil || !complete {
		return result, false, err
	}
	value, err := m.RefGet(result)
	if err != nil {
		return result, false, err
	}
	contains, err := ast.NewContainsApp(m, variable)
	if err != nil {
		return result, false, err
	}
	escaped, err := contains.Check(value)
	err = errors.Join(err, contains.Close())
	if err != nil {
		return result, false, err
	}
	if escaped {
		err = m.DelRef(&result)
		return result, false, err
	}
	if forall {
		value, e := m.RefGet(result)
		if e != nil {
			return result, false, e
		}
		negated, e := m.MkNot(value)
		if e != nil {
			return result, false, e
		}
		if e = rw.Rewrite(negated, result); e != nil {
			return result, false, e
		}
	}
	return result, true, nil
}

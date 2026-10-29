package smtcore

// Source: src/smt/theory_str.cpp seq_expr_solver and theory_str::init.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// smtStringExpressionSolver owns its kernel, borrows the parent manager and
// SMT parameters, and lends only the source expr_solver query operation.
type smtStringExpressionSolver struct {
	kernel *SMTKernel
}

func newSMTStringExpressionSolver(manager *ast.ASTManager, parameters *SMTParams) (*smtStringExpressionSolver, error) {
	if manager == nil || parameters == nil {
		return nil, ast.ErrASTDomain
	}
	if _, err := parameters.State(); err != nil {
		return nil, err
	}
	kernel, err := NewSMTKernel(manager, nil)
	if err != nil {
		return nil, err
	}
	empty := parameters.Context.EmptyValue()
	if err = kernel.configurePreprocessingParams(parameters.Context, parameters.Globals, &empty, parameters, true); err != nil {
		return nil, errors.Join(err, kernel.Close())
	}
	return &smtStringExpressionSolver{kernel: kernel}, nil
}

func (s *smtStringExpressionSolver) CheckSatisfiable(expression ast.ASTHandle) (result lbool.LBool, resultErr error) {
	if s == nil || s.kernel == nil {
		return lbool.Undef, ast.ErrASTHandle
	}
	// Native context reads the shared m_fparams during each push/check.
	// Refresh the Go scalar snapshots without applying an empty update (which
	// would reset the shared parent's options) or replacing parameter owners.
	full := s.kernel.smtParams
	state, err := full.State()
	if err != nil {
		return lbool.Undef, err
	}
	arithmetic, err := full.ArithmeticState()
	if err != nil {
		return lbool.Undef, err
	}
	bitVector, err := full.BitVectorState()
	if err != nil {
		return lbool.Undef, err
	}
	preprocess, err := full.preprocess.State()
	if err != nil {
		return lbool.Undef, err
	}
	s.kernel.smtState, s.kernel.arithState, s.kernel.bvState = state, arithmetic, bitVector
	s.kernel.preprocessState, s.kernel.preprocess = preprocess, state.Preprocess
	if err := s.kernel.Push(); err != nil {
		return lbool.Undef, err
	}
	defer func() {
		if err := s.kernel.Pop(1); err != nil {
			result = lbool.Undef
			resultErr = errors.Join(resultErr, err)
		}
	}()
	if err := s.kernel.Assert(expression, ast.ASTHandle{}); err != nil {
		return lbool.Undef, err
	}
	return s.kernel.Check()
}

func (s *smtStringExpressionSolver) Close() error {
	if s == nil || s.kernel == nil {
		return nil
	}
	if err := s.kernel.Close(); err != nil {
		return err
	}
	s.kernel = nil
	return nil
}

func (t *SMTStringTheory) Initialize(state *SMTInternalizerState) error {
	if err := t.SMTTheoryBase.Initialize(state); err != nil {
		return err
	}
	// Standalone internalizer fixtures have no configured kernel/SMT parameter
	// service. They can exercise concrete String callbacks, but do not supply
	// the nested symbolic solver. Configured kernel consumers always do.
	if state.smtParams == nil {
		return nil
	}
	solver, err := newSMTStringExpressionSolver(state.manager, state.smtParams)
	if err != nil {
		t.state = nil
		return err
	}
	t.regexExpressionSolver = solver
	return nil
}

var _ rewriter.SequenceExpressionSolver = (*smtStringExpressionSolver)(nil)

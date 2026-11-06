package smtcore

// Source: pre-search fparams mutation in
// src/muz/pdr/pdr_context.cpp:init_core_generalizers and the arithmetic solver
// identifiers in src/smt/params/theory_arith_params.h.

import "github.com/glycerine/g3/pkg/ast"

const (
	SMTArithmeticNoSolver uint32 = iota
	SMTArithmeticDifferenceLogic
	SMTArithmeticSimplex
	SMTArithmeticDenseDifferenceLogic
	SMTArithmeticUTVPI
	SMTArithmeticOptInf
)

// ArithmeticState returns the effective arithmetic parameters currently held
// by the solver. Before the first push/check, callers may adjust the returned
// copy and install it through SetArithmeticState.
func (s *SMTSolver) ArithmeticState() (SMTArithState, error) {
	if err := s.open(); err != nil {
		return SMTArithState{}, err
	}
	if s.kernel == nil || s.kernel.smtParams == nil {
		return SMTArithState{}, ast.ErrASTHandle
	}
	return s.kernel.arithState, nil
}

// SetArithmeticState replaces the arithmetic setup parameters before the
// first push/check chooses and initializes a concrete theory backend. It does
// not rewrite the caller's ParamsRef; a later UpdateParams may supersede it.
func (s *SMTSolver) SetArithmeticState(state SMTArithState) error {
	if err := s.open(); err != nil {
		return err
	}
	kernel := s.kernel
	if kernel == nil || kernel.smtParams == nil {
		return ast.ErrASTHandle
	}
	if state.Mode > SMTArithmeticOptInf {
		return ast.ErrASTDomain
	}
	if kernel.preprocessingSetupDone || kernel.internalizer == nil || kernel.internalizer.theoriesSetup || kernel.copiedArithmeticPlugin {
		return ast.ErrASTDomain
	}
	kernel.smtParams.arith.state = state
	kernel.arithState = state
	return nil
}

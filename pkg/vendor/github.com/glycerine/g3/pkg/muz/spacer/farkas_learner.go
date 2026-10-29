package spacer

// Source: src/muz/spacer/spacer_farkas_learner.h and
// src/muz/spacer/spacer_farkas_learner.cpp. The revised Spacer learner shares
// its proof traversal and arithmetic combination algorithm with muz/pdr.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/pdr"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// FarkasLearner extracts B-vocabulary arithmetic consequences from a proof.
// Returned ASTRef values are owned by the caller.
type FarkasLearner struct {
	manager *ast.ASTManager
	context *uparams.ParamsContext
	globals *uparams.GlobalParamsService
	inner   *pdr.FarkasLearner
	closed  bool
}

func NewFarkasLearner(manager *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (*FarkasLearner, error) {
	inner, err := pdr.NewFarkasLearner(manager, context, globals)
	if err != nil {
		return nil, err
	}
	return &FarkasLearner{manager: manager, context: context, globals: globals, inner: inner}, nil
}

func (f *FarkasLearner) open() error {
	if f == nil || f.closed || f.inner == nil {
		return ast.ErrASTHandle
	}
	return nil
}

// SetSplitLiterals controls whether disconnected arithmetic constraints are
// retained as separate disjuncts.
func (f *FarkasLearner) SetSplitLiterals(enabled bool) error {
	if err := f.open(); err != nil {
		return err
	}
	return f.inner.SetSplitLiterals(enabled)
}

// GetLemmas traverses root and returns caller-owned consequences expressed in
// the vocabulary of bAssertions.
func (f *FarkasLearner) GetLemmas(root ast.ASTHandle, bAssertions []ast.ASTHandle) ([]ast.ASTRef, error) {
	if err := f.open(); err != nil {
		return nil, err
	}
	values, err := f.inner.GetLemmas(root, bAssertions)
	if err != nil {
		return nil, err
	}
	return simplifyBounds(f.manager, f.context, f.globals, values)
}

// CollectStatistics matches the source learner, which records no statistics.
func (f *FarkasLearner) CollectStatistics() map[string]float64 {
	if f.open() != nil {
		return nil
	}
	return nil
}

// ResetStatistics matches the source no-op while still enforcing lifetime.
func (f *FarkasLearner) ResetStatistics() error { return f.open() }

func (f *FarkasLearner) Close() error {
	if f == nil {
		return ast.ErrASTHandle
	}
	if f.closed {
		return nil
	}
	f.closed = true
	err := f.inner.Close()
	f.inner = nil
	f.manager = nil
	f.context = nil
	f.globals = nil
	return err
}

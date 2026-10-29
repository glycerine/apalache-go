package rewriter

// Source: src/ast/rewriter/bool_rewriter.h:86-105;
// src/ast/rewriter/bool_rewriter.cpp:23-87; src/model/model_evaluator.cpp:66-93.

import (
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// BoolRewriter is the original configured Boolean reducer. Traversal invokes its
// core through a Configuration; it does not traverse child expressions itself.
type BoolRewriter = boolRewriter

// NewBoolRewriter borrows the AST and parameter services. A nonnil partial owner
// returned with an error must be closed.
func NewBoolRewriter(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef) (*BoolRewriter, error) {
	return newBoolRewriter(m, c, g, p)
}
func (rw *boolRewriter) Flat() (bool, error) {
	if e := rw.configuredOpen(); e != nil {
		return false, e
	}
	return rw.flat, nil
}
func (rw *boolRewriter) SetFlat(flat bool) error {
	if e := rw.configuredOpen(); e != nil {
		return e
	}
	rw.flat = flat
	return nil
}

// SetEliminateAnd and the concrete gate factories serve the retained pb2bv
// tactic's private configured Boolean reducer.
func (rw *boolRewriter) SetEliminateAnd(enabled bool) error {
	if err := rw.configuredOpen(); err != nil {
		return err
	}
	rw.elimAnd = enabled
	return nil
}
func (rw *boolRewriter) MkAnd(args []ast.ASTHandle, out ast.ASTRef) error {
	if err := rw.configuredOpen(); err != nil {
		return err
	}
	return rw.mkAnd(args, out)
}

// MkOr is the configured lexical Boolean constructor used by QE-lite DER.
func (rw *boolRewriter) MkOr(args []ast.ASTHandle, out ast.ASTRef) error {
	if err := rw.configuredOpen(); err != nil {
		return err
	}
	return rw.mkOr(args, out)
}
func (rw *boolRewriter) MkNot(arg ast.ASTHandle, out ast.ASTRef) error {
	if err := rw.configuredOpen(); err != nil {
		return err
	}
	return rw.mkNot(arg, out)
}
func (rw *boolRewriter) MkIte(condition, then, otherwise ast.ASTHandle, out ast.ASTRef) error {
	if err := rw.configuredOpen(); err != nil {
		return err
	}
	return rw.mkITE(condition, then, otherwise, out)
}

// MkAppCore retains the source status and owning result-reference contract.
// RewriteFailed leaves result unchanged, as the original core does.
func (rw *boolRewriter) MkAppCore(d ast.ASTHandle, args ast.AppArguments, result ast.ASTRef) (RewriteStatus, error) {
	if e := rw.configuredOpen(); e != nil {
		return RewriteFailed, e
	}
	return rw.mkAppCoreArguments(d, args, result)
}

// SimplifyBooleanApplicationDefault is the concrete default-parameter
// bool_rewriter used by proof elimination. It owns no persistent parameter
// snapshot because the source constructs and destroys this reducer within one
// mk_app call.
func SimplifyBooleanApplicationDefault(m *ast.ASTManager, d ast.ASTHandle, args []ast.ASTHandle, result ast.ASTRef) error {
	if m == nil {
		return ast.ErrASTHandle
	}
	if _, err := m.ProofsEnabled(); err != nil {
		return err
	}
	rw := boolRewriter{manager: m, flat: true, localContextLimit: ^uint32(0)}
	return rw.mkApp(d, args, result)
}

// SimplifyBooleanGateDefault exposes the concrete Boolean operations borrowed
// by bit_blaster_cfg. The result reference owns the selected or constructed
// expression exactly as expr_ref does in the source configuration.
func SimplifyBooleanGateDefault(m *ast.ASTManager, operation int32, args []ast.ASTHandle, result ast.ASTRef) error {
	return SimplifyBooleanGateConfigured(m, operation, args, result, true, false)
}

// SimplifyBooleanGateConfigured exposes the two source gate-shape switches
// used by the bit-blaster rewriter's private Boolean reducer.
func SimplifyBooleanGateConfigured(m *ast.ASTManager, operation int32, args []ast.ASTHandle, result ast.ASTRef, flat, elimAnd bool) error {
	if m == nil {
		return ast.ErrASTHandle
	}
	rw := boolRewriter{manager: m, flat: flat, elimAnd: elimAnd, localContextLimit: ^uint32(0)}
	switch operation {
	case ast.ASTOpNot:
		if len(args) != 1 {
			return ast.ErrASTDomain
		}
		return rw.mkNot(args[0], result)
	case ast.ASTOpXor:
		if len(args) != 2 {
			return ast.ErrASTDomain
		}
		return rw.mkXor(args[0], args[1], result)
	case ast.ASTOpIff:
		if len(args) != 2 {
			return ast.ErrASTDomain
		}
		return rw.mkEq(args[0], args[1], result)
	case ast.ASTOpAnd:
		return rw.mkAnd(args, result)
	case ast.ASTOpOr:
		return rw.mkOr(args, result)
	case ast.ASTOpIte:
		if len(args) != 3 {
			return ast.ErrASTDomain
		}
		return rw.mkITE(args[0], args[1], args[2], result)
	default:
		return ast.ErrASTDomain
	}
}

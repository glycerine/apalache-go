package api

// Source: generated api_commands.cpp, Boolean construction commands.

import "github.com/glycerine/g3/pkg/ast"

func replayBooleanBinary(r *Z3Replayer, op uint8) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	a, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	b, err := r.astObject(owner, 2)
	if err != nil {
		return err
	}
	var h ast.ASTHandle
	switch op {
	case 0:
		h, err = owner.context.Iff(a.handle, b.handle)
	case 1:
		h, err = owner.context.Implies(a.handle, b.handle)
	case 2:
		h, err = owner.context.Xor(a.handle, b.handle)
	default:
		return ErrZ3ReplayState
	}
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayIff(r *Z3Replayer) error     { return replayBooleanBinary(r, 0) }
func replayImplies(r *Z3Replayer) error { return replayBooleanBinary(r, 1) }
func replayXor(r *Z3Replayer) error     { return replayBooleanBinary(r, 2) }
func replayIte(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	a, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	b, err := r.astObject(owner, 2)
	if err != nil {
		return err
	}
	c, err := r.astObject(owner, 3)
	if err != nil {
		return err
	}
	h, err := owner.context.Ite(a.handle, b.handle, c.handle)
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayBooleanArray(r *Z3Replayer, op uint8) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	args, err := replayASTArray(r, owner, 1, 2)
	if err != nil {
		return err
	}
	var h ast.ASTHandle
	switch op {
	case 0:
		h, err = owner.context.Distinct(args...)
	case 1:
		h, err = owner.context.And(args...)
	case 2:
		h, err = owner.context.Or(args...)
	default:
		return ErrZ3ReplayState
	}
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayDistinct(r *Z3Replayer) error { return replayBooleanArray(r, 0) }
func replayAnd(r *Z3Replayer) error      { return replayBooleanArray(r, 1) }
func replayOr(r *Z3Replayer) error       { return replayBooleanArray(r, 2) }

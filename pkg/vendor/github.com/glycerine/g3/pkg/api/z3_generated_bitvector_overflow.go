package api

// Source: generated api_commands.cpp, bit-vector overflow predicates.

import "github.com/glycerine/g3/pkg/ast"

func registerZ3GeneratedBitVectorOverflowCommands(r *Z3Replayer) error {
	entries := []struct {
		id   uint32
		name string
	}{
		{120, "Z3_mk_bvadd_no_overflow"}, {121, "Z3_mk_bvadd_no_underflow"}, {122, "Z3_mk_bvsub_no_overflow"},
		{123, "Z3_mk_bvsub_no_underflow"}, {124, "Z3_mk_bvsdiv_no_overflow"}, {125, "Z3_mk_bvneg_no_overflow"},
		{126, "Z3_mk_bvmul_no_overflow"}, {127, "Z3_mk_bvmul_no_underflow"},
	}
	for _, entry := range entries {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error {
			owner, err := replayContext(r, 0)
			if err != nil {
				return err
			}
			left, err := r.astObject(owner, 1)
			if err != nil {
				return err
			}
			var right *replayASTOwner
			if id != 125 {
				right, err = r.astObject(owner, 2)
				if err != nil {
					return err
				}
			}
			var signed bool
			if id == 120 || id == 123 || id == 126 {
				signed, err = r.Bool(3)
				if err != nil {
					return err
				}
			}
			var h ast.ASTHandle
			switch id {
			case 120:
				h, err = owner.context.BitVectorAddNoOverflow(left.handle, right.handle, signed)
			case 121:
				h, err = owner.context.BitVectorAddNoUnderflow(left.handle, right.handle)
			case 122:
				h, err = owner.context.BitVectorSubNoOverflow(left.handle, right.handle)
			case 123:
				h, err = owner.context.BitVectorSubNoUnderflow(left.handle, right.handle, signed)
			case 124:
				h, err = owner.context.BitVectorSDivNoOverflow(left.handle, right.handle)
			case 125:
				h, err = owner.context.BitVectorNegNoOverflow(left.handle)
			case 126:
				h, err = owner.context.BitVectorMulNoOverflow(left.handle, right.handle, signed)
			case 127:
				h, err = owner.context.BitVectorMulNoUnderflow(left.handle, right.handle)
			}
			if err != nil {
				return err
			}
			return r.storeAST(owner, h)
		}); err != nil {
			return err
		}
	}
	return nil
}

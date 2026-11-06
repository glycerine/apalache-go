package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp:43-44,120;
// src/tactic/arith/bv2real_rewriter.h:125,142;
// src/util/ref_vector.h:39-59,90-94; src/util/vector.h:59-99.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// Concrete storage for live BV2Real and NLA2BV AST-handle vectors. Reset releases
// their references while preserving capacity; destruction frees the allocation.
func (u *BV2RealUtil) appendVectorRef(values *[]ast.ASTHandle, charged *uint64, h ast.ASTHandle) error {
	if err := u.manager.IncRef(h); err != nil {
		return err
	}
	rollback := func(cause error) error {
		consumed, err := u.manager.DecRefConsumed(h)
		if !consumed {
			u.vectorPending = append(u.vectorPending, h)
		}
		return errors.Join(cause, err)
	}
	if len(*values) == cap(*values) {
		capacity := uint64(cap(*values))
		next := uint64(2)
		if capacity != 0 {
			next = (3*capacity + 1) >> 1
		}
		if next <= capacity || next > uint64(^uint32(0)) {
			return rollback(arena.ErrBounds)
		}
		bytes := next*uint64(reflect.TypeOf(ast.ASTHandle{}).Size()) + 8
		if err := u.manager.Budget().Reserve(bytes - *charged); err != nil {
			return rollback(err)
		}
		grown := make([]ast.ASTHandle, len(*values), int(next))
		copy(grown, *values)
		*values, *charged = grown, bytes
	}
	*values = append(*values, h)
	return nil
}

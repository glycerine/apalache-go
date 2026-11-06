package fp

// Source: dl_compiler.cpp make_duplicate_column and dl_instruction.cpp JOIN.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func (c *externalIntervalDatalogCartesianContinuation) duplicateColumn(column uint32) (bool, error) {
	sorts, err := c.temporary.Sorts()
	if err != nil {
		return false, err
	}
	if uint64(column) >= uint64(len(sorts)) {
		return false, ast.ErrASTDomain
	}
	right := c.temporary
	if len(sorts) != 1 {
		removed := make([]uint32, 0, len(sorts)-1)
		for index := range sorts {
			if uint32(index) != column {
				removed = append(removed, uint32(index))
			}
		}
		c.auxiliary, err = c.temporary.Project(removed)
		if err != nil {
			return false, err
		}
		right = c.auxiliary
	}
	joined, err := rel.JoinExternalIntervalProducts(c.temporary, right, []uint32{column}, []uint32{0})
	if joined != nil {
		// Adopt the JOIN output before retiring its input, including any
		// returned product accompanying an error from the lower operation.
		err = errors.Join(err, c.replaceTemporary(joined))
	}
	if err != nil {
		return false, err
	}
	empty, err := c.temporary.Empty()
	if err != nil || empty {
		// The empty JOIN register and projected column stay together until
		// the caller finishes the rule or rolls back the failed operation.
		return empty, err
	}
	// The source DEALLOC follows JOIN's emptiness check. For unary input the
	// right side aliases the old input, so no auxiliary register was created.
	if c.auxiliary != nil {
		if err := c.auxiliary.Close(); err != nil {
			return false, err
		}
		c.auxiliary = nil
	}
	return false, nil
}

package fp

// Source: dl_compiler.cpp make_add_constant_column and make_assembling_code.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func (c *externalIntervalDatalogCartesianContinuation) closeProducts() error {
	if c.temporary != nil {
		if err := c.temporary.Close(); err != nil {
			return err
		}
		c.temporary = nil
	}
	if c.retiring != nil {
		if err := c.retiring.Close(); err != nil {
			return err
		}
		c.retiring = nil
	}
	if c.auxiliary != nil {
		if err := c.auxiliary.Close(); err != nil {
			return err
		}
		c.auxiliary = nil
	}
	return nil
}

func (r *externalIntervalDatalogRegisters) initializeCartesianHeadConstants(plan *externalIntervalDatalogCartesianRule) error {
	return r.initializeHeadAssembly(plan.head, plan.headAssemblyAdditions(), plan.headConstants, plan.headUnbound)
}

func (r *externalIntervalDatalogRegisters) initializeHeadAssembly(head ast.ASTHandle, additions []externalIntervalCartesianHeadAddition, constants []externalIntervalCartesianHeadConstant, unbound []externalIntervalCartesianHeadVariable) error {
	for _, addition := range additions {
		if addition.total {
			variable := unbound[addition.index]
			if _, err := r.totalRelation(head, variable.sort); err != nil {
				return err
			}
		} else {
			constant := constants[addition.index]
			if _, err := r.unarySingleton(head, constant.sort, constant.value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *externalIntervalDatalogCartesianContinuation) appendHeadTotal(head ast.ASTHandle, variable externalIntervalCartesianHeadVariable) (bool, error) {
	total, err := c.registers.totalRelation(head, variable.sort)
	if err != nil {
		return false, err
	}
	joined, err := rel.JoinExternalIntervalProducts(c.temporary, total, nil, nil)
	if joined != nil {
		err = errors.Join(err, c.replaceTemporary(joined))
	}
	if err != nil {
		return false, err
	}
	return c.temporary.Empty()
}

func (c *externalIntervalDatalogCartesianContinuation) appendHeadConstant(head ast.ASTHandle, constant externalIntervalCartesianHeadConstant) (bool, error) {
	singleton, err := c.registers.unarySingleton(head, constant.sort, constant.value)
	if err != nil {
		return false, err
	}
	// Even a nullary current is a real producer register. Its first constant
	// is appended with JOIN; only the singleton itself is borrowed from cache.
	joined, err := rel.JoinExternalIntervalProducts(c.temporary, singleton, nil, nil)
	if joined != nil {
		err = errors.Join(err, c.replaceTemporary(joined))
	}
	if err != nil {
		return false, err
	}
	return c.temporary.Empty()
}

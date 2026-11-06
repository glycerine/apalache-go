package fp

// Source: dl_compiler.cpp compile_rule adds full columns for free variables
// in the interpreted conjunction before constructing its column substitution.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func (c *datalogUDocContext) addUDocConditionColumns(working **rel.TypedUDocRelation, rule *base.Rule, first, total int, variables map[uint32]uint32) error {
	if first == total {
		return nil
	}
	if working == nil {
		return ErrUDocDatalogFragment
	}
	used, err := ast.NewASTUsedVars(c.manager)
	if err != nil {
		return err
	}
	for i := first; i < total; i++ {
		condition, negated, cause := rule.Tail(i)
		if cause != nil {
			return cause
		}
		if negated {
			return ErrUDocDatalogFragment
		}
		if cause = used.Process(condition); cause != nil {
			return cause
		}
	}
	// Validate every missing column before changing the working relation.
	sorts := []ast.ASTHandle(nil)
	if *working != nil {
		sorts = (*working).Sorts()
	}
	for variable := uint32(0); variable < used.MaxFoundVarIndexPlusOne(); variable++ {
		sort := used.Contains(variable)
		if sort.IsNull() {
			continue
		}
		if _, found := variables[variable]; found {
			continue
		}
		supported, cause := c.factory.CanHandleSignature([]ast.ASTHandle{sort})
		if cause != nil {
			return cause
		}
		if !supported {
			return fmt.Errorf("%w: unsupported interpreted variable sort", ErrUDocDatalogFragment)
		}
	}
	for variable := uint32(0); variable < used.MaxFoundVarIndexPlusOne(); variable++ {
		sort := used.Contains(variable)
		if sort.IsNull() {
			continue
		}
		if _, found := variables[variable]; found {
			continue
		}
		if err = c.checkpoint(); err != nil {
			return err
		}
		appended, appendedSorts, index, cause := c.appendFullColumn(*working, sorts, sort)
		if cause != nil {
			return cause
		}
		*working, sorts = appended, appendedSorts
		variables[variable] = index
	}
	// A variable-free interpreted conjunction still filters one logical
	// nullary row when there was no positive or negative column source.
	return c.ensureFullNullary(working)
}

package fp

// Source: dl_compiler.cpp's make_add_unbound_column, make_full_relation,
// and rule-wide validation performed before compiled instructions execute.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func (c *datalogUDocContext) appendFullColumn(working *rel.TypedUDocRelation, sorts []ast.ASTHandle, sortHandle ast.ASTHandle) (*rel.TypedUDocRelation, []ast.ASTHandle, uint32, error) {
	column, err := c.factory.NewFull([]ast.ASTHandle{sortHandle})
	if err != nil {
		return nil, nil, 0, err
	}
	if working == nil {
		return column, []ast.ASTHandle{sortHandle}, 0, nil
	}
	joined, err := working.Join(column, nil, nil)
	if err != nil {
		return nil, nil, 0, errors.Join(err, column.Close())
	}
	if err = errors.Join(working.Close(), column.Close()); err != nil {
		return nil, nil, 0, errors.Join(err, joined.Close())
	}
	return joined, append(sorts, sortHandle), uint32(len(sorts)), nil
}

func (c *datalogUDocContext) ensureFullNullary(working **rel.TypedUDocRelation) error {
	if working == nil {
		return ErrUDocDatalogFragment
	}
	if *working != nil {
		return nil
	}
	full, err := c.factory.NewFull(nil)
	if err != nil {
		return err
	}
	*working = full
	return nil
}

// projectUDocHeadInputs mirrors make_assembling_code's first projection: body
// columns absent from the head are removed before full, singleton, or duplicate
// head columns are appended.
func (c *datalogUDocContext) projectUDocHeadInputs(working *rel.TypedUDocRelation, sorts []ast.ASTHandle, head ast.ASTHandle, variables map[uint32]uint32) (*rel.TypedUDocRelation, []ast.ASTHandle, error) {
	if working == nil {
		return nil, sorts, nil
	}
	referenced := make(map[uint32]bool)
	count, err := c.manager.GetNumArgs(head)
	if err != nil {
		return nil, nil, err
	}
	for column := uint32(0); column < count; column++ {
		argument, cause := c.manager.GetArg(head, column)
		if cause != nil {
			return nil, nil, cause
		}
		kind, cause := c.manager.GetKind(argument)
		if cause != nil {
			return nil, nil, cause
		}
		if kind != ast.ASTVar {
			continue
		}
		variable, cause := c.manager.GetVarIndex(argument)
		if cause != nil {
			return nil, nil, cause
		}
		if source, found := variables[variable]; found {
			referenced[source] = true
		}
	}
	remove := make([]uint32, 0, len(sorts)-len(referenced))
	removed := make(map[uint32]bool)
	for column := range sorts {
		if !referenced[uint32(column)] {
			remove = append(remove, uint32(column))
			removed[uint32(column)] = true
		}
	}
	if len(remove) == 0 {
		return working, sorts, nil
	}
	projected, err := working.Project(remove)
	if err != nil {
		return nil, nil, err
	}
	if err = working.Close(); err != nil {
		return nil, nil, errors.Join(err, projected.Close())
	}
	for variable, source := range variables {
		if removed[source] {
			delete(variables, variable)
			continue
		}
		var shift uint32
		for _, removedColumn := range remove {
			if removedColumn < source {
				shift++
			}
		}
		variables[variable] = source - shift
	}
	return projected, projected.Sorts(), nil
}

func (c *datalogUDocContext) recordUDocVariableSort(sorts map[uint32]ast.ASTHandle, variable uint32, sortHandle ast.ASTHandle) error {
	if prior, found := sorts[variable]; found && prior != sortHandle {
		return fmt.Errorf("%w: variable %d has inconsistent sorts", ErrUDocDatalogFragment, variable)
	}
	sorts[variable] = sortHandle
	return nil
}

func (c *datalogUDocContext) preflightUDocApplication(application ast.ASTHandle, variables map[uint32]ast.ASTHandle) error {
	declaration, err := c.manager.GetDecl(application)
	if err != nil {
		return err
	}
	if _, finite := c.nullaryResult[declaration]; finite {
		count, err := c.manager.GetNumArgs(application)
		if err != nil || count != 0 {
			return errors.Join(err, ErrUDocDatalogFragment)
		}
		return nil
	}
	relation := c.result[declaration]
	if relation == nil {
		return fmt.Errorf("%w: undeclared relation", ErrUDocDatalogFragment)
	}
	expected := relation.Sorts()
	count, err := c.manager.GetNumArgs(application)
	if err != nil || int(count) != len(expected) {
		return errors.Join(err, ErrUDocDatalogFragment)
	}
	for column := uint32(0); column < count; column++ {
		argument, cause := c.manager.GetArg(application, column)
		if cause != nil {
			return cause
		}
		classified, cause := c.classifyUDocArgument(argument, expected[column])
		if cause != nil {
			return cause
		}
		if classified.variable {
			if cause = c.recordUDocVariableSort(variables, classified.index, classified.sort); cause != nil {
				return cause
			}
		}
	}
	return nil
}

// preflightUDocRule validates every source operation that can allocate a full
// column before saturation publishes even an earlier ground rule.
func (c *datalogUDocContext) preflightUDocRule(rule *base.Rule) error {
	_, finiteSource, err := classifyUDocFiniteForwarding(c.manager, rule, c.finiteInputs)
	if err != nil {
		return err
	}
	if !finiteSource.IsNull() {
		return nil
	}
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return err
	}
	total, err := rule.TailSize()
	if err != nil || positive < 0 || uninterpreted < positive || total < uninterpreted {
		return errors.Join(err, ErrUDocDatalogFragment)
	}
	variables := make(map[uint32]ast.ASTHandle)
	for index := 0; index < positive; index++ {
		literal, negative, cause := rule.Tail(index)
		if cause != nil || negative {
			return errors.Join(cause, ErrUDocDatalogFragment)
		}
		if cause = c.preflightUDocApplication(literal, variables); cause != nil {
			return cause
		}
	}
	negativeTails, err := c.negativeRuleTails(rule)
	if err != nil {
		return err
	}
	for _, tail := range negativeTails {
		for _, argument := range tail.arguments {
			if argument.variable {
				if err = c.recordUDocVariableSort(variables, argument.index, argument.sort); err != nil {
					return err
				}
			}
		}
	}
	used, err := ast.NewASTUsedVars(c.manager)
	if err != nil {
		return err
	}
	for index := uninterpreted; index < total; index++ {
		condition, negative, cause := rule.Tail(index)
		if cause != nil || negative {
			return errors.Join(cause, ErrUDocDatalogFragment)
		}
		if cause = used.Process(condition); cause != nil {
			return cause
		}
	}
	for variable := uint32(0); variable < used.MaxFoundVarIndexPlusOne(); variable++ {
		sortHandle := used.Contains(variable)
		if sortHandle.IsNull() {
			continue
		}
		if err = c.recordUDocVariableSort(variables, variable, sortHandle); err != nil {
			return err
		}
	}
	head, err := rule.Head()
	if err != nil {
		return err
	}
	declaration, err := c.manager.GetDecl(head)
	if err != nil {
		return err
	}
	if _, finite := c.nullaryResult[declaration]; finite {
		return ErrUDocDatalogFragment
	}
	if err = c.preflightUDocApplication(head, variables); err != nil {
		return err
	}
	for _, sortHandle := range variables {
		supported, cause := c.factory.CanHandleSignature([]ast.ASTHandle{sortHandle})
		if cause != nil {
			return cause
		}
		if !supported {
			return fmt.Errorf("%w: unsupported full-column sort", ErrUDocDatalogFragment)
		}
	}
	return nil
}

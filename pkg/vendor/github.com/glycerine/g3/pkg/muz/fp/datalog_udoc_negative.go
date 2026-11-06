package fp

// Source: dl_compiler.cpp's variable_intersection mapping and negative-tail
// filter instructions, dispatched to the registered UDoc plugin.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type udocNegativeArgument struct {
	index    uint32
	sort     ast.ASTHandle
	value    ast.ASTHandle
	variable bool
}

type udocNegativeTail struct {
	declaration ast.ASTHandle
	arguments   []udocNegativeArgument
}

func (c *datalogUDocContext) classifyUDocArgument(argument, expectedSort ast.ASTHandle) (udocNegativeArgument, error) {
	result := udocNegativeArgument{sort: expectedSort}
	actualSort, err := c.manager.GetSort(argument)
	if err != nil || actualSort != expectedSort {
		return result, errors.Join(err, ErrUDocDatalogFragment)
	}
	kind, err := c.manager.GetKind(argument)
	if err != nil {
		return result, err
	}
	if kind == ast.ASTVar {
		result.index, err = c.manager.GetVarIndex(argument)
		result.variable = true
		return result, err
	}
	value, err := c.manager.IsValue(argument)
	if err != nil || !value {
		return result, errors.Join(err, ErrUDocDatalogFragment)
	}
	result.value = argument
	return result, nil
}

func (c *datalogUDocContext) negativeRuleTails(rule *base.Rule) ([]udocNegativeTail, error) {
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return nil, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil || positive == uninterpreted {
		return nil, err
	}
	result := make([]udocNegativeTail, 0, uninterpreted-positive)
	for index := positive; index < uninterpreted; index++ {
		literal, negative, err := rule.Tail(index)
		if err != nil || !negative {
			return nil, errors.Join(err, ErrUDocDatalogFragment)
		}
		declaration, err := c.manager.GetDecl(literal)
		relation := c.result[declaration]
		if err != nil || relation == nil {
			return nil, errors.Join(err, ErrUDocDatalogFragment)
		}
		sorts := relation.Sorts()
		count, err := c.manager.GetNumArgs(literal)
		if err != nil || int(count) != len(sorts) {
			return nil, errors.Join(err, ErrUDocDatalogFragment)
		}
		tail := udocNegativeTail{declaration: declaration, arguments: make([]udocNegativeArgument, 0, count)}
		for column := uint32(0); column < count; column++ {
			argument, err := c.manager.GetArg(literal, column)
			if err != nil {
				return nil, err
			}
			classified, err := c.classifyUDocArgument(argument, sorts[column])
			if err != nil {
				return nil, err
			}
			tail.arguments = append(tail.arguments, classified)
		}
		result = append(result, tail)
	}
	return result, nil
}

func (c *datalogUDocContext) addUDocNegativeColumns(working **rel.TypedUDocRelation, sorts *[]ast.ASTHandle, tails []udocNegativeTail, variables map[uint32]uint32) error {
	if working == nil || sorts == nil {
		return ErrUDocDatalogFragment
	}
	for _, tail := range tails {
		for _, argument := range tail.arguments {
			if !argument.variable {
				continue
			}
			if _, found := variables[argument.index]; found {
				continue
			}
			if err := c.checkpoint(); err != nil {
				return err
			}
			appended, appendedSorts, column, err := c.appendFullColumn(*working, *sorts, argument.sort)
			if err != nil {
				return err
			}
			*working, *sorts = appended, appendedSorts
			variables[argument.index] = column
		}
	}
	return nil
}

func (c *datalogUDocContext) filterNegativeTails(working **rel.TypedUDocRelation, sorts *[]ast.ASTHandle, tails []udocNegativeTail, variables map[uint32]uint32) error {
	if len(tails) == 0 {
		return nil
	}
	if working == nil || sorts == nil {
		return ErrUDocDatalogFragment
	}
	if err := c.ensureFullNullary(working); err != nil {
		return err
	}
	for _, tail := range tails {
		if err := c.checkpoint(); err != nil {
			return err
		}
		columns := make([]uint32, len(tail.arguments))
		negativeColumns := make([]uint32, len(tail.arguments))
		for index, argument := range tail.arguments {
			if argument.variable {
				column, found := variables[argument.index]
				if !found {
					return ErrUDocDatalogFragment
				}
				columns[index] = column
			} else {
				appended, appendedSorts, column, err := c.appendConstantColumn(*working, *sorts, argument.sort, argument.value)
				if err != nil {
					return err
				}
				*working, *sorts = appended, appendedSorts
				columns[index] = column
			}
			negativeColumns[index] = uint32(index)
		}
		if err := (*working).FilterByNegation(c.result[tail.declaration], columns, negativeColumns); err != nil {
			return err
		}
	}
	return nil
}

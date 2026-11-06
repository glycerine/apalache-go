package api

// Source: src/api/api_parsers.cpp parse_smtlib2_stream temporary cmd_context.

import (
	"errors"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func (c *SMTContext) newSMT2ParserContext(bindings SMT2Bindings) (result *cmd_context.Context, resultErr error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	result, err := c.command.NewParserContext()
	if err != nil {
		return result, err
	}
	defer func() {
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	for _, binding := range bindings.Declarations {
		if _, err = result.InsertFunction(binding.Name, binding.Declaration); err != nil {
			return result, err
		}
	}
	for _, binding := range bindings.Sorts {
		if err = result.InsertSort(binding.Name, binding.Sort); err != nil {
			return result, err
		}
	}
	return result, nil
}

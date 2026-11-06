package cmd_context

// Source: src/api/api_parsers.cpp parse_smtlib2_stream constructs a temporary
// cmd_context that borrows the API context's ast_manager.

import (
	"errors"
)

// NewParserContext creates a command view over the same AST manager and
// parameter services. The returned context owns its declarations, assertions,
// options and references, but never closes the shared manager.
func (c *Context) NewParserContext() (result *Context, resultErr error) {
	if err := c.valid(); err != nil {
		return nil, err
	}
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	result, err = NewContext(c.config)
	if err != nil {
		return result, err
	}
	defer func() {
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	if err = c.config.Params.CheckServices(manager.Symbols, manager.NumeralService()); err != nil {
		return result, err
	}
	result.manager = manager
	result.borrowedManager = true
	if err = result.initBuiltins(); err != nil {
		return result, err
	}
	// Assertions retain the ordinary cmd_context publication path even when
	// check commands are ignored, so the temporary view needs its own solver.
	if err = result.mkSolver(manager); err != nil {
		return result, err
	}
	return result, nil
}

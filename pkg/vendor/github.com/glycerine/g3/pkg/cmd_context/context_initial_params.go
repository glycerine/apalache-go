package cmd_context

// Source: src/api/api_context.cpp context::context configuration copy.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/smtcore"
)

// SetInitialContextParams copies construction options before the manager is
// initialized. The source and destination remain independently owned.
func (c *Context) SetInitialContextParams(source *smtcore.ContextParams) error {
	if err := c.valid(); err != nil {
		return err
	}
	if c.manager != nil || c.initializationError != nil {
		return fmt.Errorf("context is already initialized")
	}
	if err := c.parameters.CopyValuesFrom(source); err != nil {
		return err
	}
	state, err := c.parameters.State()
	if err != nil {
		return err
	}
	c.PrintSuccess = state.SMTLib2Compliant
	return nil
}

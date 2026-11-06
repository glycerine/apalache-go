package api

// Source: src/api/api_context.cpp:121-145,358-363.

import "github.com/glycerine/g3/pkg/util/control"

// Interrupt notifies the currently borrowed operation handler with the
// source API-interrupt caller identifier, then cancels the context AST limit.
func (c *SMTContext) Interrupt() error {
	if err := c.open(); err != nil {
		return err
	}
	manager, err := c.command.Manager()
	if err != nil {
		return err
	}
	c.interruptMu.Lock()
	defer c.interruptMu.Unlock()
	if c.interruptable != nil {
		c.interruptable.HandleEvent(control.EventCallerAPIInterrupt)
	}
	c.polyLimit.Cancel()
	manager.ResourceLimit().Cancel()
	return nil
}

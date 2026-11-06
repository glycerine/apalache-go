package api

// Source: src/api/api_tactic.cpp:378-389.

import "github.com/glycerine/g3/pkg/cmd_context"

// TacticDescription returns the retained source registration text for name.
func (c *SMTContext) TacticDescription(name string) (string, error) {
	if err := c.open(); err != nil {
		return "", err
	}
	return cmd_context.BuiltinTacticDescription(apiTacticName(name))
}

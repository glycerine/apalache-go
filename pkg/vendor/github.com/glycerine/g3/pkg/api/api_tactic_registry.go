package api

// Source: src/api/api_tactic.cpp:314-336. Go returns copied strings and
// slices rather than native borrowed string pointers.

import "github.com/glycerine/g3/pkg/cmd_context"

// TacticCount returns the number of tactics backed by retained concrete
// factories.
func (c *SMTContext) TacticCount() (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	return cmd_context.BuiltinTacticCount(), nil
}

// TacticName returns the source-ordered retained tactic name at index.
func (c *SMTContext) TacticName(index uint32) (string, error) {
	if err := c.open(); err != nil {
		return "", err
	}
	return cmd_context.BuiltinTacticName(index)
}

// TacticNames returns a caller-owned registry snapshot.
func (c *SMTContext) TacticNames() ([]string, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return cmd_context.BuiltinTacticNames(), nil
}

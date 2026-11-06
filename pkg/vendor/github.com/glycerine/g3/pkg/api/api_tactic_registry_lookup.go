package api

// Source: src/api/api_tactic.cpp Z3_mk_tactic lookup boundary.

import (
	"github.com/glycerine/g3/pkg/cmd_context"
	"strings"
)

func apiTacticName(name string) string { prefix, _, _ := strings.Cut(name, "\x00"); return prefix }

// HasTactic reports whether name identifies a retained concrete tactic
// factory. It distinguishes discovery from construction, which may allocate
// the full tactic pipeline.
func (c *SMTContext) HasTactic(name string) (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	return cmd_context.IsBuiltinTactic(apiTacticName(name)), nil
}

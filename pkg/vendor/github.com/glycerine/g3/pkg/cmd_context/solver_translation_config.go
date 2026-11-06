package cmd_context

// Source: src/api/api_solver.cpp Z3_solver_translate target context services.

import "github.com/glycerine/g3/pkg/smtcore"

// SolverTranslationConfig borrows this context's solver services. They remain
// valid until the context is closed.
func (c *Context) SolverTranslationConfig() (smtcore.SMTSolverConfig, error) {
	if err := c.valid(); err != nil {
		return smtcore.SMTSolverConfig{}, err
	}
	return c.config, nil
}

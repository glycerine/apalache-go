package api

// Source: src/api/api_tactic.cpp:48-67; src/tactic/probe.cpp retained
// primitive probe factories.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// NamedProbe constructs a retained primitive probe using its native name.
// The returned probe owns one reference and must be closed.
func (c *SMTContext) NamedProbe(name string) (result *APIProbe, resultErr error) {
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	registration, found := apiProbeRegistrationByName(name)
	if !found {
		return nil, ast.ErrASTDomain
	}
	probe, err := c.newRegisteredProbe(manager, registration)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, probe.Close())
		}
	}()
	result, resultErr = newAPIProbe(c.smtContextState, probe)
	return result, resultErr
}

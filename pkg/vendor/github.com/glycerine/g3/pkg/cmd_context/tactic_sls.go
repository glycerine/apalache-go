package cmd_context

// Source: src/tactic/sls/sls_tactic.h qfbv-sls command registration.

import (
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/sls"
)

func (c *Context) newQFBVSLSTactic() (tactic.Tactic, error) {
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	config, err := sls.QFBVConfigFromSolver(c.config)
	if err != nil {
		return nil, err
	}
	return sls.NewQFBVSLSTactic(manager, nil, config)
}

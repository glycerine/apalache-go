package api

// Source: src/api/api_qe.cpp public QE entry-point service borrowing.

import (
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/qe"
)

func (c *SMTContext) apiQEConfig() (qe.Config, error) {
	if err := c.open(); err != nil {
		return qe.Config{}, err
	}
	services, err := c.command.ModelEvaluatorConfig()
	if err != nil {
		return qe.Config{}, err
	}
	return qe.Config{
		Context:     services.Context,
		Globals:     services.Globals,
		Cooperation: services.Cooperation,
		Trace:       services.Trace,
	}, nil
}

func (c *SMTContext) checkQEModel(value *model.ModelCore) error {
	if err := c.open(); err != nil {
		return err
	}
	manager, err := c.Manager()
	if err != nil {
		return err
	}
	if value == nil || value.Manager() != manager {
		return ErrSMTContextOwner
	}
	return value.Check()
}

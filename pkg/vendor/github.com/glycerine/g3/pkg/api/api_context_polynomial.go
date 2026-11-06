package api

// Source: src/api/api_context.h:201-205; src/api/api_polynomial.h:29-38;
// src/api/api_polynomial.cpp:35-41. These owners are independent of the
// arithmetic plugin's algebraic-number manager and its AST resource limit.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/util/control"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type apiPolynomialManager struct {
	failure error
	// The concrete Go polynomial overloads use both rational and integer
	// temporaries. This private wrapper supplies their shared integer domain;
	// its runtime remains borrowed from the owning context.
	rationals         *unumeral.RationalManager
	unfinishedInteger *unumeral.IntegerManager
	manager           *polynomial.MultivariatePolynomialManager
}

func (p *apiPolynomialManager) close() error {
	if p == nil {
		return nil
	}
	if p.manager != nil {
		if err := p.manager.CloseChecked(); err != nil {
			return err
		}
		p.manager = nil
	}
	if p.rationals != nil {
		if err := p.rationals.CloseChecked(); err != nil {
			return err
		}
		p.rationals = nil
	}
	if p.unfinishedInteger != nil {
		if err := p.unfinishedInteger.CloseChecked(); err != nil {
			return err
		}
		p.unfinishedInteger = nil
	}
	return nil
}

func (c *SMTContext) polynomialLimit() (*control.Reslimit, error) {
	m, err := c.Manager()
	if err != nil {
		return nil, err
	}
	c.polyLimit.InitEmbedded(m.Budget())
	return &c.polyLimit, nil
}

// polynomialManager is initialized with the Go context's lazy AST services;
// it persists across calls and closes before those borrowed services.
func (c *SMTContext) polynomialManager() (*polynomial.MultivariatePolynomialManager, error) {
	m, err := c.Manager()
	if err != nil {
		return nil, err
	}
	if c.polynomial != nil {
		return c.polynomial.manager, c.polynomial.failure
	}
	limit, err := c.polynomialLimit()
	if err != nil {
		return nil, err
	}
	config, err := c.command.ModelEvaluatorConfig()
	if err != nil {
		return nil, err
	}
	p := &apiPolynomialManager{}
	p.rationals, err = m.NumeralService().NewPrivateRationalManager()
	if err != nil {
		// Failed numerical construction may itself own unfinished cleanup.
		var unfinished *unumeral.RationalManager
		if errors.As(err, &unfinished) {
			p.rationals = unfinished
		} else {
			errors.As(err, &p.unfinishedInteger)
		}
	} else {
		p.manager, err = polynomial.NewMultivariatePolynomialManager(polynomial.MultivariatePolynomialManagerConfig{
			Core: polynomial.MultivariatePolynomialConfig{
				Integer: p.rationals.Integers(), Rationals: p.rationals, Checkpoint: limit.Inc,
				Cooperation: func() error { return config.Cooperation.Cooperate("polynomial") },
			},
		})
	}
	if err != nil {
		if cleanupErr := p.close(); cleanupErr != nil {
			// Preserve the owner for Close to finish before runtime teardown.
			p.failure = errors.Join(err, cleanupErr)
			c.polynomial = p
			return nil, p.failure
		}
		return nil, err
	}
	c.polynomial = p
	return p.manager, nil
}

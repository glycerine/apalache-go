package cmd_context

// Source: src/opt/opt_context.cpp:950-997,1062-1137,336-343.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/opt"
)

// The same definitions must survive all calls in one frontier enumeration.
type paretoPurification struct {
	purifier   *opt.ObjectivePurifier
	objectives []optimizationObjective
	hard       []ast.ASTHandle
}

func (c *Context) prepareParetoPurification() (resultErr error) {
	purifier, err := opt.NewObjectivePurifier(c.manager)
	if err != nil {
		return err
	}
	state := &paretoPurification{purifier: purifier}
	c.optimizationParetoPurification = state
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, c.closeParetoBackend())
		}
	}()
	objectives := make([]opt.DecomposedObjective, len(c.objectives))
	for i, objective := range c.objectives {
		kind := opt.FormulaMinimize
		if objective.maximize {
			kind = opt.FormulaMaximize
		}
		objectives[i] = opt.DecomposedObjective{SourceKind: kind, Term: objective.expression, OriginalTerm: objective.expression}
		if objective.soft {
			objectives[i].SourceKind = opt.FormulaMaxSAT
			objectives[i].Term = ast.ASTHandle{}
			for _, term := range objective.softTerms {
				objectives[i].SoftTerms = append(objectives[i].SoftTerms, term.expression)
				objectives[i].Weights = append(objectives[i].Weights, *new(big.Rat).Set(&term.weight))
			}
		}
	}
	hard := make([]ast.ASTHandle, len(c.assertions))
	for i := range c.assertions {
		hard[i] = c.assertions[i].expression
	}
	set, err := purifier.Purify(hard, objectives)
	if err != nil {
		return err
	}
	state.hard = set.Hard
	state.objectives = append([]optimizationObjective(nil), c.objectives...)
	for i, objective := range set.Objectives {
		if !state.objectives[i].soft {
			state.objectives[i].expression = objective.Term
			continue
		}
		state.objectives[i].softTerms = make([]optimizationSoftTerm, len(objective.SoftTerms))
		for j, term := range objective.SoftTerms {
			state.objectives[i].softTerms[j].expression = term
			state.objectives[i].softTerms[j].weight.Set(&objective.Weights[j])
		}
	}
	return nil
}

// Keep the raw solver model for comparisons; only the published copy is fixed.
func (c *Context) filterParetoModel(raw *model.ModelCore) (*model.ModelCore, error) {
	return c.optimizationParetoPurification.purifier.FilterModel(raw)
}

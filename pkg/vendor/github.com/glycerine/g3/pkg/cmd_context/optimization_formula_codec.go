package cmd_context

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
)

// EncodeOptimizationFormulas retains opt_context::to_fmls: hard constraints
// are followed by fresh Boolean objective-marker applications.
func (c *Context) EncodeOptimizationFormulas() (*opt.ObjectiveFormulaSet, error) {
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	hard := make([]ast.ASTHandle, len(c.assertions))
	for i := range c.assertions {
		hard[i] = c.assertions[i].expression
	}
	objectives := make([]opt.FormulaObjective, len(c.objectives))
	for i := range c.objectives {
		source := &c.objectives[i]
		if !source.soft {
			kind := opt.FormulaMinimize
			if source.maximize {
				kind = opt.FormulaMaximize
			}
			objectives[i] = opt.FormulaObjective{Kind: kind, Term: source.expression}
			continue
		}
		objective := opt.FormulaObjective{Kind: opt.FormulaMaxSAT, ID: source.id, SoftTerms: make([]ast.ASTHandle, len(source.softTerms)), Weights: make([]*big.Rat, len(source.softTerms))}
		for j := range source.softTerms {
			objective.SoftTerms[j] = source.softTerms[j].expression
			objective.Weights[j] = new(big.Rat).Set(&source.softTerms[j].weight)
		}
		objectives[i] = objective
	}
	return opt.NewObjectiveFormulaSet(manager, hard, objectives)
}

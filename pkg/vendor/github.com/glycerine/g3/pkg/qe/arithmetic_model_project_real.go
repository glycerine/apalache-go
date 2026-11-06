package qe

import "github.com/glycerine/g3/pkg/ast"

func (c *arithmeticModelProjectCall) realBranch(entry *arithmeticPluginBounds, target ast.ASTHandle) (uint32, error) {
	policy, err := c.plugin.branchPolicy(entry, target)
	if err != nil {
		return 0, err
	}
	direction := ArithmeticUpperBound
	if policy.Lower {
		direction = ArithmeticLowerBound
	}
	non := entry.processor.Bounds(false, direction)
	strict := entry.processor.Bounds(true, direction)
	nonValue, nonIndex, nonFound, err := c.findMinMax(non, policy.Lower)
	if err != nil {
		return 0, err
	}
	strictValue, strictIndex, strictFound, err := c.findMinMax(strict, policy.Lower)
	if err != nil {
		return 0, err
	}
	if !nonFound && !strictFound {
		return 0, nil
	}
	if strictFound && (!nonFound || strictValue.Cmp(nonValue) <= 0) {
		return 1 + 2*uint32(len(non)) + strictIndex, nil
	}
	variableValue, err := c.numeral(target)
	if err != nil {
		return 0, err
	}
	branch := 1 + 2*nonIndex
	if variableValue.Cmp(nonValue) != 0 {
		branch++
	}
	return branch, nil
}

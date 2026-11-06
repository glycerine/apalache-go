package qe

import "github.com/glycerine/g3/pkg/ast"

func (c *arithmeticModelProjectCall) integerBranch(entry *arithmeticPluginBounds, target ast.ASTHandle) (uint32, error) {
	policy, err := c.plugin.branchPolicy(entry, target)
	if err != nil {
		return 0, err
	}
	direction := ArithmeticUpperBound
	if policy.Lower {
		direction = ArithmeticLowerBound
	}
	_, index, found, err := c.findMinMax(entry.processor.Bounds(false, direction), policy.Lower)
	if err != nil || !found {
		return 0, err
	}
	return 1 + index, nil
}

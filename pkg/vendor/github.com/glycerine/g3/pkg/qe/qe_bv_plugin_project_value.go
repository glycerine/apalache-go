package qe

// Source: src/qe/qe_bv_plugin.cpp project evaluation and numeral extraction.

import "github.com/glycerine/g3/pkg/ast"

func (c *bitVectorModelProjectCall) evaluate(target ast.ASTHandle) error {
	if err := c.evaluator.Evaluate(target, c.value); err != nil {
		return err
	}
	evaluated, err := c.plugin.manager.RefGet(c.value)
	if err != nil {
		return err
	}
	// Source starts val at zero and ignores is_numeral's Boolean result.
	// The evaluator uses model_completion=false, so an uninterpreted value
	// follows that zero branch without changing the model.
	_, err = c.plugin.manager.BitVectorNumeralValue(evaluated, &c.numeral, &c.width)
	return err
}

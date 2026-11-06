package pdr

// Source: context::init_core_generalizers arithmetic classification in
// src/muz/pdr/pdr_context.cpp.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

type contextArithmeticPlan struct {
	enabled    bool
	selectMode bool
	mode       uint32
	eq2ineq    bool
}

func (c *Context) planArithmetic(forms []ast.ASTHandle, nonBoolean, hasArithmetic bool) (contextArithmeticPlan, error) {
	plan := contextArithmeticPlan{
		enabled: nonBoolean,
		mode:    smtcore.SMTArithmeticSimplex,
	}
	if !nonBoolean {
		return plan, nil
	}
	plan.selectMode = c.useUTVPI && !c.useConvexClosure && !c.useConvexInterior
	if !plan.selectMode || !hasArithmetic {
		return plan, nil
	}
	difference, err := IsDifferenceLogic(c.manager, forms)
	if err != nil {
		return contextArithmeticPlan{}, err
	}
	if difference {
		plan.mode = smtcore.SMTArithmeticDifferenceLogic
		plan.eq2ineq = true
		return plan, nil
	}
	utvpi, err := IsUTVPILogic(c.manager, forms)
	if err != nil {
		return contextArithmeticPlan{}, err
	}
	if utvpi {
		plan.mode = smtcore.SMTArithmeticUTVPI
		plan.eq2ineq = true
	}
	return plan, nil
}

func (c *Context) arithmeticPlanCompatible(plan contextArithmeticPlan) bool {
	current := &c.owner.contexts.arithmetic
	return current.enabled == plan.enabled &&
		(!plan.enabled || current.selectMode == plan.selectMode &&
			current.mode == plan.mode && current.eq2ineq == plan.eq2ineq)
}

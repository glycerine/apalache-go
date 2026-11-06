package cmd_context

// Source: src/opt/opt_context.cpp update_solver, probe_bv and is_bv.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
)

func (c *Context) optimizationSATCoreEligible(assumptions []ast.ASTHandle) (eligible bool, resultErr error) {
	if c.manager == nil || len(c.objectives) == 0 {
		return false, nil
	}
	proofs, err := c.manager.ProofsEnabled()
	if err != nil || proofs {
		return false, err
	}
	priority, err := c.optimizationPriority()
	if err != nil || priority == "pareto" {
		return false, err
	}
	parameters, err := opt.NewParams(c.config.Params, c.config.Globals, &c.params)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, parameters.Close()) }()
	enabled, err := parameters.EnableSAT()
	if err != nil || !enabled {
		return false, err
	}
	engine, err := parameters.MaxSATEngine()
	if err != nil {
		return false, err
	}
	engineName, err := c.manager.Symbols.String(engine)
	if err != nil {
		return false, err
	}
	switch engineName {
	case "maxres", "pd-maxres", "bcd2", "sls":
	default:
		return false, nil
	}
	for i := range c.objectives {
		objective := &c.objectives[i]
		if !objective.soft && !objective.bitVector {
			return false, nil
		}
	}
	booleanFamily := c.manager.BasicFamilyID()
	pseudoBooleanFamily, err := c.manager.RegisterPseudoBooleanPlugin()
	if err != nil {
		return false, err
	}
	bitVectorFamily, err := c.manager.RegisterBitVectorPlugin()
	if err != nil {
		return false, err
	}
	for _, assertion := range c.assertions {
		ok, checkErr := c.optimizationSATFormula(assertion.expression, booleanFamily, pseudoBooleanFamily, bitVectorFamily)
		if checkErr != nil || !ok {
			return false, checkErr
		}
	}
	for _, assumption := range assumptions {
		ok, checkErr := c.optimizationSATFormula(assumption, booleanFamily, pseudoBooleanFamily, bitVectorFamily)
		if checkErr != nil || !ok {
			return false, checkErr
		}
	}
	for i := range c.objectives {
		objective := &c.objectives[i]
		if objective.bitVector {
			ok, checkErr := c.optimizationSATFormula(objective.expression, booleanFamily, pseudoBooleanFamily, bitVectorFamily)
			if checkErr != nil || !ok {
				return false, checkErr
			}
		}
		for j := range objective.softTerms {
			ok, checkErr := c.optimizationSATFormula(objective.softTerms[j].expression, booleanFamily, pseudoBooleanFamily, bitVectorFamily)
			if checkErr != nil || !ok {
				return false, checkErr
			}
		}
	}
	return true, nil
}

func (c *Context) optimizationSATFormula(root ast.ASTHandle, booleanFamily, pseudoBooleanFamily, bitVectorFamily ast.FamilyID) (bool, error) {
	stack := []ast.ASTHandle{root}
	visited := make(map[ast.ASTHandle]struct{})
	for len(stack) != 0 {
		h := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, found := visited[h]; found {
			continue
		}
		visited[h] = struct{}{}
		kind, err := c.manager.GetKind(h)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp {
			return false, nil
		}
		family, err := c.manager.GetFamilyID(h)
		if err != nil {
			return false, err
		}
		count, err := c.manager.GetNumArgs(h)
		if err != nil {
			return false, err
		}
		if family != booleanFamily && family != pseudoBooleanFamily && family != bitVectorFamily && !(family == ast.NullFamilyID && count == 0) {
			return false, nil
		}
		for i := uint32(0); i < count; i++ {
			arg, getErr := c.manager.GetArg(h, i)
			if getErr != nil {
				return false, getErr
			}
			stack = append(stack, arg)
		}
	}
	return true, nil
}

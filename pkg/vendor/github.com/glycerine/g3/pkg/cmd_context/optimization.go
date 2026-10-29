package cmd_context

// Source: src/opt/opt_context.cpp scoped objectives and optimize;
// src/opt/opt_cmds.cpp maximize/minimize command integration.

import (
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func releaseOptimizationBoxModels(models []*model.ModelCore) (result error) {
	for i := len(models) - 1; i >= 0; i-- {
		if models[i] != nil {
			consumed, err := models[i].DecRefConsumed()
			if consumed {
				models[i] = nil
			}
			result = errors.Join(result, err)
		}
	}
	return result
}

func (c *Context) clearOptimizationBoxModels() error {
	err := releaseOptimizationBoxModels(c.optimizationBoxModels)
	if err == nil {
		c.optimizationBoxModels = nil
	}
	return err
}

func (c *Context) AddObjective(expression ast.ASTHandle, maximize bool) error {
	m, err := c.Manager()
	if err != nil {
		return err
	}
	kind, err := m.GetKind(expression)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp {
		return fmt.Errorf("malformed objective term: it cannot be a quantifier or bound variable")
	}
	sort, err := m.GetSort(expression)
	if err != nil {
		return err
	}
	family, err := m.GetFamilyID(sort)
	if err != nil {
		return err
	}
	arithmetic, err := m.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	bitVector, err := m.RegisterBitVectorPlugin()
	if err != nil {
		return err
	}
	isBitVector := family == bitVector
	if family != arithmetic && !isBitVector {
		return fmt.Errorf("objective must be bit-vector, integer or real")
	}
	if err = c.logicChecker.CheckExpression(expression); err != nil {
		return err
	}
	c.checked = false
	if err = c.clearCheckSatResult(); err != nil {
		return err
	}
	ref, err := m.NewRef(expression)
	if err != nil {
		return err
	}
	objective := optimizationObjective{expression: expression, ref: ref, maximize: maximize, bitVector: isBitVector}
	if isBitVector {
		objective.bitWidth, err = m.BitVectorSize(sort)
		if err != nil {
			return errors.Join(err, m.DelRef(&ref))
		}
	}
	c.objectives = append(c.objectives, objective)
	return nil
}

// AddSoftConstraint groups constraints by identifier at the position of the
// group's first occurrence. Terms appended inside a scope are trailed
// separately because the group itself may predate that scope.
func (c *Context) AddSoftConstraint(expression ast.ASTHandle, weight *big.Rat, id string) error {
	if weight == nil {
		return ast.ErrASTHandle
	}
	m, err := c.Manager()
	if err != nil {
		return err
	}
	sort, err := m.GetSort(expression)
	if err != nil {
		return err
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return err
	}
	if sort != boolean {
		return fmt.Errorf("invalid type for expression: expected Boolean type")
	}
	if err = c.logicChecker.CheckExpression(expression); err != nil {
		return err
	}
	c.checked = false
	if err = c.clearCheckSatResult(); err != nil {
		return err
	}
	index, found := c.softObjectiveIndex[id]
	if !found {
		index = len(c.objectives)
		c.objectives = append(c.objectives, optimizationObjective{soft: true, id: id})
		c.softObjectiveIndex[id] = index
	}
	if weight.Sign() == 0 {
		return nil
	}
	ref, err := m.NewRef(expression)
	if err != nil {
		return err
	}
	term := optimizationSoftTerm{expression: expression, ref: ref}
	term.weight.Set(weight)
	c.objectives[index].softTerms = append(c.objectives[index].softTerms, term)
	c.softObjectiveTrail = append(c.softObjectiveTrail, index)
	return nil
}

func (c *Context) restoreObjectives(size, softTerms int) error {
	for len(c.softObjectiveTrail) > softTerms {
		last := len(c.softObjectiveTrail) - 1
		index := c.softObjectiveTrail[last]
		if index < 0 || index >= len(c.objectives) || !c.objectives[index].soft || len(c.objectives[index].softTerms) == 0 {
			return ast.ErrASTDomain
		}
		objective := &c.objectives[index]
		termIndex := len(objective.softTerms) - 1
		if err := c.manager.DelRef(&objective.softTerms[termIndex].ref); err != nil {
			return err
		}
		objective.softTerms = objective.softTerms[:termIndex]
		c.softObjectiveTrail = c.softObjectiveTrail[:last]
	}
	for len(c.objectives) > size {
		last := len(c.objectives) - 1
		objective := &c.objectives[last]
		if objective.soft {
			delete(c.softObjectiveIndex, objective.id)
			if len(objective.softTerms) != 0 {
				return ast.ErrASTDomain
			}
		} else if err := c.manager.DelRef(&objective.ref); err != nil {
			return err
		}
		c.objectives = c.objectives[:last]
	}
	return nil
}

func (c *Context) addBitVectorObjective(optimizer *opt.ArithmeticSolver, objective *optimizationObjective) (resultErr error) {
	if optimizer == nil || objective == nil || !objective.bitVector {
		return ast.ErrASTHandle
	}
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&refs[i]))
		}
	}()
	bit := uint64(0)
	if objective.maximize {
		bit = 1
	}
	value, err := c.manager.BitVectorNumeralUint64(bit, 1)
	if err != nil {
		return err
	}
	valueRef, err := c.manager.NewRef(value)
	if err != nil {
		return err
	}
	refs = append(refs, valueRef)
	terms := make([]ast.ASTHandle, objective.bitWidth)
	weights := make([]*big.Rat, objective.bitWidth)
	for i := uint32(0); i < objective.bitWidth; i++ {
		extracted, e := c.manager.MkBitVectorExtract(i, i, objective.expression)
		if e != nil {
			return e
		}
		extractedRef, e := c.manager.NewRef(extracted)
		if e != nil {
			return e
		}
		refs = append(refs, extractedRef)
		value, e = c.manager.RefGet(valueRef)
		if e != nil {
			return e
		}
		extracted, e = c.manager.RefGet(extractedRef)
		if e != nil {
			return e
		}
		term, e := c.manager.MkEq(value, extracted)
		if e != nil {
			return e
		}
		termRef, e := c.manager.NewRef(term)
		if e != nil {
			return e
		}
		refs = append(refs, termRef)
		terms[i] = term
		weights[i] = new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), uint(i)))
	}
	return optimizer.AddSoftObjective(terms, weights)
}

func (c *Context) optimizationPriority() (priority string, resultErr error) {
	module, err := c.config.Globals.SourceGetModuleName("opt")
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, c.config.Params.Del(&module)) }()
	key := "priority"
	value, err := c.config.Params.GetSymbolName(&module, &key, usymbol.Symbol{})
	if err != nil {
		return "", err
	}
	priority, _, err = c.config.Params.SymbolsField.BareString(value)
	return priority, err
}

func (c *Context) newOptimizer(assumptions []ast.ASTHandle) (optimizer *opt.ArithmeticSolver, resultErr error) {
	optimizer, err := opt.NewArithmeticSolver(c.manager, &c.params, c.logic, c.config)
	if err != nil {
		return nil, err
	}
	published := false
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, optimizer.Close())
		}
	}()
	for _, assertion := range c.assertions {
		if err = optimizer.Assert(assertion.expression); err != nil {
			return nil, err
		}
	}
	for _, assumption := range assumptions {
		if err = optimizer.Assert(assumption); err != nil {
			return nil, err
		}
	}
	published = true
	return optimizer, nil
}

func (c *Context) addOptimizerObjective(optimizer *opt.ArithmeticSolver, index int) error {
	if optimizer == nil || index < 0 || index >= len(c.objectives) {
		return ast.ErrASTHandle
	}
	objective := &c.objectives[index]
	if objective.soft {
		terms := make([]ast.ASTHandle, len(objective.softTerms))
		weights := make([]*big.Rat, len(objective.softTerms))
		for i := range objective.softTerms {
			terms[i] = objective.softTerms[i].expression
			weights[i] = &objective.softTerms[i].weight
		}
		return optimizer.AddSoftObjective(terms, weights)
	}
	if objective.bitVector {
		return c.addBitVectorObjective(optimizer, objective)
	}
	return optimizer.AddObjective(objective.expression, objective.maximize)
}

func (c *Context) checkOptimizeLex(assumptions []ast.ASTHandle) (optimized *opt.ArithmeticResult, resultErr error) {
	optimizer, err := c.newOptimizer(assumptions)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, optimizer.Close()) }()
	for i := range c.objectives {
		if err = c.addOptimizerObjective(optimizer, i); err != nil {
			return nil, err
		}
	}
	return optimizer.Check()
}

func (c *Context) checkOptimizeBox(assumptions []ast.ASTHandle, continuation bool) (optimized *opt.ArithmeticResult, resultErr error) {
	fresh := !continuation || !c.optimizationBoxActive
	if fresh {
		if err := c.clearOptimizationBoxModels(); err != nil {
			return nil, err
		}
		c.optimizationBoxNext = 0
	}
	c.optimizationBoxActive = true
	if c.optimizationBoxNext >= len(c.objectives) {
		c.optimizationBoxActive = false
		return &opt.ArithmeticResult{Status: lbool.False, Values: make([]opt.ObjectiveValue, len(c.objectives)), Optimal: true}, nil
	}
	selected := c.optimizationBoxNext
	c.optimizationBoxNext++
	combined := &opt.ArithmeticResult{Status: lbool.True, Values: make([]opt.ObjectiveValue, len(c.objectives)), Optimal: true}
	var boxModels []*model.ModelCore
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, combined.Close())
		}
		resultErr = errors.Join(resultErr, releaseOptimizationBoxModels(boxModels))
	}()
	for i := range c.objectives {
		optimizer, err := c.newOptimizer(assumptions)
		if err != nil {
			return nil, err
		}
		if err = c.addOptimizerObjective(optimizer, i); err != nil {
			return nil, errors.Join(err, optimizer.Close())
		}
		one, err := optimizer.Check()
		err = errors.Join(err, optimizer.Close())
		if err != nil {
			if one != nil {
				err = errors.Join(err, one.Close())
			}
			return nil, err
		}
		if one.Status != lbool.True || !one.Optimal || len(one.Values) != 1 {
			combined.Status, combined.Optimal = one.Status, one.Optimal
			return combined, one.Close()
		}
		if fresh {
			if one.Model != nil {
				if err = one.Model.IncRef(); err != nil {
					return nil, errors.Join(err, one.Close())
				}
			}
			boxModels = append(boxModels, one.Model)
		}
		combined.Values[i] = one.Values[0]
		if i == selected {
			combined.Model = one.Model
			one.Model = nil
		}
		if err = one.Close(); err != nil {
			return nil, err
		}
	}
	if fresh {
		c.optimizationBoxModels = boxModels
		boxModels = nil
	}
	return combined, nil
}

func (c *Context) checkOptimize(assumptions []ast.ASTHandle, continuation bool) (status lbool.LBool, resultErr error) {
	priority, err := c.optimizationPriority()
	if err != nil {
		return lbool.Undef, err
	}
	var optimized *opt.ArithmeticResult
	if priority != "pareto" {
		if err = c.closeParetoBackend(); err != nil {
			return lbool.Undef, err
		}
	}
	if priority == "box" {
		optimized, err = c.checkOptimizeBox(assumptions, continuation)
	} else if priority == "pareto" {
		if err = c.clearOptimizationBoxModels(); err != nil {
			return lbool.Undef, err
		}
		c.optimizationBoxActive = false
		optimized, err = c.checkOptimizePareto(assumptions, continuation)
	} else {
		if err = c.clearOptimizationBoxModels(); err != nil {
			return lbool.Undef, err
		}
		c.optimizationBoxActive = false
		optimized, err = c.checkOptimizeLex(assumptions)
	}
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, optimized.Close()) }()
	result, err := solver.NewSimpleCheckSatResult(c.manager)
	if err != nil {
		return lbool.Undef, err
	}
	published := false
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	if err = result.SetStatus(optimized.Status); err != nil {
		return lbool.Undef, err
	}
	if optimized.Status == lbool.Undef {
		if err = result.SetReasonUnknown("optimization returned unknown"); err != nil {
			return lbool.Undef, err
		}
	}
	if err = result.SetModel(optimized.Model); err != nil {
		return lbool.Undef, err
	}
	if optimized.Status == lbool.True && len(c.objectives) == len(optimized.Values) {
		for i := range c.objectives {
			if c.objectives[i].soft {
				c.objectives[i].softValue.Set(&optimized.Values[i].SoftCost)
			} else if c.objectives[i].bitVector {
				c.objectives[i].softValue.Set(&optimized.Values[i].SoftCost)
				if c.objectives[i].maximize {
					maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(c.objectives[i].bitWidth)), big.NewInt(1))
					c.objectives[i].softValue.Sub(new(big.Rat).SetInt(maximum), &c.objectives[i].softValue)
				}
			} else {
				c.objectives[i].value = optimized.Values[i].Arithmetic
				c.objectives[i].modelValue = optimized.Values[i].ArithmeticModel
				c.objectives[i].openBounds = optimized.Values[i].ArithmeticOpenBounds
				c.objectives[i].interval = optimized.Values[i].ArithmeticInterval && optimized.Values[i].ArithmeticModelValid &&
					!sameArithmeticObjectiveValue(c.objectives[i].value, c.objectives[i].modelValue)
			}
			c.objectives[i].valueReady = optimized.Optimal
		}
	}
	c.tacticResult = result
	c.status, c.checked = optimized.Status, true
	published = true
	return optimized.Status, nil
}

func (c *Context) WriteObjectives(out io.Writer) error {
	if !c.checked {
		return fmt.Errorf("check-sat has not been called with latest objectives")
	}
	if _, err := io.WriteString(out, "(objectives\n"); err != nil {
		return err
	}
	for _, objective := range c.objectives {
		if !objective.valueReady {
			return fmt.Errorf("objective value is not available")
		}
		if _, err := io.WriteString(out, " ("); err != nil {
			return err
		}
		if objective.soft {
			if _, err := io.WriteString(out, objective.id); err != nil {
				return err
			}
		} else {
			if err := c.WriteExpression(out, objective.expression); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(out, " "); err != nil {
			return err
		}
		if objective.soft || objective.bitVector {
			if err := c.writeSoftObjectiveValue(out, &objective.softValue); err != nil {
				return err
			}
		} else {
			if err := c.writeObjectiveValue(out, &objective); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(out, ")\n"); err != nil {
			return err
		}
	}
	_, err := io.WriteString(out, ")\n")
	return err
}

func (c *Context) writeSoftObjectiveValue(out io.Writer, value *big.Rat) (resultErr error) {
	service := c.manager.NumeralService()
	if service == nil || value == nil {
		return ast.ErrASTHandle
	}
	numeral, err := service.NewValue()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, service.Del(&numeral)) }()
	if err = service.SetString(&numeral, value.RatString()); err != nil {
		return err
	}
	expression, err := c.manager.ArithmeticNumeral(&numeral, value.IsInt())
	if err != nil {
		return err
	}
	ref, err := c.manager.NewRef(expression)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&ref)) }()
	expression, err = c.manager.RefGet(ref)
	if err != nil {
		return err
	}
	return c.WriteExpression(out, expression)
}

func (c *Context) writeObjectiveValue(out io.Writer, objective *optimizationObjective) error {
	if objective.openBounds {
		_, err := io.WriteString(out, " (interval (* (- 1) oo) oo)")
		return err
	}
	if objective.interval {
		if _, err := io.WriteString(out, " (interval "); err != nil {
			return err
		}
		lower, upper := objective.modelValue, objective.value
		if !objective.maximize {
			lower, upper = upper, lower
		}
		if err := c.writeArithmeticObjectiveValue(out, objective.expression, lower, objective.maximize); err != nil {
			return err
		}
		if _, err := io.WriteString(out, " "); err != nil {
			return err
		}
		if err := c.writeArithmeticObjectiveValue(out, objective.expression, upper, objective.maximize); err != nil {
			return err
		}
		_, err := io.WriteString(out, ")")
		return err
	}
	return c.writeArithmeticObjectiveValue(out, objective.expression, objective.value, objective.maximize)
}

func sameArithmeticObjectiveValue(a, b smtcore.SMTArithmeticObjectiveValue) bool {
	return a.Unbounded == b.Unbounded && (a.Unbounded || a.Finite.Cmp(&b.Finite) == 0)
}

func (c *Context) writeArithmeticObjectiveValue(out io.Writer, expression ast.ASTHandle, objective smtcore.SMTArithmeticObjectiveValue, maximize bool) error {
	if objective.Unbounded {
		if maximize {
			_, err := io.WriteString(out, "oo")
			return err
		}
		_, err := io.WriteString(out, "(* (- 1) oo)")
		return err
	}
	value := &objective.Finite
	sort, err := c.manager.GetSort(expression)
	if err != nil {
		return err
	}
	kind, err := c.manager.GetDeclKind(sort)
	if err != nil {
		return err
	}
	ref, err := opt.ArithmeticValueExpression(c.manager, value, kind == 1)
	if err != nil {
		return err
	}
	h, err := c.manager.RefGet(ref)
	if err == nil {
		err = c.WriteExpression(out, h)
	}
	return errors.Join(err, c.manager.DelRef(&ref))
}

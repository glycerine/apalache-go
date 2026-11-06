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
		c.optimizationBoxValues = nil
		c.optimizationBoxLabels = nil
		c.optimizationBoxReasonUnknown = ""
	}
	return err
}

func cloneOptimizationBoxValues(source []opt.ObjectiveValue) []opt.ObjectiveValue {
	result := make([]opt.ObjectiveValue, len(source))
	for i := range source {
		result[i] = source[i]
		result[i].Arithmetic.Finite = source[i].Arithmetic.Finite.Clone()
		result[i].ArithmeticModel.Finite = source[i].ArithmeticModel.Finite.Clone()
		result[i].SoftCost.Set(&source[i].SoftCost)
		result[i].SoftSatisfied.Set(&source[i].SoftSatisfied)
		result[i].SoftLower.Set(&source[i].SoftLower)
		result[i].SoftUpper.Set(&source[i].SoftUpper)
		result[i].SourceLower.Set(&source[i].SourceLower)
		result[i].SourceUpper.Set(&source[i].SourceUpper)
		result[i].SoftAssignment = append([]bool(nil), source[i].SoftAssignment...)
	}
	return result
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
	if err = c.invalidateOptimizationState(); err != nil {
		return err
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
	if err = c.invalidateOptimizationState(); err != nil {
		return err
	}
	if err = c.logicChecker.CheckExpression(expression); err != nil {
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

// Source: opt::context::clear_state, called before adding an objective or a
// soft constraint. Drop the active Pareto solver and published check result
// before validating/publishing the changed objective set.
func (c *Context) invalidateOptimizationState() error {
	c.checked = false
	c.optimizationBoxActive = false
	c.optimizationBoxNext = 0
	if err := c.closeParetoBackend(); err != nil {
		return err
	}
	return c.clearCheckSatResult()
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
	key := "priority"
	local, err := c.config.Params.ContainsName(&c.params, &key)
	if err != nil {
		return "", err
	}
	if local {
		value, err := c.config.Params.GetSymbolName(&c.params, &key, usymbol.Symbol{})
		if err != nil {
			return "", err
		}
		priority, _, err := c.config.Params.SymbolsField.BareString(value)
		return priority, err
	}
	module, err := c.config.Globals.SourceGetModuleName("opt")
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, c.config.Params.Del(&module)) }()
	value, err := c.config.Params.GetSymbolName(&module, &key, usymbol.Symbol{})
	if err != nil {
		return "", err
	}
	priority, _, err = c.config.Params.SymbolsField.BareString(value)
	return priority, err
}

func (c *Context) setupOptimizationArithmeticSolver() (resultErr error) {
	parameters, err := opt.NewParams(c.config.Params, c.config.Globals, &c.params)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, parameters.Close()) }()
	engine, err := parameters.OptSMTEngine()
	if err != nil {
		return err
	}
	name, _, err := c.config.Params.SymbolsField.BareString(engine)
	if err != nil {
		return err
	}
	if name == "symba" || name == "farkas" {
		return c.config.Globals.SourceSetName("smt.arith.solver", "5")
	}
	return nil
}

func (c *Context) newOptimizer(assumptions []ast.ASTHandle) (optimizer *opt.ArithmeticSolver, resultErr error) {
	solverParams, err := c.config.Params.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, c.config.Params.Del(&solverParams)) }()
	if err = c.config.Params.Copy(&solverParams, &c.params); err != nil {
		return nil, err
	}
	parameters, err := opt.NewParams(c.config.Params, c.config.Globals, &solverParams)
	if err != nil {
		return nil, err
	}
	engine, err := parameters.OptSMTEngine()
	if err == nil {
		var name string
		name, err = c.manager.Symbols.String(engine)
		if err == nil && (name == "symba" || name == "farkas") {
			mode := "arith.solver"
			err = c.config.Params.SetUIntName(&solverParams, &mode, 5)
		}
	}
	err = errors.Join(err, parameters.Close())
	if err != nil {
		return nil, err
	}
	optimizer, err = opt.NewArithmeticSolver(c.manager, &solverParams, c.logic, c.config)
	if err != nil {
		return nil, err
	}
	satCoreEnabled, err := c.optimizationSATCoreEligible(assumptions)
	if err != nil {
		return nil, errors.Join(err, optimizer.Close())
	}
	if err = optimizer.SetSATCoreEnabled(satCoreEnabled); err != nil {
		return nil, errors.Join(err, optimizer.Close())
	}
	published := false
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, optimizer.Close())
		}
	}()
	if c.optimizationPreprocessor != nil {
		if err = optimizer.SetPreprocessingConverter(c.optimizationPreprocessor); err != nil {
			return nil, err
		}
	}
	if c.optimizationHardOverrideActive {
		for _, formula := range c.optimizationHardOverride {
			if err = optimizer.Assert(formula); err != nil {
				return nil, err
			}
		}
	} else {
		for _, assertion := range c.assertions {
			if err = optimizer.Assert(assertion.expression); err != nil {
				return nil, err
			}
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
		return optimizer.AddSoftObjectiveWithOffset(terms, weights, &objective.softOffset)
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
	if !fresh {
		if len(c.optimizationBoxModels) == 0 {
			if c.optimizationBoxNext < len(c.objectives) {
				c.optimizationBoxNext++
				return &opt.ArithmeticResult{
					Status:        lbool.Undef,
					ReasonUnknown: c.optimizationBoxReasonUnknown,
					Labels:        append([]usymbol.Symbol(nil), c.optimizationBoxLabels...),
					Values:        make([]opt.ObjectiveValue, len(c.objectives)),
				}, nil
			}
			c.optimizationBoxActive = false
			c.optimizationBoxReasonUnknown = ""
			return &opt.ArithmeticResult{Status: lbool.False, Values: make([]opt.ObjectiveValue, len(c.objectives)), Optimal: true}, nil
		}
		if c.optimizationBoxNext >= len(c.optimizationBoxModels) {
			c.optimizationBoxActive = false
			c.optimizationBoxReasonUnknown = ""
			return &opt.ArithmeticResult{Status: lbool.False, Values: make([]opt.ObjectiveValue, len(c.objectives)), Optimal: true}, nil
		}
		selected := c.optimizationBoxNext
		c.optimizationBoxNext++
		optimized = &opt.ArithmeticResult{Status: lbool.True, Values: cloneOptimizationBoxValues(c.optimizationBoxValues), Labels: append([]usymbol.Symbol(nil), c.optimizationBoxLabels...), Optimal: true}
		if selected < len(c.optimizationBoxModels) && c.optimizationBoxModels[selected] != nil {
			if err := c.optimizationBoxModels[selected].IncRef(); err != nil {
				return nil, err
			}
			optimized.Model = c.optimizationBoxModels[selected]
		}
		return optimized, nil
	}
	if c.optimizationBoxNext >= len(c.objectives) {
		c.optimizationBoxActive = false
		return &opt.ArithmeticResult{Status: lbool.False, Values: make([]opt.ObjectiveValue, len(c.objectives)), Optimal: true}, nil
	}
	selected := c.optimizationBoxNext
	c.optimizationBoxNext++
	allArithmetic := true
	for i := range c.objectives {
		if c.objectives[i].soft {
			allArithmetic = false
			break
		}
	}
	if allArithmetic {
		optimizer, err := c.newOptimizer(assumptions)
		if err != nil {
			return nil, err
		}
		for i := range c.objectives {
			if err = c.addOptimizerObjective(optimizer, i); err != nil {
				return nil, errors.Join(err, optimizer.Close())
			}
		}
		combined, err := optimizer.CheckBox()
		err = errors.Join(err, optimizer.Close())
		if err != nil {
			if combined != nil {
				err = errors.Join(err, combined.Close())
			}
			return nil, err
		}
		if combined.Status != lbool.True || !combined.Optimal || len(combined.ObjectiveModels) != len(c.objectives) {
			if combined.Status == lbool.Undef {
				c.optimizationBoxReasonUnknown = combined.ReasonUnknown
			}
			return combined, nil
		}
		c.optimizationBoxLabels = append(c.optimizationBoxLabels[:0], combined.InitialLabels...)
		c.optimizationBoxModels = combined.ObjectiveModels
		combined.ObjectiveModels = nil
		c.optimizationBoxValues = cloneOptimizationBoxValues(combined.Values)
		if selected >= len(c.optimizationBoxModels) || c.optimizationBoxModels[selected] == nil {
			return nil, errors.Join(ast.ErrASTHandle, c.clearOptimizationBoxModels(), combined.Close())
		}
		if err = c.optimizationBoxModels[selected].IncRef(); err != nil {
			return nil, errors.Join(err, c.clearOptimizationBoxModels(), combined.Close())
		}
		if combined.Model != nil {
			initialModel := combined.Model
			combined.Model = nil
			if err = initialModel.DecRef(); err != nil {
				_ = c.optimizationBoxModels[selected].DecRef()
				return nil, errors.Join(err, c.clearOptimizationBoxModels(), combined.Close())
			}
		}
		combined.Model = c.optimizationBoxModels[selected]
		c.optimizationBoxActive = true
		return combined, nil
	}
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
		// context::optimize captures labels on its initial satisfiable check.
		// execute_box changes them only when a MaxSMT group supplies a model;
		// selecting another cached arithmetic box model preserves those labels.
		if i == 0 {
			combined.Labels = append([]usymbol.Symbol(nil), one.InitialLabels...)
		}
		if c.objectives[i].soft && one.Model != nil {
			combined.Labels = append(combined.Labels[:0], one.Labels...)
		}
		c.optimizationBoxLabels = append(c.optimizationBoxLabels[:0], combined.Labels...)
		if one.Status != lbool.True || !one.Optimal || len(one.Values) != 1 {
			combined.Status, combined.Optimal = one.Status, one.Optimal
			combined.ReasonUnknown = one.ReasonUnknown
			if one.Status == lbool.Undef {
				c.optimizationBoxReasonUnknown = one.ReasonUnknown
			}
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
		c.optimizationBoxValues = cloneOptimizationBoxValues(combined.Values)
	}
	return combined, nil
}

func (c *Context) checkOptimize(assumptions []ast.ASTHandle, continuation bool) (status lbool.LBool, resultErr error) {
	c.optimizationException = ""
	execution, err := c.beginOptimizationExecution()
	if err != nil {
		return c.handleOptimizationFailure(err)
	}
	defer func() { resultErr = errors.Join(resultErr, execution.Close()) }()
	priority, err := c.optimizationPriority()
	if err != nil {
		return lbool.Undef, err
	}
	var optimized *opt.ArithmeticResult
	paretoContinuation := c.optimizationParetoActive && c.optimizationParetoBackend != nil
	boxContinuation := c.optimizationBoxActive
	if !paretoContinuation && priority != "pareto" {
		if err = c.closeParetoBackend(); err != nil {
			return lbool.Undef, err
		}
	}
	if !paretoContinuation && !boxContinuation {
		if err = c.setupOptimizationArithmeticSolver(); err != nil {
			return lbool.Undef, err
		}
	}
	optimized, err = c.runNormalizedOptimization(assumptions, continuation, priority, paretoContinuation, boxContinuation)
	if err != nil {
		return c.handleOptimizationFailure(err)
	}
	defer func() { resultErr = errors.Join(resultErr, optimized.Close()) }()
	completed := optimized.Status == lbool.True
	// Source optimize performs adjust_unknown only after the initial execution;
	// an existing Pareto engine returns execute_pareto directly. yield updates
	// objective bounds before that initial status adjustment.
	if completed && !paretoContinuation && c.optimizationParetoBackend != nil {
		wasUnknown, e := c.optimizationParetoBackend.OptimizationWasUnknown()
		if e != nil {
			return lbool.Undef, e
		}
		if wasUnknown {
			optimized.Status = lbool.Undef
			optimized.ReasonUnknown, err = c.optimizationParetoBackend.ReasonUnknown()
			if err != nil {
				return lbool.Undef, err
			}
		}
	}
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
		reason, reasonErr := c.optimizationResultUnknownReason(optimized)
		if reasonErr != nil {
			return lbool.Undef, reasonErr
		}
		if err = result.SetReasonUnknown(reason); err != nil {
			return lbool.Undef, err
		}
	}
	if err = result.SetModel(optimized.Model); err != nil {
		return lbool.Undef, err
	}
	if err = result.SetLabels(optimized.Labels); err != nil {
		return lbool.Undef, err
	}
	c.publishOptimizationBounds(optimized)
	if completed && len(c.objectives) == len(optimized.Values) {
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

func (c *Context) optimizationUnknownReason() (string, error) {
	canceled, err := c.manager.Canceled()
	if err != nil {
		return "", err
	}
	if canceled {
		return c.manager.ResourceLimit().CancelMessage(), nil
	}
	return "unknown", nil
}

func (c *Context) optimizationResultUnknownReason(result *opt.ArithmeticResult) (string, error) {
	canceled, err := c.manager.Canceled()
	if err != nil {
		return "", err
	}
	if canceled {
		return c.manager.ResourceLimit().CancelMessage(), nil
	}
	if result != nil && result.ReasonUnknown != "" {
		return result.ReasonUnknown, nil
	}
	return "unknown", nil
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

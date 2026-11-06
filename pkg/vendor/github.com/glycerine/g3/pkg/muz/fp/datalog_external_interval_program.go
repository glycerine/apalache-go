package fp

// Source: src/muz/rel/dl_compiler.cpp compile_nonrecursive_stratum and
// do_compilation: one LOAD bank, dependency-first rules, then termination STORE.

import (
	"errors"
	"fmt"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type externalIntervalDatalogProgramRule struct {
	identity  *externalIntervalDatalogIdentityRule
	ground    *externalIntervalDatalogGroundRule
	cartesian *externalIntervalDatalogCartesianRule
	stratum   uint32
}

type externalIntervalDatalogProgramAction struct {
	rule      int
	inputTail int
}

type externalIntervalDatalogProgramStratum struct {
	heads   []ast.ASTHandle
	seeds   []int
	loop    []externalIntervalDatalogProgramAction
	allLoop []externalIntervalDatalogProgramAction
	cyclic  bool
}

func (r externalIntervalDatalogProgramRule) declarations() []ast.ASTHandle {
	if r.identity != nil {
		return []ast.ASTHandle{r.identity.head, r.identity.tail}
	}
	if r.ground != nil {
		return []ast.ASTHandle{r.ground.head}
	}
	return []ast.ASTHandle{r.cartesian.head, r.cartesian.left, r.cartesian.right}
}

type externalIntervalDatalogProgram struct {
	rules        []externalIntervalDatalogProgramRule
	declarations []ast.ASTHandle
	schedule     []int
	strata       []externalIntervalDatalogProgramStratum
	registers    *externalIntervalDatalogRegisters
	next         int
	stratum      int
	phase        uint8
	current      map[ast.ASTHandle]*rel.ExternalIntervalProduct
	nextDelta    map[ast.ASTHandle]*rel.ExternalIntervalProduct
	actionNext   int
	widen        bool
	allOrNothing bool
	storing      bool
}

// Validate every rule before any callback. Constants are constructed lazily
// in scheduled rule/column order and shared across the execution register bank.
func (e *PropositionalEngine) externalIntervalProgramPlan() (*externalIntervalDatalogProgram, error) {
	return e.externalIntervalCompiler().externalIntervalProgramPlan()
}

func (c *externalIntervalDatalogCompiler) externalIntervalProgramPlan() (*externalIntervalDatalogProgram, error) {
	count, err := c.set.Len()
	if err != nil || count == 0 {
		return nil, err
	}
	if count == 1 {
		rule, err := c.set.Rule(0)
		if err != nil {
			return nil, err
		}
		head, err := rule.HeadDeclaration()
		recursive := false
		if err == nil {
			tails, cause := rule.UninterpretedTailSize()
			err = cause
			for index := 0; err == nil && index < tails; index++ {
				var tail ast.ASTHandle
				tail, err = rule.TailDeclaration(index)
				recursive = recursive || tail == head
			}
		}
		err = errors.Join(err, rule.Close())
		if err != nil || !recursive {
			return nil, err
		}
	}
	program := &externalIntervalDatalogProgram{}
	for index := 0; index < count; index++ {
		rule, cause := c.set.Rule(index)
		if cause != nil {
			return nil, cause
		}
		plan := externalIntervalDatalogProgramRule{}
		positive, cause := rule.PositiveTailSize()
		if cause == nil {
			switch {
			case positive == 0:
				plan.ground, cause = c.externalIntervalGroundRule(rule)
			case positive == 1:
				plan.identity, cause = c.externalIntervalIdentityRulePlan(rule)
			case positive == 2:
				plan.cartesian, cause = c.externalIntervalCartesianRule(rule)
			default:
				cause = ErrExternalIntervalDatalogFragment
			}
		}
		cause = errors.Join(cause, rule.Close())
		if cause != nil {
			return nil, cause
		}
		program.rules = append(program.rules, plan)
		program.declarations = append(program.declarations, plan.declarations()...)
	}
	finalized, err := c.set.IsFinalized()
	if err != nil {
		return nil, err
	}
	if !finalized {
		valid, cause := c.set.Finalize()
		if cause != nil {
			return nil, cause
		}
		if !valid {
			return nil, ErrExternalIntervalDatalogFragment
		}
	}
	for index := range program.rules {
		plan := &program.rules[index]
		declarations := plan.declarations()
		plan.stratum, err = c.set.PredicateStratum(declarations[0])
		if err != nil {
			return nil, err
		}
		program.schedule = append(program.schedule, index)
	}
	// A nonrecursive source stratum has one head; stable sorting preserves
	// the source rule vector order for multiple producers of that predicate.
	sort.SliceStable(program.schedule, func(i, j int) bool {
		return program.rules[program.schedule[i]].stratum < program.rules[program.schedule[j]].stratum
	})
	if err := c.externalIntervalProgramStrata(program); err != nil {
		return nil, err
	}
	return program, nil
}

func (c *externalIntervalDatalogCompiler) externalIntervalProgramStrata(program *externalIntervalDatalogProgram) error {
	strata, err := c.set.Strata()
	if err != nil {
		return err
	}
	byHead := make(map[ast.ASTHandle][]int)
	for index, rule := range program.rules {
		head := rule.declarations()[0]
		byHead[head] = append(byHead[head], index)
	}
	for _, declarations := range strata {
		heads := make([]ast.ASTHandle, 0, len(declarations))
		for _, declaration := range declarations {
			if len(byHead[declaration]) != 0 {
				heads = append(heads, declaration)
			}
		}
		if len(heads) == 0 {
			continue
		}
		inside := make(map[ast.ASTHandle]bool, len(heads))
		dependencies := make(map[ast.ASTHandle][]ast.ASTHandle, len(heads))
		for _, head := range heads {
			inside[head] = true
			for _, index := range byHead[head] {
				for _, tail := range program.rules[index].declarations()[1:] {
					dependencies[head] = append(dependencies[head], tail)
				}
			}
		}
		ordered, err := rel.SourceRecursivePredicateOrder(heads, dependencies)
		if err != nil {
			return errors.Join(err, ErrExternalIntervalDatalogFragment)
		}
		stage := externalIntervalDatalogProgramStratum{heads: ordered, cyclic: len(ordered) > 1}
		for _, head := range ordered {
			for _, index := range byHead[head] {
				plan := program.rules[index]
				insideTail := false
				for _, tail := range plan.declarations()[1:] {
					stratum, cause := c.set.PredicateStratum(tail)
					if cause != nil {
						return cause
					}
					if stratum < plan.stratum {
						continue
					}
					insideTail = true
					stage.cyclic = true
				}
				if !insideTail {
					stage.seeds = append(stage.seeds, index)
				}
				stage.allLoop = append(stage.allLoop, externalIntervalDatalogProgramAction{rule: index, inputTail: -1})
				for tailIndex, tail := range plan.declarations()[1:] {
					stratum, cause := c.set.PredicateStratum(tail)
					if cause != nil {
						return cause
					}
					if inside[tail] && stratum >= plan.stratum {
						stage.loop = append(stage.loop, externalIntervalDatalogProgramAction{rule: index, inputTail: tailIndex})
					}
				}
			}
		}
		program.strata = append(program.strata, stage)
	}
	return nil
}

func (a *externalIntervalDatalogActivation) installExternalIntervalProgram(program *externalIntervalDatalogProgram) error {
	if a.program != nil {
		if len(a.program.rules) != len(program.rules) {
			return ErrExternalIntervalDatalogFragment
		}
		for index, left := range a.program.rules {
			right := program.rules[index]
			if left.stratum != right.stratum || !(sameExternalIntervalIdentityRule(left.identity, right.identity) || sameExternalIntervalGroundRule(left.ground, right.ground) || sameExternalIntervalCartesianRule(left.cartesian, right.cartesian)) {
				return ErrExternalIntervalDatalogFragment
			}
		}
		return a.pinExternalIntervalProgramPlans()
	}
	if a.pending != nil || a.ground != nil {
		return ErrExternalIntervalDatalogFragment
	}
	if err := a.identity.close(a.manager); err != nil {
		return err
	}
	if err := a.cartesian.close(a.manager); err != nil {
		return err
	}
	a.program = program
	a.identity, a.cartesian = nil, nil
	return a.pinExternalIntervalProgramPlans()
}

// Adoption precedes rooting: a partially installed program stays owned by
// the activation, including every already-acquired application root.
func (a *externalIntervalDatalogActivation) pinExternalIntervalProgramPlans() error {
	for _, rule := range a.program.rules {
		if err := rule.identity.pin(a.manager); err != nil {
			return err
		}
		if err := rule.cartesian.pin(a.manager); err != nil {
			return err
		}
		if err := rule.ground.pin(a.manager); err != nil {
			return err
		}
	}
	return nil
}

func (a *externalIntervalDatalogActivation) closeExternalIntervalProgramPlans() error {
	if a.program == nil {
		return nil
	}
	for i := len(a.program.rules) - 1; i >= 0; i-- {
		if err := a.program.rules[i].identity.close(a.manager); err != nil {
			return err
		}
		if err := a.program.rules[i].cartesian.close(a.manager); err != nil {
			return err
		}
		if err := a.program.rules[i].ground.close(a.manager); err != nil {
			return err
		}
	}
	return nil
}

// prepareExternalIntervalNormalization retires a previously installed binary
// program while preserving the source relation entries it updated. A partial
// prior execution must finish through its normal continuation before its plan
// is released.
func (a *externalIntervalDatalogActivation) prepareExternalIntervalNormalization() error {
	if err := a.open(); err != nil {
		return err
	}
	if a.program == nil {
		return nil
	}
	if a.program.registers != nil || a.program.storing || a.pending != nil {
		if err := a.executeExternalIntervalProgram(); err != nil {
			return err
		}
	}
	if err := a.discardExternalIntervalProgramExecution(); err != nil {
		return err
	}
	if err := a.closeExternalIntervalProgramPlans(); err != nil {
		return err
	}
	a.program = nil
	a.identity, a.ground, a.cartesian = nil, nil, nil
	return nil
}

func (a *externalIntervalDatalogActivation) clearExternalIntervalProgramExecution() {
	a.program.registers = nil
	a.program.next = 0
	a.program.stratum = 0
	a.program.phase = 0
	a.program.actionNext = 0
	a.program.current = nil
	a.program.nextDelta = nil
	a.program.widen = false
	a.program.allOrNothing = false
	a.program.storing = false
	a.identity, a.cartesian = nil, nil
	a.ground = nil
}

func (a *externalIntervalDatalogActivation) discardExternalIntervalProgramExecution() error {
	if a.program == nil || a.program.registers == nil {
		return nil
	}
	if err := a.program.registers.Close(); err != nil {
		failure := finishExternalIntervalDatalogRegisterFailure(err, a.program.registers)
		errors.As(failure, &a.pending)
		return failure
	}
	a.clearExternalIntervalProgramExecution()
	return nil
}

func (a *externalIntervalDatalogActivation) executeExternalIntervalProgram() error {
	program := a.program
	completed, err := a.resumeExternalIntervalIdentity()
	if err != nil {
		return err
	}
	if len(program.strata) != 0 {
		if completed {
			program.actionNext++
		}
		return a.executeExternalIntervalProgramStrata(program)
	}
	if program.storing {
		a.clearExternalIntervalProgramExecution()
		return nil
	}
	if program.registers != nil && program.registers.closed {
		a.clearExternalIntervalProgramExecution()
	}
	if completed {
		program.next++
	}
	if program.registers == nil {
		registers, cause := a.loadExternalIntervalDatalogRegisters(program.declarations)
		if cause != nil {
			errors.As(cause, &a.pending)
			return cause
		}
		registers.deferStore = true
		program.registers = registers
	}
	for program.next < len(program.schedule) {
		plan := program.rules[program.schedule[program.next]]
		a.identity, a.cartesian = plan.identity, plan.cartesian
		a.ground = plan.ground
		if err := a.executeExternalIntervalIdentity(); err != nil {
			return err
		}
		program.next++
	}
	program.registers.deferStore = false
	program.storing = true
	if err := program.registers.store(); err != nil {
		var failure *externalIntervalDatalogRegisterFailure
		if errors.As(err, &failure) && failure.owner == program.registers {
			a.pending = failure
		} else {
			a.pending = newExternalIntervalDatalogIdentityFailure(err, program.registers, nil)
		}
		a.pendingCompletesRule = false
		return a.pending
	}
	a.clearExternalIntervalProgramExecution()
	return nil
}

func (a *externalIntervalDatalogActivation) executeExternalIntervalProgramStrata(program *externalIntervalDatalogProgram) error {
	if program.storing {
		a.clearExternalIntervalProgramExecution()
		return nil
	}
	if program.registers != nil && program.registers.closed {
		a.clearExternalIntervalProgramExecution()
	}
	if program.registers == nil {
		registers, err := a.loadExternalIntervalDatalogRegisters(program.declarations)
		if err != nil {
			errors.As(err, &a.pending)
			return err
		}
		registers.deferStore = true
		program.registers = registers
		program.stratum, program.phase, program.actionNext = 0, 0, 0
		program.current, program.nextDelta = nil, nil
	}
	registers := program.registers
	for program.stratum < len(program.strata) {
		stage := program.strata[program.stratum]
		if program.phase == 0 {
			for program.actionNext < len(stage.seeds) {
				index := stage.seeds[program.actionNext]
				if err := a.runExternalIntervalProgramAction(program, externalIntervalDatalogProgramAction{rule: index, inputTail: -1}, nil); err != nil {
					return fmt.Errorf("external interval seed rule %d: %w", index, err)
				}
				program.actionNext++
			}
			program.actionNext = 0
			if !stage.cyclic {
				program.stratum++
				continue
			}
			program.current = make(map[ast.ASTHandle]*rel.ExternalIntervalProduct, len(stage.heads))
			program.nextDelta = make(map[ast.ASTHandle]*rel.ExternalIntervalProduct, len(stage.heads))
			for _, head := range stage.heads {
				current, err := registers.cloneRelation(head)
				if err != nil {
					return err
				}
				next, err := registers.emptyProduct(head)
				if err != nil {
					return err
				}
				program.current[head], program.nextDelta[head] = current, next
			}
			program.phase = 1
		}
		active := false
		for _, head := range stage.heads {
			empty, err := program.current[head].Empty()
			if err != nil {
				return err
			}
			active = active || !empty
		}
		if !active {
			program.stratum++
			program.phase = 0
			program.current, program.nextDelta = nil, nil
			program.actionNext = 0
			continue
		}
		actions := stage.loop
		if program.allOrNothing {
			actions = stage.allLoop
		}
		for program.actionNext < len(actions) {
			action := actions[program.actionNext]
			var input *rel.ExternalIntervalProduct
			if action.inputTail >= 0 {
				declarations := program.rules[action.rule].declarations()
				if action.inputTail+1 >= len(declarations) {
					return fmt.Errorf("%w: recursive tail %d outside rule %d", ErrExternalIntervalDatalogFragment, action.inputTail, action.rule)
				}
				tail := declarations[action.inputTail+1]
				input = program.current[tail]
				if input == nil {
					return fmt.Errorf("%w: missing recursive input for rule %d tail %d declaration %v", ErrExternalIntervalDatalogFragment, action.rule, action.inputTail, tail)
				}
			}
			if err := a.runExternalIntervalProgramAction(program, action, input); err != nil {
				return fmt.Errorf("external interval recursive rule %d tail %d: %w", action.rule, action.inputTail, err)
			}
			program.actionNext++
		}
		program.actionNext = 0
		program.current, program.nextDelta = program.nextDelta, make(map[ast.ASTHandle]*rel.ExternalIntervalProduct, len(stage.heads))
		for _, head := range stage.heads {
			next, err := registers.emptyProduct(head)
			if err != nil {
				return err
			}
			program.nextDelta[head] = next
		}
	}
	registers.inputDelta = nil
	registers.outputHead, registers.output, registers.widen = ast.ASTHandle{}, nil, false
	registers.deferStore = false
	program.storing = true
	if err := registers.store(); err != nil {
		var failure *externalIntervalDatalogRegisterFailure
		if errors.As(err, &failure) && failure.owner == registers {
			a.pending = failure
		} else {
			a.pending = newExternalIntervalDatalogIdentityFailure(err, registers, nil)
		}
		a.pendingCompletesRule = false
		return a.pending
	}
	a.clearExternalIntervalProgramExecution()
	return nil
}

func (a *externalIntervalDatalogActivation) runExternalIntervalProgramAction(program *externalIntervalDatalogProgram, action externalIntervalDatalogProgramAction, input *rel.ExternalIntervalProduct) error {
	plan := program.rules[action.rule]
	a.identity, a.cartesian, a.ground = plan.identity, plan.cartesian, plan.ground
	registers := program.registers
	registers.inputDelta = nil
	registers.outputHead, registers.output, registers.widen = ast.ASTHandle{}, nil, false
	if program.phase == 1 {
		head := plan.declarations()[0]
		registers.outputHead = head
		registers.output = program.nextDelta[head]
		registers.widen = program.widen
		if action.inputTail >= 0 {
			registers.inputDelta = map[int]*rel.ExternalIntervalProduct{action.inputTail: input}
		}
	}
	return a.executeExternalIntervalIdentity()
}

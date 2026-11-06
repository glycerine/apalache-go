package fp

// Source: src/muz/rel/dl_compiler.cpp zero-tail head assembly and
// src/muz/rel/dl_instruction.cpp LOAD, TOTAL, MK_UNARY_SINGLETON, JOIN, UNION,
// DEALLOC, and STORE.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
)

type externalIntervalDatalogGroundRule struct {
	head            ast.ASTHandle
	application     ast.ASTHandle
	values          []ast.ASTHandle
	headConstants   []externalIntervalCartesianHeadConstant
	headUnbound     []externalIntervalCartesianHeadVariable
	headAdditions   []externalIntervalCartesianHeadAddition
	duplicates      []uint32
	renames         [][]uint32
	guardRemoved    []uint32
	condition       *externalIntervalUnaryCondition
	applicationRoot ast.ASTRef
}

func (r *externalIntervalDatalogGroundRule) close(manager *ast.ASTManager) error {
	if r == nil {
		return nil
	}
	if err := r.condition.close(manager); err != nil {
		return err
	}
	if r.applicationRoot != (ast.ASTRef{}) {
		return manager.DelRef(&r.applicationRoot)
	}
	return nil
}

func (r *externalIntervalDatalogGroundRule) pin(manager *ast.ASTManager) error {
	if r == nil {
		return nil
	}
	if r.applicationRoot == (ast.ASTRef{}) {
		root, err := manager.NewRef(r.application)
		if err != nil {
			return err
		}
		r.applicationRoot = root
	}
	return r.condition.prepare(manager)
}

func sameExternalIntervalGroundRule(left, right *externalIntervalDatalogGroundRule) bool {
	if left == nil || right == nil || left.head != right.head || len(left.values) != len(right.values) || len(left.headConstants) != len(right.headConstants) || len(left.headUnbound) != len(right.headUnbound) || len(left.headAdditions) != len(right.headAdditions) || !externalIntervalColumnsEqual(left.duplicates, right.duplicates) || !externalIntervalColumnsEqual(left.guardRemoved, right.guardRemoved) || !sameExternalIntervalUnaryCondition(left.condition, right.condition) || len(left.renames) != len(right.renames) {
		return false
	}
	for i, value := range left.values {
		if value != right.values[i] {
			return false
		}
	}
	for i, constant := range left.headConstants {
		other := right.headConstants[i]
		if constant.sort != other.sort || constant.value != other.value {
			return false
		}
	}
	for i := range left.headUnbound {
		if left.headUnbound[i] != right.headUnbound[i] {
			return false
		}
	}
	for i := range left.headAdditions {
		if left.headAdditions[i] != right.headAdditions[i] {
			return false
		}
	}
	for i := range left.renames {
		if !externalIntervalColumnsEqual(left.renames[i], right.renames[i]) {
			return false
		}
	}
	return true
}

func (e *PropositionalEngine) externalIntervalGroundRule(rule *base.Rule) (*externalIntervalDatalogGroundRule, error) {
	return e.externalIntervalCompiler().externalIntervalGroundRule(rule)
}

func (c *externalIntervalDatalogCompiler) externalIntervalGroundRule(rule *base.Rule) (*externalIntervalDatalogGroundRule, error) {
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return nil, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, err
	}
	if positive != 0 || uninterpreted != 0 {
		return nil, ErrExternalIntervalDatalogFragment
	}
	tailSize, err := rule.TailSize()
	if err != nil {
		return nil, err
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	declaration, err := c.manager.GetDecl(head)
	if err != nil {
		return nil, err
	}
	state := c.predicates[declaration]
	if state == nil || !isExternalIntervalRepresentation(state.kinds) {
		return nil, ErrExternalIntervalDatalogFragment
	}
	count, err := c.manager.GetNumArgs(head)
	if err != nil {
		return nil, err
	}
	if int(count) != len(state.sorts) {
		return nil, ErrExternalIntervalDatalogFragment
	}
	plan := &externalIntervalDatalogGroundRule{head: declaration, application: head}
	values := make([]ast.ASTHandle, count)
	headSources := make([]uint32, count)
	variableSources := make(map[uint32]uint32)
	width := uint32(0)
	for i := range values {
		value, cause := c.manager.GetArg(head, uint32(i))
		if cause != nil {
			return nil, fmt.Errorf("ground head argument %d: %w", i, cause)
		}
		sort, cause := c.manager.GetSort(value)
		if cause != nil {
			return nil, fmt.Errorf("ground head argument %d sort: %w", i, cause)
		}
		kind, cause := c.manager.GetKind(value)
		if cause != nil {
			return nil, fmt.Errorf("ground head argument %d kind: %w", i, cause)
		}
		if sort != state.sorts[i] {
			return nil, ErrExternalIntervalDatalogFragment
		}
		if kind == ast.ASTVar {
			variable, cause := c.manager.GetVarIndex(value)
			if cause != nil {
				return nil, fmt.Errorf("ground head argument %d variable index: %w", i, cause)
			}
			source, found := variableSources[variable]
			if !found {
				index := len(plan.headUnbound)
				plan.headUnbound = append(plan.headUnbound, externalIntervalCartesianHeadVariable{sort: state.sorts[i]})
				plan.headAdditions = append(plan.headAdditions, externalIntervalCartesianHeadAddition{total: true, index: index})
				source = width
				width++
				variableSources[variable] = source
			}
			headSources[i] = source
			continue
		}
		isValue, cause := c.manager.IsValue(value)
		if cause != nil {
			return nil, fmt.Errorf("ground head argument %d value: %w", i, cause)
		}
		if !isValue {
			return nil, ErrExternalIntervalDatalogFragment
		}
		_, _, numeral, cause := c.manager.ArithmeticNumeralText(value)
		if cause != nil {
			return nil, fmt.Errorf("ground head argument %d numeral: %w", i, cause)
		}
		if !numeral {
			return nil, ErrExternalIntervalDatalogFragment
		}
		values[i] = value
		index := len(plan.headConstants)
		plan.headConstants = append(plan.headConstants, externalIntervalCartesianHeadConstant{sort: sort, value: value})
		plan.headAdditions = append(plan.headAdditions, externalIntervalCartesianHeadAddition{index: index})
		headSources[i] = width
		width++
	}
	if tailSize != 0 {
		boolean, cause := c.manager.MkBoolSort()
		if cause != nil {
			return nil, cause
		}
		used, cause := ast.NewASTUsedVars(c.manager)
		if cause != nil {
			return nil, cause
		}
		condition := &externalIntervalUnaryCondition{}
		for index := uninterpreted; index < tailSize; index++ {
			guard, negated, cause := rule.Tail(index)
			if cause != nil {
				return nil, cause
			}
			kind, cause := c.manager.GetKind(guard)
			if cause != nil {
				return nil, cause
			}
			sort, cause := c.manager.GetSort(guard)
			if cause != nil {
				return nil, cause
			}
			if negated || kind != ast.ASTApp || sort != boolean {
				return nil, ErrExternalIntervalDatalogFragment
			}
			if cause = used.Collect(guard); cause != nil {
				return nil, cause
			}
			condition.source = append(condition.source, guard)
		}
		for variable := uint32(0); variable < used.MaxFoundVarIndexPlusOne(); variable++ {
			sort := used.Contains(variable)
			if sort.IsNull() {
				continue
			}
			column, found := variableSources[variable]
			if !found {
				index := len(plan.headUnbound)
				plan.headUnbound = append(plan.headUnbound, externalIntervalCartesianHeadVariable{sort: sort})
				plan.headAdditions = append(plan.headAdditions, externalIntervalCartesianHeadAddition{total: true, index: index})
				column = width
				width++
				plan.guardRemoved = append(plan.guardRemoved, column)
			}
			condition.bindings = append(condition.bindings, externalIntervalUnaryConditionBinding{variable: variable, column: column, sort: sort})
		}
		plan.condition = condition
	}
	plan.values = values
	permutation := make([]uint32, len(headSources))
	usedSources := make(map[uint32]bool)
	for column, source := range headSources {
		if usedSources[source] {
			plan.duplicates = append(plan.duplicates, source)
			permutation[column] = width
			width++
		} else {
			usedSources[source] = true
			permutation[column] = source
		}
	}
	for column := range permutation {
		if permutation[column] == uint32(column) {
			continue
		}
		var cycle []uint32
		next := uint32(column)
		for {
			cycle = append(cycle, next)
			previous := next
			next = permutation[previous]
			permutation[previous] = previous
			if next == uint32(column) {
				break
			}
		}
		for i := 0; i+1 < len(cycle); i++ {
			plan.renames = append(plan.renames, []uint32{cycle[i], cycle[i+1]})
		}
	}
	return plan, nil
}

// externalIntervalRulePlans validates the complete retained one-rule fragment
// before relation callbacks. The live plans are the aligned identity edge, a
// zero-tail arithmetic numeral fact, and two positive variable-only tails with
// a bound, distinct-variable head.
func (e *PropositionalEngine) externalIntervalRulePlans() (_ *externalIntervalDatalogIdentityRule, _ *externalIntervalDatalogGroundRule, _ *externalIntervalDatalogCartesianRule, resultErr error) {
	return e.externalIntervalCompiler().externalIntervalRulePlans()
}

func (c *externalIntervalDatalogCompiler) externalIntervalRulePlans() (_ *externalIntervalDatalogIdentityRule, _ *externalIntervalDatalogGroundRule, _ *externalIntervalDatalogCartesianRule, resultErr error) {
	count, err := c.set.Len()
	if err != nil {
		return nil, nil, nil, err
	}
	if count == 0 {
		return nil, nil, nil, nil
	}
	if count != 1 {
		return nil, nil, nil, ErrExternalIntervalDatalogFragment
	}
	rule, err := c.set.Rule(0)
	if err != nil {
		return nil, nil, nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, rule.Close()) }()
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return nil, nil, nil, err
	}
	if positive == 0 {
		ground, cause := c.externalIntervalGroundRule(rule)
		return nil, ground, nil, cause
	}
	if positive == 1 {
		identity, cause := c.externalIntervalIdentityRulePlan(rule)
		return identity, nil, nil, cause
	}
	if positive == 2 {
		cartesian, cause := c.externalIntervalCartesianRule(rule)
		return nil, nil, cartesian, cause
	}
	return nil, nil, nil, ErrExternalIntervalDatalogFragment
}

func (a *externalIntervalDatalogActivation) installExternalIntervalRulePlans(identity *externalIntervalDatalogIdentityRule, ground *externalIntervalDatalogGroundRule, cartesian *externalIntervalDatalogCartesianRule) error {
	if a.program != nil {
		return ErrExternalIntervalDatalogFragment
	}
	selected := 0
	if identity != nil {
		selected++
	}
	if ground != nil {
		selected++
	}
	if cartesian != nil {
		selected++
	}
	if selected > 1 {
		return ErrExternalIntervalDatalogFragment
	}
	if a.ground != nil {
		if !sameExternalIntervalGroundRule(a.ground, ground) || identity != nil || cartesian != nil {
			return ErrExternalIntervalDatalogFragment
		}
		ground = a.ground
	}
	if a.identity != nil && identity != nil {
		if !sameExternalIntervalIdentityRule(a.identity, identity) {
			return ErrExternalIntervalDatalogFragment
		}
		identity = a.identity
	}
	if a.cartesian != nil && cartesian != nil {
		if !sameExternalIntervalCartesianRule(a.cartesian, cartesian) {
			return ErrExternalIntervalDatalogFragment
		}
		cartesian = a.cartesian
	}
	if a.identity != nil && (ground != nil || cartesian != nil) ||
		a.ground != nil && (identity != nil || cartesian != nil) ||
		a.cartesian != nil && (identity != nil || ground != nil) {
		return ErrExternalIntervalDatalogFragment
	}
	if err := ground.pin(a.manager); err != nil {
		return err
	}
	if identity == nil {
		if err := a.identity.close(a.manager); err != nil {
			return err
		}
	}
	if cartesian == nil {
		if err := a.cartesian.close(a.manager); err != nil {
			return err
		}
	}
	a.identity = identity
	a.ground = ground
	a.cartesian = cartesian
	if err := identity.pin(a.manager); err != nil {
		return err
	}
	return cartesian.pin(a.manager)
}

type externalIntervalDatalogGroundContinuation struct {
	registers *externalIntervalDatalogRegisters
	temporary *rel.ExternalIntervalProduct
	tuple     *externalIntervalDatalogGroundTuple
	nested    memory.ConstructionFailure
	runtime   *memory.Runtime
	publish   bool
}

func (c *externalIntervalDatalogGroundContinuation) resume() error {
	if c.nested != nil {
		if err := c.nested.Cleanup(); err != nil {
			return err
		}
		c.nested = nil
	}
	// A joined head register is explicitly deallocated by the source rule
	// code before termination STORE; cached unary singletons outlive STORE.
	if c.tuple != nil {
		if err := c.tuple.closeJoined(); err != nil {
			return err
		}
	}
	if c.publish && c.registers != nil {
		if err := c.registers.store(); err != nil {
			var owner *externalIntervalDatalogRegisterFailure
			if errors.As(err, &owner) && owner.owner == c.registers {
				c.nested = owner
				c.registers = nil
			}
			return err
		}
		c.registers = nil
	}
	if c.temporary != nil {
		if err := c.temporary.Close(); err != nil {
			return err
		}
		c.temporary = nil
	}
	if c.tuple != nil {
		if err := c.tuple.closeSingletons(); err != nil {
			return err
		}
		c.tuple = nil
	}
	if !c.publish && c.registers != nil {
		if err := c.registers.Close(); err != nil {
			return err
		}
		c.registers = nil
	}
	return nil
}

type externalIntervalDatalogGroundFailure struct {
	original, cleanup error
	reported          error
	text              string
	owner             *externalIntervalDatalogGroundContinuation
}

var _ memory.ConstructionFailure = (*externalIntervalDatalogGroundFailure)(nil)

func (f *externalIntervalDatalogGroundFailure) Error() string        { return f.text }
func (f *externalIntervalDatalogGroundFailure) Unwrap() error        { return f.reported }
func (f *externalIntervalDatalogGroundFailure) OriginalCause() error { return f.original }
func (f *externalIntervalDatalogGroundFailure) CleanupCause() error  { return f.cleanup }
func (f *externalIntervalDatalogGroundFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.runtime
}
func (f *externalIntervalDatalogGroundFailure) Cleanup() error {
	f.cleanup = f.owner.resume()
	return f.cleanup
}

func newExternalIntervalDatalogGroundFailure(cause error, owner *externalIntervalDatalogGroundContinuation) *externalIntervalDatalogGroundFailure {
	return &externalIntervalDatalogGroundFailure{
		original: cause, cleanup: cause, reported: cause, text: cause.Error(), owner: owner,
	}
}

func finishExternalIntervalDatalogGroundFailure(cause error, owner *externalIntervalDatalogGroundContinuation) error {
	if cause != nil {
		errors.As(cause, &owner.nested)
	}
	cleanup := owner.resume()
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &externalIntervalDatalogGroundFailure{
		original: original, cleanup: cleanup, reported: reported,
		text: reported.Error(), owner: owner,
	}
}

func (a *externalIntervalDatalogActivation) executeExternalIntervalGround() error {
	registers, err := a.loadExternalIntervalDatalogRegisters([]ast.ASTHandle{a.ground.head})
	if err != nil {
		var pending memory.ConstructionFailure
		if errors.As(err, &pending) {
			a.pending = pending
			a.pendingCompletesRule = false
			a.pendingIdentity = nil
			a.pendingGround = nil
			a.pendingCartesian = nil
		}
		return fmt.Errorf("load zero-tail relation registers: %w", err)
	}
	owner := &externalIntervalDatalogGroundContinuation{
		registers: registers,
		runtime:   registers.runtime,
	}
	rollback := func(cause error) error {
		finished := finishExternalIntervalDatalogGroundFailure(cause, owner)
		var pending memory.ConstructionFailure
		if errors.As(finished, &pending) {
			a.pending = pending
			a.pendingCompletesRule = false
			a.pendingIdentity = nil
			a.pendingGround = nil
			a.pendingCartesian = nil
		}
		owner = nil
		return finished
	}
	entry := a.entries[a.ground.head]
	if entry == nil {
		return rollback(ErrExternalIntervalDatalogFragment)
	}
	var temporary *rel.ExternalIntervalProduct
	var selected bool
	if len(a.ground.headUnbound) != 0 || a.ground.condition != nil {
		owner.tuple = &externalIntervalDatalogGroundTuple{}
		err = owner.tuple.assembleHead(registers, a.ground.head, a.ground.headConstants, a.ground.headUnbound, a.ground.headAdditions)
		if err != nil {
			return rollback(fmt.Errorf("assemble zero-tail variable head: %w", err))
		}
		if a.ground.condition != nil {
			if owner.tuple.current == nil {
				owner.tuple.current, err = registers.nullaryTotal(a.ground.head)
				if err != nil {
					return rollback(err)
				}
			}
			condition, cause := a.ground.condition.handle(a.manager)
			if cause != nil {
				return rollback(cause)
			}
			if len(a.ground.guardRemoved) != 0 {
				projected, cause := filterProjectExternalIntervalUnary(owner.tuple.current, condition, a.ground.guardRemoved, registers.runtime)
				if projected != nil {
					owner.tuple.retiring, owner.tuple.joined, owner.tuple.current = owner.tuple.joined, projected, projected
				}
				if cause != nil {
					return rollback(cause)
				}
				if owner.tuple.retiring != nil {
					if cause = owner.tuple.retiring.Close(); cause != nil {
						return rollback(cause)
					}
					owner.tuple.retiring = nil
				}
			} else if err = owner.tuple.filterInterpreted(condition); err != nil {
				return rollback(err)
			}
		}
		for _, column := range a.ground.duplicates {
			if err = owner.tuple.duplicateColumn(column); err != nil {
				return rollback(err)
			}
		}
		for _, cycle := range a.ground.renames {
			if err = owner.tuple.rename(cycle); err != nil {
				return rollback(err)
			}
		}
		temporary = owner.tuple.current
		selected = temporary != nil
	} else if len(a.ground.values) > 1 {
		owner.tuple = &externalIntervalDatalogGroundTuple{}
		if a.program != nil {
			err = owner.tuple.assembleCached(registers, a.ground.head, entry.sorts, a.ground.values)
		} else {
			err = owner.tuple.assemble(a.plugin, entry.kinds, entry.sorts, a.ground.values)
		}
		if err != nil {
			return rollback(err)
		}
		temporary = owner.tuple.current
		selected = temporary != nil
	} else if a.program != nil {
		if len(a.ground.values) == 0 {
			temporary, err = registers.nullaryTotal(a.ground.head)
		} else {
			temporary, err = registers.unarySingleton(a.ground.head, entry.sorts[0], a.ground.values[0])
		}
		selected = temporary != nil
	} else if len(a.ground.values) == 0 {
		temporary, selected, err = rel.NewSelectedExternalIntervalNullaryTotal(a.plugin, entry.kinds)
	} else {
		temporary, selected, err = rel.NewSelectedExternalIntervalProduct(a.plugin, entry.sorts, entry.kinds)
	}
	if err != nil {
		return rollback(err)
	}
	if !selected || temporary == nil {
		return rollback(ErrExternalIntervalDatalogFragment)
	}
	if owner.tuple == nil && a.program == nil {
		owner.temporary = temporary
	}
	if len(a.ground.headUnbound) == 0 && a.ground.condition == nil && len(a.ground.values) == 1 && a.program == nil {
		if err = temporary.AddFact(a.ground.values); err != nil {
			return rollback(err)
		}
	}
	if err = registers.union(a.ground.head, temporary); err != nil {
		err = fmt.Errorf("publish ground head: %w", err)
		if rel.ExternalAssignmentCommitted(err) {
			var nested memory.ConstructionFailure
			if !errors.As(err, &nested) {
				return rollback(fmt.Errorf("%w: committed union has no cleanup owner", ErrExternalIntervalDatalogFragment))
			}
			owner.nested = nested
			owner.publish = true
			failure := newExternalIntervalDatalogGroundFailure(err, owner)
			a.pending = failure
			a.pendingCompletesRule = true
			a.pendingIdentity = nil
			a.pendingGround = a.ground
			a.pendingCartesian = nil
			return failure
		}
		return rollback(err)
	}
	owner.publish = true
	if err = owner.resume(); err != nil {
		err = fmt.Errorf("finish ground head: %w", err)
		failure := newExternalIntervalDatalogGroundFailure(err, owner)
		a.pending = failure
		a.pendingCompletesRule = true
		a.pendingIdentity = nil
		a.pendingGround = a.ground
		a.pendingCartesian = nil
		return failure
	}
	return nil
}

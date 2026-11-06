package fp

// Source: src/muz/rel/dl_compiler.cpp aligned one-tail rule lowering and
// src/muz/rel/dl_instruction.cpp LOAD, UNION, and STORE execution.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
)

type externalIntervalDatalogIdentityRule struct {
	head      ast.ASTHandle
	tail      ast.ASTHandle
	transform *externalIntervalDatalogUnaryTransform
}

func sameExternalIntervalIdentityRule(left, right *externalIntervalDatalogIdentityRule) bool {
	return left != nil && right != nil && left.head == right.head && left.tail == right.tail && sameExternalIntervalUnaryTransform(left.transform, right.transform)
}

// externalIntervalIdentityRule validates the complete rule fragment without
// invoking relation callbacks. Positive variable-only tails may filter repeated
// columns and project or permute distinct bound head variables.
func (e *PropositionalEngine) externalIntervalIdentityRule() (_ *externalIntervalDatalogIdentityRule, resultErr error) {
	count, err := e.set.Len()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, nil
	}
	if count != 1 {
		return nil, ErrExternalIntervalDatalogFragment
	}
	rule, err := e.set.Rule(0)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, rule.Close()) }()
	return e.externalIntervalIdentityRulePlan(rule)
}

func (e *PropositionalEngine) externalIntervalIdentityRulePlan(rule *base.Rule) (*externalIntervalDatalogIdentityRule, error) {
	return e.externalIntervalCompiler().externalIntervalIdentityRulePlan(rule)
}

func (c *externalIntervalDatalogCompiler) externalIntervalIdentityRulePlan(rule *base.Rule) (*externalIntervalDatalogIdentityRule, error) {
	head, tail, transform, err := c.externalIntervalUnaryPlan(rule)
	if err != nil {
		return nil, err
	}
	return &externalIntervalDatalogIdentityRule{head: head, tail: tail, transform: transform}, nil
}

type externalIntervalDatalogIdentityContinuation struct {
	registers *externalIntervalDatalogRegisters
	nested    memory.ConstructionFailure
	runtime   *memory.Runtime
}

func (c *externalIntervalDatalogIdentityContinuation) resume() error {
	if c.nested != nil {
		if err := c.nested.Cleanup(); err != nil {
			return err
		}
		c.nested = nil
	}
	if c.registers == nil {
		return nil
	}
	if err := c.registers.store(); err != nil {
		var owner *externalIntervalDatalogRegisterFailure
		if errors.As(err, &owner) && owner.owner == c.registers {
			c.nested = owner
			c.registers = nil
		}
		return err
	}
	c.registers = nil
	return nil
}

type externalIntervalDatalogIdentityFailure struct {
	original, cleanup error
	reported          error
	text              string
	owner             *externalIntervalDatalogIdentityContinuation
}

var _ memory.ConstructionFailure = (*externalIntervalDatalogIdentityFailure)(nil)

func (f *externalIntervalDatalogIdentityFailure) Error() string        { return f.text }
func (f *externalIntervalDatalogIdentityFailure) Unwrap() error        { return f.reported }
func (f *externalIntervalDatalogIdentityFailure) OriginalCause() error { return f.original }
func (f *externalIntervalDatalogIdentityFailure) CleanupCause() error  { return f.cleanup }
func (f *externalIntervalDatalogIdentityFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.runtime
}
func (f *externalIntervalDatalogIdentityFailure) Cleanup() error {
	f.cleanup = f.owner.resume()
	return f.cleanup
}

func newExternalIntervalDatalogIdentityFailure(cause error, registers *externalIntervalDatalogRegisters, nested memory.ConstructionFailure) *externalIntervalDatalogIdentityFailure {
	return &externalIntervalDatalogIdentityFailure{
		original: cause, cleanup: cause, reported: cause, text: cause.Error(),
		owner: &externalIntervalDatalogIdentityContinuation{
			registers: registers,
			nested:    nested,
			runtime:   registers.runtime,
		},
	}
}

func (a *externalIntervalDatalogActivation) resumeExternalIntervalIdentity() (bool, error) {
	if a.pending == nil {
		return false, nil
	}
	if err := a.pending.Cleanup(); err != nil {
		return false, err
	}
	completed := a.pendingCompletesRule && (sameExternalIntervalIdentityRule(a.pendingIdentity, a.identity) ||
		sameExternalIntervalGroundRule(a.pendingGround, a.ground) ||
		sameExternalIntervalCartesianRule(a.pendingCartesian, a.cartesian))
	a.pending = nil
	a.pendingCompletesRule = false
	a.pendingIdentity = nil
	a.pendingGround = nil
	a.pendingCartesian = nil
	return completed, nil
}

func (a *externalIntervalDatalogActivation) executeExternalIntervalIdentity() (resultErr error) {
	if err := a.open(); err != nil {
		return err
	}
	completed, err := a.resumeExternalIntervalIdentity()
	if err != nil {
		return err
	}
	if completed {
		return nil
	}
	if a.identity == nil {
		if a.ground != nil {
			return a.executeExternalIntervalGround()
		}
		if a.cartesian != nil {
			return a.executeExternalIntervalCartesian()
		}
		return nil
	}
	if a.identity.transform != nil {
		return a.executeExternalIntervalUnary(a.identity, a.identity.transform)
	}
	registers, err := a.loadExternalIntervalDatalogRegisters([]ast.ASTHandle{a.identity.head, a.identity.tail})
	if err != nil {
		var pending memory.ConstructionFailure
		if errors.As(err, &pending) {
			a.pending = pending
			a.pendingCompletesRule = false
			a.pendingIdentity = nil
			a.pendingGround = nil
			a.pendingCartesian = nil
		}
		return err
	}
	rollback := func(cause error) error {
		finished := finishExternalIntervalDatalogRegisterFailure(cause, registers)
		var pending memory.ConstructionFailure
		if errors.As(finished, &pending) {
			a.pending = pending
			a.pendingCompletesRule = false
			a.pendingIdentity = nil
			a.pendingGround = nil
			a.pendingCartesian = nil
		}
		registers = nil
		return finished
	}
	source, err := registers.relationAt(a.identity.tail, 0)
	if err != nil {
		return rollback(err)
	}
	if source != nil {
		cause := registers.union(a.identity.head, source)
		if cause != nil {
			if rel.ExternalAssignmentCommitted(cause) {
				var nested memory.ConstructionFailure
				if !errors.As(cause, &nested) {
					return rollback(fmt.Errorf("%w: committed union has no cleanup owner", ErrExternalIntervalDatalogFragment))
				}
				failure := newExternalIntervalDatalogIdentityFailure(cause, registers, nested)
				a.pending = failure
				a.pendingCompletesRule = true
				a.pendingIdentity = a.identity
				a.pendingGround = nil
				a.pendingCartesian = nil
				registers = nil
				return failure
			}
			return rollback(cause)
		}
	}
	if err = registers.store(); err != nil {
		var pending *externalIntervalDatalogRegisterFailure
		if errors.As(err, &pending) && pending.owner == registers {
			a.pending = pending
			a.pendingCompletesRule = true
			a.pendingIdentity = a.identity
			a.pendingGround = nil
			a.pendingCartesian = nil
			registers = nil
			return err
		}
		failure := newExternalIntervalDatalogIdentityFailure(err, registers, nil)
		a.pending = failure
		a.pendingCompletesRule = true
		a.pendingIdentity = a.identity
		a.pendingGround = nil
		a.pendingCartesian = nil
		registers = nil
		return failure
	}
	registers = nil
	return nil
}

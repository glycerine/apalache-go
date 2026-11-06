package fp

// Source: dl_compiler.cpp interpreted-tail conjunction and column binding.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
)

type externalIntervalUnaryConditionBinding struct {
	variable, column uint32
	sort             ast.ASTHandle
}

// Immutable source/bindings describe the compiled condition. Source roots
// protect those handles through failed preparation and later installation retry.
type externalIntervalUnaryCondition struct {
	source      []ast.ASTHandle
	bindings    []externalIntervalUnaryConditionBinding
	manager     *ast.ASTManager
	sourceRoots []ast.ASTRef
	mapped      ast.ASTRef
	attempt     *externalIntervalUnaryConditionAttempt
}

type externalIntervalUnaryConditionAttempt struct {
	manager      *ast.ASTManager
	nested       memory.ConstructionFailure
	substitution *astrewriter.VarSubstitution
	roots        []ast.ASTRef
	mapped       ast.ASTRef
}

func (a *externalIntervalUnaryConditionAttempt) closeTemporary() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	if a.substitution != nil {
		if err := a.substitution.Close(); err != nil {
			return err
		}
		a.substitution = nil
	}
	for len(a.roots) != 0 {
		last := len(a.roots) - 1
		err := a.manager.DelRef(&a.roots[last])
		if a.roots[last] == (ast.ASTRef{}) {
			a.roots = a.roots[:last]
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *externalIntervalUnaryConditionAttempt) close() error {
	if err := a.closeTemporary(); err != nil {
		return err
	}
	if a.mapped != (ast.ASTRef{}) {
		return a.manager.DelRef(&a.mapped)
	}
	return nil
}

type externalIntervalUnaryConditionFailure struct {
	original, cleanup, reported error
	text                        string
	owner                       *externalIntervalUnaryConditionAttempt
}

var _ memory.ConstructionFailure = (*externalIntervalUnaryConditionFailure)(nil)

func (f *externalIntervalUnaryConditionFailure) Error() string        { return f.text }
func (f *externalIntervalUnaryConditionFailure) Unwrap() error        { return f.reported }
func (f *externalIntervalUnaryConditionFailure) OriginalCause() error { return f.original }
func (f *externalIntervalUnaryConditionFailure) CleanupCause() error  { return f.cleanup }
func (f *externalIntervalUnaryConditionFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *externalIntervalUnaryConditionFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}

func (c *externalIntervalUnaryCondition) failPreparation(cause error) error {
	owner := c.attempt
	if cause != nil {
		errors.As(cause, &owner.nested)
	}
	cleanup := owner.close()
	if cleanup == nil {
		c.attempt = nil
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &externalIntervalUnaryConditionFailure{
		original: original, cleanup: cleanup, reported: reported,
		text: reported.Error(), owner: owner,
	}
}

func (c *externalIntervalUnaryCondition) prepare(manager *ast.ASTManager) error {
	if c == nil {
		return nil
	}
	if manager == nil || c.manager != nil && c.manager != manager {
		return ast.ErrASTHandle
	}
	c.manager = manager
	if c.attempt != nil {
		if err := c.attempt.close(); err != nil {
			return err
		}
		c.attempt = nil
	}
	if c.mapped != (ast.ASTRef{}) {
		return nil
	}
	if len(c.source) == 0 {
		return ErrExternalIntervalDatalogFragment
	}
	attempt := &externalIntervalUnaryConditionAttempt{manager: manager}
	c.attempt = attempt
	for len(c.sourceRoots) < len(c.source) {
		root, err := manager.NewRef(c.source[len(c.sourceRoots)])
		if err != nil {
			return c.failPreparation(err)
		}
		c.sourceRoots = append(c.sourceRoots, root)
	}
	free, err := astrewriter.NewExprFreeVars(manager)
	if err != nil {
		return c.failPreparation(err)
	}
	for _, source := range c.source {
		if err = free.Accumulate(source); err != nil {
			return c.failPreparation(err)
		}
	}
	byVariable := make(map[uint32]externalIntervalUnaryConditionBinding, len(c.bindings))
	for _, binding := range c.bindings {
		if _, exists := byVariable[binding.variable]; exists {
			return c.failPreparation(ErrExternalIntervalDatalogFragment)
		}
		byVariable[binding.variable] = binding
	}
	bindings := make([]ast.ASTHandle, free.Size())
	for variable := uint32(0); variable < free.Size(); variable++ {
		sort, cause := free.Get(variable)
		if cause != nil {
			return c.failPreparation(cause)
		}
		if sort.IsNull() {
			continue
		}
		binding, found := byVariable[variable]
		if !found || binding.sort != sort {
			return c.failPreparation(ErrExternalIntervalDatalogFragment)
		}
		// Reserve an owned reference before creating a fresh variable: a
		// later reference-page allocation must not orphan its zero-ref node.
		root, cause := manager.NewRef(ast.ASTHandle{})
		if cause != nil {
			return c.failPreparation(cause)
		}
		attempt.roots = append(attempt.roots, root)
		value, cause := manager.MkVar(binding.column, sort)
		// Trace emission can fail after the constructor returns a live node.
		if !value.IsNull() {
			cause = errors.Join(cause, manager.SetRef(root, value))
		}
		if cause != nil {
			return c.failPreparation(cause)
		}
		bindings[variable] = value
	}
	condition := c.source[0]
	if len(c.source) != 1 {
		root, cause := manager.NewRef(ast.ASTHandle{})
		if cause != nil {
			return c.failPreparation(cause)
		}
		attempt.roots = append(attempt.roots, root)
		condition, err = manager.MkAndArgs(c.source)
		if !condition.IsNull() {
			err = errors.Join(err, manager.SetRef(root, condition))
		}
		if err != nil {
			return c.failPreparation(err)
		}
	}
	// NewVarSubstitution may return a partial owner alongside an error.
	attempt.substitution, err = astrewriter.NewVarSubstitution(manager, false)
	if err != nil {
		return c.failPreparation(err)
	}
	attempt.mapped, err = manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return c.failPreparation(err)
	}
	if err = attempt.substitution.Substitute(condition, bindings, attempt.mapped); err != nil {
		return c.failPreparation(err)
	}
	if err = attempt.closeTemporary(); err != nil {
		return c.failPreparation(err)
	}
	c.mapped, attempt.mapped = attempt.mapped, ast.ASTRef{}
	c.attempt = nil
	return nil
}

func (c *externalIntervalUnaryCondition) handle(manager *ast.ASTManager) (ast.ASTHandle, error) {
	if c == nil || manager == nil || c.manager != manager || c.mapped == (ast.ASTRef{}) || c.attempt != nil {
		return ast.ASTHandle{}, ErrExternalIntervalDatalogFragment
	}
	return manager.RefGet(c.mapped)
}

func (c *externalIntervalUnaryCondition) close(manager *ast.ASTManager) error {
	if c == nil {
		return nil
	}
	if manager == nil || c.manager != nil && c.manager != manager {
		return ast.ErrASTHandle
	}
	if c.attempt != nil {
		if err := c.attempt.close(); err != nil {
			return err
		}
		c.attempt = nil
	}
	if c.mapped != (ast.ASTRef{}) {
		if err := manager.DelRef(&c.mapped); err != nil {
			return err
		}
	}
	for len(c.sourceRoots) != 0 {
		last := len(c.sourceRoots) - 1
		err := manager.DelRef(&c.sourceRoots[last])
		if c.sourceRoots[last] == (ast.ASTRef{}) {
			c.sourceRoots = c.sourceRoots[:last]
		}
		if err != nil {
			return err
		}
	}
	return nil
}

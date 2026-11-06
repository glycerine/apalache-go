package rel

// Source: src/muz/rel/dl_compiler.cpp interpreted-tail binding and
// src/muz/rel/dl_external_relation.cpp filter_interpreted_fn.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
)

type externalInterpretedAttempt struct {
	manager      *ast.ASTManager
	nested       memory.ConstructionFailure
	substitution *astrewriter.VarSubstitution
	roots        []ast.ASTRef
}

func (a *externalInterpretedAttempt) close() error {
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
		index := len(a.roots) - 1
		err := a.manager.DelRef(&a.roots[index])
		if a.roots[index] == (ast.ASTRef{}) {
			a.roots = a.roots[:index]
		}
		if err != nil {
			return err
		}
		if len(a.roots) != index {
			return ast.ErrASTPending
		}
	}
	return nil
}

type externalInterpretedCleanupFailure struct {
	attempt               *externalInterpretedAttempt
	cause, cleanup, shown error
	message               string
}

func (f *externalInterpretedCleanupFailure) Error() string { return f.message }
func (f *externalInterpretedCleanupFailure) Unwrap() error { return f.shown }
func (f *externalInterpretedCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *externalInterpretedCleanupFailure) OriginalCause() error { return f.cause }
func (f *externalInterpretedCleanupFailure) CleanupCause() error  { return f.cleanup }
func (f *externalInterpretedCleanupFailure) Cleanup() error {
	f.cleanup = f.attempt.close()
	return f.cleanup
}

func finishExternalInterpretedAttempt(attempt *externalInterpretedAttempt, cause error) error {
	if cause != nil {
		var nested memory.ConstructionFailure
		if errors.As(cause, &nested) {
			attempt.nested = nested
		}
	}
	cleanup := attempt.close()
	if cleanup == nil {
		return cause
	}
	if cause == nil {
		cause = cleanup
		return &externalInterpretedCleanupFailure{attempt: attempt, cause: cause, cleanup: cleanup, shown: cause, message: cause.Error()}
	}
	shown := errors.Join(cause, cleanup)
	return &externalInterpretedCleanupFailure{attempt: attempt, cause: cause, cleanup: cleanup, shown: shown, message: shown.Error()}
}

var _ memory.ConstructionFailure = (*externalInterpretedCleanupFailure)(nil)

func validateExternalInterpretedColumns(manager *ast.ASTManager, conditions []ast.ASTHandle, columns map[uint32]uint32) error {
	if manager == nil || len(conditions) == 0 {
		return ErrCompilerUnsupported
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return err
	}
	for _, condition := range conditions {
		kind, cause := manager.GetKind(condition)
		if cause != nil {
			return cause
		}
		sort, cause := manager.GetSort(condition)
		if cause != nil {
			return cause
		}
		if kind != ast.ASTApp || sort != boolean {
			return ErrCompilerUnsupported
		}
	}
	free, err := astrewriter.NewExprFreeVars(manager)
	if err != nil {
		return err
	}
	for _, condition := range conditions {
		if err = free.Accumulate(condition); err != nil {
			return err
		}
	}
	for variable := uint32(0); variable < free.Size(); variable++ {
		sort, cause := free.Get(variable)
		if cause != nil {
			return cause
		}
		if !sort.IsNull() {
			if _, found := columns[variable]; !found {
				return ErrCompilerUnsupported
			}
		}
	}
	return nil
}

// filterExternalRuleInterpreted substitutes rule variables with the concrete
// joined-relation columns and applies one source conjunction filter in place.
func filterExternalRuleInterpreted(relation *ExternalRelation, conditions []ast.ASTHandle, columns map[uint32]uint32) (resultErr error) {
	if relation == nil || relation.open() != nil || len(conditions) == 0 {
		return ErrCompilerUnsupported
	}
	m := relation.ast
	attempt := &externalInterpretedAttempt{manager: m}
	defer func() { resultErr = finishExternalInterpretedAttempt(attempt, resultErr) }()
	if err := validateExternalInterpretedColumns(m, conditions, columns); err != nil {
		return err
	}
	sorts, err := relation.Sorts()
	if err != nil {
		return err
	}
	free, err := astrewriter.NewExprFreeVars(m)
	if err != nil {
		return err
	}
	for _, condition := range conditions {
		if err = free.Accumulate(condition); err != nil {
			return err
		}
	}
	bindings := make([]ast.ASTHandle, free.Size())
	for variable := uint32(0); variable < free.Size(); variable++ {
		sort, cause := free.Get(variable)
		if cause != nil {
			return cause
		}
		if sort.IsNull() {
			continue
		}
		column, found := columns[variable]
		if !found || uint64(column) >= uint64(len(sorts)) || sorts[column] != sort {
			return ErrCompilerUnsupported
		}
		binding, cause := m.MkVar(column, sort)
		if cause != nil {
			return cause
		}
		root, cause := m.NewRef(binding)
		if cause != nil {
			return cause
		}
		attempt.roots = append(attempt.roots, root)
		bindings[variable] = binding
	}
	condition := conditions[0]
	if len(conditions) != 1 {
		condition, err = m.MkAndArgs(conditions)
		if err != nil {
			return err
		}
		root, cause := m.NewRef(condition)
		if cause != nil {
			return cause
		}
		attempt.roots = append(attempt.roots, root)
	}
	substitution, err := astrewriter.NewVarSubstitution(m, false)
	attempt.substitution = substitution
	if err != nil {
		return err
	}
	bound, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	attempt.roots = append(attempt.roots, bound)
	if err = substitution.Substitute(condition, bindings, bound); err != nil {
		return err
	}
	boundHandle, err := m.RefGet(bound)
	if err != nil {
		return err
	}
	return relation.FilterInterpreted(boundHandle)
}

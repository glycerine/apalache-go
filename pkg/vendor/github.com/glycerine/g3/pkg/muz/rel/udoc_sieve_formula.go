package rel

// Source: sieve_relation::to_formula maps each inner variable back to its
// original column and sort. Ignored columns introduce no formula constraints.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
)

type udocSieveFormulaOwner struct {
	values       typedUDocFormulaOwner
	substitution *rewriter.VarSubstitution
}

func (o *udocSieveFormulaOwner) closeTemps() error {
	if o.values.nested != nil {
		if err := o.values.nested.Cleanup(); err != nil {
			return err
		}
		o.values.nested = nil
	}
	if o.substitution != nil {
		if err := o.substitution.Close(); err != nil {
			return err
		}
		o.substitution = nil
	}
	return o.values.closeTemps()
}

func (o *udocSieveFormulaOwner) close() error {
	if err := o.closeTemps(); err != nil {
		return err
	}
	return o.values.close()
}

type udocSieveFormulaFailure struct {
	owner                       *udocSieveFormulaOwner
	original, cleanup, reported error
	text                        string
}

func (f *udocSieveFormulaFailure) Error() string        { return f.text }
func (f *udocSieveFormulaFailure) Unwrap() error        { return f.reported }
func (f *udocSieveFormulaFailure) OriginalCause() error { return f.original }
func (f *udocSieveFormulaFailure) CleanupCause() error  { return f.cleanup }
func (f *udocSieveFormulaFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.values.manager.NumeralService().RuntimeOwner()
}
func (f *udocSieveFormulaFailure) Cleanup() error { f.cleanup = f.owner.close(); return f.cleanup }

func finishUDocSieveFormula(owner *udocSieveFormulaOwner, cause error) error {
	if cause != nil {
		errors.As(cause, &owner.values.nested)
	}
	cleanup := owner.closeTemps()
	if cleanup == nil && cause != nil {
		cleanup = owner.close()
	}
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &udocSieveFormulaFailure{owner: owner, original: original, cleanup: cleanup, reported: reported, text: reported.Error()}
}

func (r *UDocSieve) ToFormula() (result ast.ASTRef, resultErr error) {
	if err := r.open(); err != nil {
		return result, err
	}
	m := r.manager
	owner := &udocSieveFormulaOwner{values: typedUDocFormulaOwner{manager: m}}
	defer func() {
		resultErr = finishUDocSieveFormula(owner, resultErr)
		if resultErr == nil {
			result, owner.values.result = owner.values.result, ast.ASTRef{}
		} else {
			result = ast.ASTRef{}
		}
	}()
	bindings := make([]ast.ASTHandle, len(r.innerToSig))
	for inner, outer := range r.innerToSig {
		ref, err := m.NewRef(ast.ASTHandle{})
		if ref != (ast.ASTRef{}) {
			owner.values.roots = append(owner.values.roots, ref)
		}
		if err != nil {
			return result, err
		}
		variable, cause := m.MkVar(outer, r.sorts[outer])
		if err = errors.Join(cause, m.SetRef(ref, variable)); err != nil {
			return result, err
		}
		bindings[inner] = variable
	}
	inner, err := r.inner.ToFormula()
	if inner != (ast.ASTRef{}) {
		owner.values.roots = append(owner.values.roots, inner)
	}
	if err != nil {
		return result, err
	}
	body, err := m.RefGet(inner)
	if err != nil {
		return result, err
	}
	owner.substitution, err = rewriter.NewVarSubstitution(m, false)
	if err != nil {
		return result, err
	}
	owner.values.result, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	return result, owner.substitution.Substitute(body, bindings, owner.values.result)
}

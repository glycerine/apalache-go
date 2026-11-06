package base

// Source display_smt2 scopes expr_refs, var_subst, and format/stream temporaries.
// The Go owner preserves those concrete scopes when native cleanup must retry.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type contextSMT2Owner struct {
	manager      *ast.ASTManager
	refs         []ast.ASTRef
	formats      *ast.ASTManager
	format       ast.ASTRef
	stream       *utext.NumericTextStream
	substitution *rewriter.VarSubstitution
	nested       memory.ConstructionFailure
}

func (o *contextSMT2Owner) reserve() (ast.ASTRef, error) {
	ref, err := o.manager.NewRef(ast.ASTHandle{})
	if ref != (ast.ASTRef{}) {
		o.refs = append(o.refs, ref)
	}
	return ref, err
}

func (o *contextSMT2Owner) close() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	if o.stream != nil {
		if err := o.stream.Close(); err != nil {
			return err
		}
		o.stream = nil
	}
	if o.format != (ast.ASTRef{}) {
		if err := o.formats.DelRef(&o.format); err != nil {
			return err
		}
	}
	if o.substitution != nil {
		if err := o.substitution.Close(); err != nil {
			return err
		}
		o.substitution = nil
	}
	for len(o.refs) != 0 {
		last := len(o.refs) - 1
		if o.refs[last] != (ast.ASTRef{}) {
			err := o.manager.DelRef(&o.refs[last])
			if o.refs[last] == (ast.ASTRef{}) {
				o.refs = o.refs[:last]
			}
			if err != nil {
				return err
			}
		} else {
			o.refs = o.refs[:last]
		}
	}
	return nil
}

type contextSMT2CleanupFailure struct {
	owner                       *contextSMT2Owner
	runtime                     *memory.Runtime
	original, cleanup, reported error
	text                        string
}

func (f *contextSMT2CleanupFailure) Error() string                 { return f.text }
func (f *contextSMT2CleanupFailure) Unwrap() error                 { return f.reported }
func (f *contextSMT2CleanupFailure) OriginalCause() error          { return f.original }
func (f *contextSMT2CleanupFailure) CleanupCause() error           { return f.cleanup }
func (f *contextSMT2CleanupFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *contextSMT2CleanupFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}

func (o *contextSMT2Owner) finish(cause error) error {
	errors.As(cause, &o.nested)
	if cleanup := o.close(); cleanup != nil {
		reported := errors.Join(cause, cleanup)
		return &contextSMT2CleanupFailure{
			owner: o, runtime: o.manager.NumeralService().RuntimeOwner(),
			original: cause, cleanup: cleanup, reported: reported, text: reported.Error(),
		}
	}
	return cause
}

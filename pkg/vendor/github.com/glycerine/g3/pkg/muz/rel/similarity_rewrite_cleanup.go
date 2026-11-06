package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
)

// similarityRewriteOwner holds the rewritten rule until temporary AST roots
// have retired. An interrupted release must not publish or lose that rule.
type similarityRewriteOwner struct {
	manager *ast.ASTManager
	refs    []ast.ASTRef
	rule    *base.Rule
	nested  memory.ConstructionFailure
}

func (o *similarityRewriteOwner) closeRefs() error {
	for len(o.refs) != 0 {
		last := len(o.refs) - 1
		if o.refs[last] == (ast.ASTRef{}) {
			o.refs = o.refs[:last]
			continue
		}
		err := o.manager.DelRef(&o.refs[last])
		if o.refs[last] == (ast.ASTRef{}) {
			o.refs = o.refs[:last]
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *similarityRewriteOwner) close() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	if o.rule != nil {
		if err := o.rule.Close(); err != nil {
			return err
		}
		o.rule = nil
	}
	return o.closeRefs()
}

type similarityRewriteConstructionFailure struct {
	owner             *similarityRewriteOwner
	runtime           *memory.Runtime
	original, cleanup error
	reported          error
	text              string
}

func (f *similarityRewriteConstructionFailure) Error() string        { return f.text }
func (f *similarityRewriteConstructionFailure) Unwrap() error        { return f.reported }
func (f *similarityRewriteConstructionFailure) OriginalCause() error { return f.original }
func (f *similarityRewriteConstructionFailure) CleanupCause() error  { return f.cleanup }
func (f *similarityRewriteConstructionFailure) RuntimeOwner() *memory.Runtime {
	return f.runtime
}
func (f *similarityRewriteConstructionFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}

func finishSimilarityRewriteConstruction(cause error, owner *similarityRewriteOwner) error {
	runtime := owner.manager.NumeralService().RuntimeOwner()
	if cause != nil {
		errors.As(cause, &owner.nested)
	}
	cleanup := owner.close()
	if cleanup == nil {
		return cause
	}
	if cause == nil {
		cause = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &similarityRewriteConstructionFailure{owner: owner, runtime: runtime, original: cause, cleanup: cleanup, reported: reported, text: reported.Error()}
}

var _ memory.ConstructionFailure = (*similarityRewriteConstructionFailure)(nil)

package pdr

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
)

// symbolMuxTraversalConstructionFailure keeps a partially initialized rewrite
// traversal reachable when its constructor cleanup refuses.
type symbolMuxTraversalConstructionFailure struct {
	manager                    *ast.ASTManager
	traversal                  *rewriter.Traversal
	original, cleanup, exposed error
	text                       string
}

var _ memory.ConstructionFailure = (*symbolMuxTraversalConstructionFailure)(nil)

func (f *symbolMuxTraversalConstructionFailure) Error() string        { return f.text }
func (f *symbolMuxTraversalConstructionFailure) Unwrap() error        { return f.exposed }
func (f *symbolMuxTraversalConstructionFailure) OriginalCause() error { return f.original }
func (f *symbolMuxTraversalConstructionFailure) CleanupCause() error  { return f.cleanup }
func (f *symbolMuxTraversalConstructionFailure) RuntimeOwner() *memory.Runtime {
	return f.manager.NumeralService().RuntimeOwner()
}
func (f *symbolMuxTraversalConstructionFailure) Cleanup() error {
	if f.traversal == nil {
		f.cleanup = nil
		return nil
	}
	f.cleanup = f.traversal.Close()
	if f.cleanup == nil {
		f.traversal = nil
	}
	return f.cleanup
}

func finishSymbolMuxTraversalConstruction(manager *ast.ASTManager, traversal *rewriter.Traversal, cause error) error {
	if traversal == nil {
		return cause
	}
	cleanup := traversal.Close()
	if cleanup == nil {
		return cause
	}
	exposed := errors.Join(cause, cleanup)
	return &symbolMuxTraversalConstructionFailure{
		manager: manager, traversal: traversal, original: cause,
		cleanup: cleanup, exposed: exposed, text: exposed.Error(),
	}
}

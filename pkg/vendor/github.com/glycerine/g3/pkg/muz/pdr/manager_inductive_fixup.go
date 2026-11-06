package pdr

// Source: inductive_property::fixup_clause and fixup_clauses in
// src/muz/pdr/pdr_manager.cpp:56-73.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
)

type inductiveFixupOwner struct {
	context    *Context
	manager    *ast.ASTManager
	runtime    *memory.Runtime
	rewriter   *rewriter.BoolRewriter
	references []ast.ASTRef
	nested     memory.ConstructionFailure
}

func (o *inductiveFixupOwner) close() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	if o.rewriter != nil {
		if err := o.rewriter.Close(); err != nil {
			return err
		}
		o.rewriter = nil
	}
	for len(o.references) != 0 {
		i := len(o.references) - 1
		err := o.manager.DelRef(&o.references[i])
		if o.references[i] == (ast.ASTRef{}) {
			o.references = o.references[:i]
		}
		if err != nil {
			return err
		}
	}
	return nil
}

type inductiveFixupFailure struct {
	owner          *inductiveFixupOwner
	cause, cleanup error
}

func (f *inductiveFixupFailure) Error() string        { return errors.Join(f.cause, f.cleanup).Error() }
func (f *inductiveFixupFailure) Unwrap() error        { return errors.Join(f.cause, f.cleanup) }
func (f *inductiveFixupFailure) OriginalCause() error { return f.cause }
func (f *inductiveFixupFailure) CleanupCause() error  { return f.cleanup }
func (f *inductiveFixupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.runtime
}
func (f *inductiveFixupFailure) Cleanup() error { f.cleanup = f.owner.close(); return f.cleanup }

func (o *inductiveFixupOwner) pin(h ast.ASTHandle) error {
	ref, err := o.manager.NewRef(h)
	if err == nil {
		o.references = append(o.references, ref)
	}
	return err
}

func (o *inductiveFixupOwner) gate(op int32, args []ast.ASTHandle) (ast.ASTRef, error) {
	m := o.manager
	out, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return out, err
	}
	o.references = append(o.references, out)
	config := o.context.owner.contexts.config
	if config.Globals == nil {
		err = rewriter.SimplifyBooleanGateDefault(m, op, args, out)
	} else {
		o.rewriter, err = rewriter.NewBoolRewriter(m, config.Params, config.Globals, nil)
		if err == nil {
			if op == ast.ASTOpOr {
				err = o.rewriter.MkOr(args, out)
			} else {
				err = o.rewriter.MkAnd(args, out)
			}
		}
		if err == nil {
			err = o.rewriter.Close()
			if err == nil {
				o.rewriter = nil
			}
		}
	}
	return out, err
}

// fixupInductiveClausesRef returns the caller-owned source-normalized body.
func (c *Context) fixupInductiveClausesRef(formula ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	o := &inductiveFixupOwner{context: c, manager: c.manager, runtime: c.manager.NumeralService().RuntimeOwner()}
	defer func() {
		if resultErr != nil {
			errors.As(resultErr, &o.nested)
		}
		cleanup := o.close()
		if cleanup == nil {
			return
		}
		if result != (ast.ASTRef{}) {
			o.references = append(o.references, result)
			result = ast.ASTRef{}
		}
		cause := resultErr
		if cause == nil {
			cause = fmt.Errorf("pdr: close inductive clause fixup: %w", cleanup)
		}
		resultErr = &inductiveFixupFailure{owner: o, cause: cause, cleanup: cleanup}
	}()
	conjuncts, err := c.manager.FlattenAnd([]ast.ASTHandle{formula})
	if err != nil {
		return result, err
	}
	for _, conj := range conjuncts {
		if err = o.pin(conj); err != nil {
			return result, err
		}
	}
	clauses := make([]ast.ASTHandle, 0, len(conjuncts))
	for _, conj := range conjuncts {
		disjuncts, x := c.manager.FlattenOr([]ast.ASTHandle{conj})
		if x != nil {
			return result, x
		}
		for _, disj := range disjuncts {
			if x = o.pin(disj); x != nil {
				return result, x
			}
		}
		clauseRef, x := o.gate(ast.ASTOpOr, disjuncts)
		if x != nil {
			return result, x
		}
		clause, x := c.manager.RefGet(clauseRef)
		if x != nil {
			return result, x
		}
		clauses = append(clauses, clause)
	}
	out, err := o.gate(ast.ASTOpAnd, clauses)
	if err != nil {
		return result, err
	}
	// Move the last owned ref to the caller before releasing lexical temporaries.
	o.references = o.references[:len(o.references)-1]
	return out, nil
}

var _ memory.ConstructionFailure = (*inductiveFixupFailure)(nil)

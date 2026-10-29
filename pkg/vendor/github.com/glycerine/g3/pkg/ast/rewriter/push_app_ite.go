package rewriter

// Source: src/ast/rewriter/push_app_ite.cpp:24-91;
// src/ast/rewriter/push_app_ite.h:24-70.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

// PushAppITEConfiguration lifts term-valued if-then-else arguments through an
// application. In conservative mode an application with multiple such
// arguments is left unchanged. NongroundOnly selects the source variation
// used by the SMT preprocessor.
type PushAppITEConfiguration struct {
	DefaultConfiguration
	manager       *ast.ASTManager
	conservative  bool
	nongroundOnly bool
}

func newPushAppITEConfiguration(m *ast.ASTManager, conservative, nongroundOnly bool) (*PushAppITEConfiguration, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	c := &PushAppITEConfiguration{manager: m, conservative: conservative, nongroundOnly: nongroundOnly}
	c.defaults.manager = m
	return c, nil
}

func NewPushAppITEConfiguration(m *ast.ASTManager, conservative bool) (*PushAppITEConfiguration, error) {
	return newPushAppITEConfiguration(m, conservative, false)
}

func NewNGPushAppITEConfiguration(m *ast.ASTManager, conservative bool) (*PushAppITEConfiguration, error) {
	return newPushAppITEConfiguration(m, conservative, true)
}

func (c *PushAppITEConfiguration) RewritePatterns() bool { return false }

func (c *PushAppITEConfiguration) isITE(h ast.ASTHandle) (bool, error) {
	nodeKind, e := c.manager.GetKind(h)
	if e != nil {
		return false, e
	}
	if nodeKind != ast.ASTApp && nodeKind != ast.ASTFuncDecl {
		return false, nil
	}
	family, e := c.manager.GetFamilyID(h)
	if e != nil || family != 0 {
		return false, e
	}
	kind, e := c.manager.GetDeclKind(h)
	return kind == ast.ASTOpIte, e
}

func (c *PushAppITEConfiguration) isTarget(decl ast.ASTHandle, args ast.AppArguments) (bool, error) {
	if yes, e := c.isITE(decl); e != nil || yes {
		return false, e
	}
	boolSort, e := c.manager.MkBoolSort()
	if e != nil {
		return false, e
	}
	found := false
	for i := uint32(0); i < args.Count(); i++ {
		h, e := args.At(i)
		if e != nil {
			return false, e
		}
		yes, e := c.isITE(h)
		if e != nil {
			return false, e
		}
		if !yes {
			continue
		}
		sort, e := c.manager.GetSort(h)
		if e != nil {
			return false, e
		}
		if sort == boolSort {
			continue
		}
		if found && c.conservative {
			return false, nil
		}
		found = true
	}
	if !found || !c.nongroundOnly {
		return found, nil
	}
	for i := uint32(0); i < args.Count(); i++ {
		h, e := args.At(i)
		if e != nil {
			return false, e
		}
		ground, e := c.manager.IsGround(h)
		if e != nil {
			return false, e
		}
		if !ground {
			return true, nil
		}
	}
	return false, nil
}

func (c *PushAppITEConfiguration) ReduceApp(decl ast.ASTHandle, arguments ast.AppArguments, out, proof ast.ASTRef) (RewriteStatus, error) {
	if c == nil || c.manager == nil {
		return RewriteFailed, ast.ErrASTHandle
	}
	target, e := c.isTarget(decl, arguments)
	if e != nil || !target {
		return RewriteFailed, e
	}
	args := make([]ast.ASTHandle, arguments.Count())
	iteIndex := -1
	for i := range args {
		args[i], e = arguments.At(uint32(i))
		if e != nil {
			return RewriteFailed, e
		}
		if iteIndex < 0 {
			yes, x := c.isITE(args[i])
			if x != nil {
				return RewriteFailed, x
			}
			if yes {
				iteIndex = i
			}
		}
	}
	if iteIndex < 0 {
		return RewriteFailed, nil
	}
	condition, e := c.manager.GetArg(args[iteIndex], 0)
	if e != nil {
		return RewriteFailed, e
	}
	thenValue, e := c.manager.GetArg(args[iteIndex], 1)
	if e != nil {
		return RewriteFailed, e
	}
	elseValue, e := c.manager.GetArg(args[iteIndex], 2)
	if e != nil {
		return RewriteFailed, e
	}
	original, e := c.manager.MkApp(decl, args)
	if e != nil {
		return RewriteFailed, e
	}
	args[iteIndex] = thenValue
	thenApp, e := c.manager.MkApp(decl, args)
	if e != nil {
		return RewriteFailed, e
	}
	args[iteIndex] = elseValue
	elseApp, e := c.manager.MkApp(decl, args)
	if e != nil {
		return RewriteFailed, e
	}
	result, e := c.manager.MkIte(condition, thenApp, elseApp)
	if e != nil {
		return RewriteFailed, e
	}
	if e = c.manager.SetRef(out, result); e != nil {
		return RewriteFailed, e
	}
	proofs, e := c.manager.ProofsEnabled()
	if e != nil {
		return RewriteFailed, e
	}
	if proofs {
		p, x := c.manager.MkProofRewrite(original, result)
		if x != nil {
			return RewriteFailed, x
		}
		if e = c.manager.SetRef(proof, p); e != nil {
			return RewriteFailed, e
		}
	}
	return Rewrite2, nil
}

// PushAppITE owns the source rewriter wrapper. Its traversal preserves source
// caching, bounded revisits, quantifier handling, and the refusal to rewrite
// patterns. The published proof establishes the same root rewrite relation.
type PushAppITE struct {
	self      *PushAppITE
	manager   *ast.ASTManager
	config    *PushAppITEConfiguration
	traversal *Traversal
	header    uint64
	closed    bool
}

func newPushAppITE(m *ast.ASTManager, conservative, nongroundOnly bool) (*PushAppITE, error) {
	c, e := newPushAppITEConfiguration(m, conservative, nongroundOnly)
	if e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(PushAppITE{}).Size() + reflect.TypeOf(PushAppITEConfiguration{}).Size())
	if e = m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	r := &PushAppITE{manager: m, config: c, header: bytes}
	r.self = r
	proofs, e := m.ProofsEnabled()
	if e != nil {
		return r, e
	}
	if proofs {
		r.traversal, e = NewProofTraversal(m, c)
	} else {
		r.traversal, e = NewTraversal(m, c)
	}
	return r, e
}

func NewPushAppITE(m *ast.ASTManager, conservative bool) (*PushAppITE, error) {
	return newPushAppITE(m, conservative, false)
}

func NewNGPushAppITE(m *ast.ASTManager, conservative bool) (*PushAppITE, error) {
	return newPushAppITE(m, conservative, true)
}

func (r *PushAppITE) open() error {
	if r == nil || r.self != r || r.closed || r.manager == nil || r.config == nil || r.traversal == nil {
		return ast.ErrASTHandle
	}
	_, e := r.manager.ProofsEnabled()
	return e
}

func (r *PushAppITE) Rewrite(root ast.ASTHandle, out, proof ast.ASTRef) error {
	if e := r.open(); e != nil {
		return e
	}
	proofs, e := r.manager.ProofsEnabled()
	if e != nil {
		return e
	}
	if proofs {
		return r.traversal.RewriteWithProof(root, out, proof)
	}
	if e = r.manager.ResetRef(proof); e != nil {
		return e
	}
	return r.traversal.Rewrite(root, out)
}

func (r *PushAppITE) Reset() error {
	if e := r.open(); e != nil {
		return e
	}
	return r.traversal.Reset()
}

func (r *PushAppITE) Close() (result error) {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	if r.traversal != nil {
		result = errors.Join(result, r.traversal.Close())
	}
	r.manager.Budget().Release(r.header)
	r.header = 0
	r.closed = true
	return result
}

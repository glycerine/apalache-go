package rewriter

// Source: src/ast/rewriter/bv_trailing.cpp:70-82,121-129,142-171,183-208.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

// Each call names its reached structural frame and numerical child. A parent
// loop reuses this call site only after a successful, fully retired return.
type bvTrailingCall struct {
	child   *bvTrailingFrame
	numeral ast.ASTBitVectorNumeralInvocation
}

func (c *bvTrailingCall) sourceReady(m *ast.ASTManager) error {
	if e := m.BitVectorNumeralInvocationSourceReady(&c.numeral); e != nil {
		return e
	}
	if f := c.child; f != nil {
		if f.sourceCause != nil {
			return f.sourceCause
		}
		if !f.sourceDone {
			return ast.ErrASTPending
		}
	}
	return nil
}
func (c *bvTrailingCall) cleanup(m *ast.ASTManager) error {
	if f := c.child; f != nil && !f.retired {
		if e := f.destroy(true); e != nil {
			return e
		}
	}
	return m.CleanupBitVectorNumeralInvocation(&c.numeral)
}

const (
	bvTrailingAddFrame = iota
	bvTrailingMulFrame
	bvTrailingConcatFrame
	bvTrailingEqFrame
)

// Frame storage and actual reference/numeral owners remain charged. Native
// owning argument slices preserve consumed-DEC cleanup without vector storage.
type bvTrailingFrame struct {
	owner                   *BVTrailing
	next                    *bvTrailingFrame
	header                  uint64
	mode                    int
	refs                    [2]ast.ASTRef
	args                    []ast.ASTHandle
	call                    bvTrailingCall
	scope                   ast.ASTBitVectorNumeralScope
	scopePlaced, scopeEnded bool
	sourceDone, retired     bool
	bodyCause, sourceCause  error
}

func (p *BVTrailing) beginFrame(mode int, call *bvTrailingCall) (*bvTrailingFrame, error) {
	n := uint64(reflect.TypeOf(bvTrailingFrame{}).Size())
	if e := p.manager.Budget().Reserve(n); e != nil {
		return nil, e
	}
	f := &bvTrailingFrame{owner: p, next: p.frames, header: n, mode: mode}
	p.frames = f
	call.child = f
	return f, nil
}
func (f *bvTrailingFrame) newRef(i int) error {
	r, e := f.owner.manager.NewRef(ast.ASTHandle{})
	if e == nil {
		f.refs[i] = r
	}
	return e
}
func (f *bvTrailingFrame) pushArgValue(h ast.ASTHandle) error {
	return appendRewriterRoot(f.owner.manager, &f.args, h)
}
func (f *bvTrailingFrame) makeApp(original ast.ASTHandle, kind int32) (ast.ASTHandle, error) {
	m := f.owner.manager
	fid, e := m.GetFamilyID(original)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkFamilyApp(fid, kind, nil, f.args)
}
func (f *bvTrailingFrame) delRef(i int) error {
	if f.refs[i] == (ast.ASTRef{}) {
		return nil
	}
	return f.owner.manager.DelRef(&f.refs[i])
}
func (f *bvTrailingFrame) releaseArgs() error {
	if e := shrinkRewriterRoots(f.owner.manager, &f.args, 0); e != nil {
		return e
	}
	f.args = nil
	return nil
}
func (f *bvTrailingFrame) destroy(own bool) error {
	if f.retired || !own && f.sourceDone {
		return nil
	}
	if !own && f.sourceCause != nil {
		return f.sourceCause
	}
	m := f.owner.manager
	if own {
		if e := f.call.cleanup(m); e != nil {
			return e
		}
		if f.scopePlaced {
			if e := m.CleanupBitVectorNumeralScope(&f.scope); e != nil {
				return e
			}
		}
	} else {
		if e := f.call.sourceReady(m); e != nil {
			return e
		}
		if f.scopePlaced {
			if e := m.BitVectorNumeralScopeSourceReady(&f.scope); e != nil {
				return e
			}
		}
	}
	switch f.mode {
	case bvTrailingAddFrame:
		if e := f.delRef(0); e != nil {
			return e
		}
		if e := f.releaseArgs(); e != nil {
			return e
		}
	case bvTrailingMulFrame, bvTrailingConcatFrame:
		if e := f.releaseArgs(); e != nil {
			return e
		}
		if e := f.delRef(0); e != nil {
			return e
		}
	case bvTrailingEqFrame:
		if e := f.delRef(1); e != nil {
			return e
		}
		if e := f.delRef(0); e != nil {
			return e
		}
	default:
		return ast.ErrASTDomain
	}
	if !own {
		f.sourceDone = true
	}
	return nil
}
func (f *bvTrailingFrame) finish(body *error) {
	f.bodyCause = *body
	// A stopped exact younger child blocks the source footer, including
	// the multiplication caller's older c_val/vector/tmp sequence.
	e := f.call.sourceReady(f.owner.manager)
	if e == nil && f.scopePlaced && !f.scopeEnded {
		f.scopeEnded = true
		*body = f.owner.manager.EndBitVectorNumeralScope(&f.scope, *body)
	}
	if e == nil {
		e = f.destroy(false)
	}
	f.sourceCause = e
	*body = errors.Join(*body, e)
	if *body == nil && f.sourceDone {
		*body = f.destroy(true)
		if *body == nil {
			p := f.owner
			if p.frames != f {
				*body = ast.ErrASTPending
				return
			}
			p.frames = f.next
			p.manager.Budget().Release(f.header)
			f.header = 0
			f.retired = true
		}
	}
}

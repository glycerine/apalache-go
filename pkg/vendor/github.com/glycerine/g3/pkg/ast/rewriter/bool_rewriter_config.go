package rewriter

// Source: src/ast/rewriter/bool_rewriter.cpp:23-87;
// src/ast/rewriter/bool_rewriter.h:85-107,113-126,132-156;
// src/ast/rewriter/bool_rewriter_params.hpp:6-30;
// src/util/ref_buffer.h:35-68.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// The standalone cold constructor charges its complete header once. A failed
// reached parameter constructor returns its owner for explicit Close, following
// the existing generated-helper lifetime engine. AST/parameter services borrow.

func newBoolRewriter(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef) (*boolRewriter, error) {
	if m == nil || c == nil || g == nil || c.SymbolsField != m.Symbols || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(boolRewriter{}).Size())
	if e := m.BudgetField.Reserve(n); e != nil {
		return nil, e
	}
	rw := &boolRewriter{header: n}
	return rw, rw.init(m, c, g, p)
}
func (rw *boolRewriter) init(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef) error {
	rw.manager = m
	rw.self = rw
	rw.params = uparams.ModuleParamsOwner{Budget: g.Config.Budget}
	rw.params.Self = &rw.params
	if e := rw.params.Bind(c, g); e != nil {
		return e
	}
	e := rw.updateParams(p)
	if e == nil {
		rw.ready = true
	}
	return e
}
func (rw *boolRewriter) configuredOpen() error {
	if rw == nil || rw.self != rw || rw.closed {
		return uparams.ErrParamsOwner
	}
	if !rw.ready {
		return uparams.ErrParamsPrecondition
	}
	_, e := rw.manager.ProofsEnabled()
	return e
}
func (rw *boolRewriter) UpdtParams(p *uparams.ParamsRef) error {
	if e := rw.configuredOpen(); e != nil {
		return e
	}
	return rw.updateParams(p)
}
func (rw *boolRewriter) updateParams(local *uparams.ParamsRef) (err error) {
	o := &rw.params
	if o.Self != o || o.Running || o.DestructorCause != nil {
		return uparams.ErrParamsPrecondition
	}
	if local == nil {
		empty, e := o.Context.NewValue()
		if e != nil {
			return e
		}
		local = &empty
		defer func() { err = errors.Join(err, o.Context.Del(&empty)) }()
	}
	if e := o.Context.Check(local); e != nil {
		return e
	}
	o.Running = true
	defer func() { o.Running = false }()
	f, e := o.BeginHelper(local, "rewriter")
	if f != nil {
		defer func() { err = o.FinishHelper(f, err) }()
	}
	if e != nil {
		return e
	}
	b, e := o.FrameBool(f, "flat", true)
	if e != nil {
		return e
	}
	rw.flat = b
	b, e = o.FrameBool(f, "elim_and", false)
	if e != nil {
		return e
	}
	rw.elimAnd = b
	b, e = o.FrameBool(f, "local_ctx", false)
	if e != nil {
		return e
	}
	rw.localContext = b
	u, e := o.FrameUInt(f, "local_ctx_limit", ^uint32(0))
	if e != nil {
		return e
	}
	rw.localContextLimit = u
	b, e = o.FrameBool(f, "blast_distinct", false)
	if e != nil {
		return e
	}
	rw.blastDistinct = b
	u, e = o.FrameUInt(f, "blast_distinct_threshold", ^uint32(0))
	if e != nil {
		return e
	}
	rw.blastDistinctThreshold = u
	b, e = o.FrameBool(f, "ite_extra_rules", false)
	if e != nil {
		return e
	}
	rw.iteExtraRules = b
	return nil
}
func (rw *boolRewriter) ResetLocalContextCost() { rw.localContextCost = 0 }
func (rw *boolRewriter) Close() error {
	if rw == nil || rw.self != rw {
		return uparams.ErrParamsOwner
	}
	if rw.closed {
		return nil
	}
	if rw.andDepth != 0 {
		return ast.ErrASTPending
	}
	if e := rw.closeLocalContext(); e != nil {
		return e
	}
	if e := rw.closeAndArguments(0); e != nil {
		return e
	}
	if e := rw.params.CloseHelpers(); e != nil {
		return e
	}
	rw.manager.BudgetField.Release(rw.header)
	rw.header = 0
	rw.closed = true
	return nil
}

func (rw *boolRewriter) mkAndCore(args []ast.ASTHandle, out ast.ASTRef) (brStatus, error) {
	if rw.elimAnd {
		return brDone, rw.mkAndAsOr(args, out)
	}
	if rw.flat {
		return rw.mkFlatAndCore(args, out)
	}
	return rw.mkNFlatAndCore(args, out)
}
func (rw *boolRewriter) mkOrCore(args []ast.ASTHandle, out ast.ASTRef) (brStatus, error) {
	if rw.flat {
		return rw.mkFlatOrCore(args, out)
	}
	return rw.mkNFlatOrCore(args, out)
}

func (rw *boolRewriter) mkAndAsOr(args []ast.ASTHandle, out ast.ASTRef) (err error) {
	return rw.mkAndAsOrArguments(astSliceArguments(args), out)
}
func (rw *boolRewriter) mkAndAsOrArguments(args ast.AppArguments, out ast.ASTRef) (err error) {
	m := rw.manager
	if rw.andDepth == 0 && len(rw.andArgs) != 0 {
		return ast.ErrASTPending
	}
	depth := len(rw.andArgs)
	rw.andArgs = append(rw.andArgs, nil)
	rw.andDepth++
	defer func() {
		err = errors.Join(err, rw.closeAndArguments(depth))
		rw.andDepth--
	}()
	for i := uint32(0); i < args.Count(); i++ {
		arg, e := args.At(i)
		if e != nil {
			return e
		}
		e = func() (err error) {
			tmp, e := m.NewRef(ast.ASTHandle{})
			if e != nil {
				return e
			}
			defer func() { err = errors.Join(err, m.DelRef(&tmp)) }()
			if e = rw.mkNot(arg, tmp); e != nil {
				return e
			}
			h, e := m.RefGet(tmp)
			if e != nil {
				return e
			}
			return appendRewriterRoot(m, &rw.andArgs[depth], h)
		}()
		if e != nil {
			return e
		}
	}
	tmp, e := m.NewRef(ast.ASTHandle{})
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, m.DelRef(&tmp)) }()
	if e = rw.mkOr(rw.andArgs[depth], tmp); e != nil {
		return e
	}
	h, e := m.RefGet(tmp)
	if e != nil {
		return e
	}
	return rw.mkNot(h, out)
}

func (rw *boolRewriter) mkApp(decl ast.ASTHandle, args []ast.ASTHandle, out ast.ASTRef) error {
	status, e := rw.mkAppCore(decl, args, out)
	if e != nil || status != brFailed {
		return e
	}
	h, e := rw.manager.MkApp(decl, args)
	if e != nil {
		return e
	}
	return rw.manager.SetRef(out, h)
}
func (rw *boolRewriter) GetParamDescrs(d *uparams.ParamDescrs) error {
	return catalogCollectBoolRewriterParams(d)
}

// Each recursive AND-as-OR invocation owns a native row until its lexical
// cleanup finishes. Stopped younger rows remain reachable before older rows.
func (rw *boolRewriter) closeAndArguments(depth int) error {
	for len(rw.andArgs) > depth {
		last := len(rw.andArgs) - 1
		if e := shrinkRewriterRoots(rw.manager, &rw.andArgs[last], 0); e != nil {
			return e
		}
		rw.andArgs[last] = nil
		rw.andArgs = rw.andArgs[:last]
	}
	if depth == 0 {
		rw.andArgs = nil
	}
	return nil
}

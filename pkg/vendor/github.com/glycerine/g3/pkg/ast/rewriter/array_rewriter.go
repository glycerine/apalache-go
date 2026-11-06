package rewriter

// Source: src/ast/rewriter/array_rewriter.cpp:25-38,70-224;
// src/ast/rewriter/array_rewriter.h:34-64;
// src/ast/rewriter/array_rewriter_params.pyg:1-6.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"

	uparams "github.com/glycerine/g3/pkg/util/params"
	"reflect"
)

// ArrayRewriter is the original core reducer, borrowing its AST services.
// Traversal is supplied by the caller, as for the source theory reducers.
type ArrayRewriter struct {
	equality                                                     arrayStoreEquality
	self                                                         *ArrayRewriter
	manager                                                      *ast.ASTManager
	params                                                       uparams.ModuleParamsOwner
	family                                                       ast.FamilyID
	header                                                       uint64
	ready, closed                                                bool
	sortStore, expandSelectStore, expandStoreEq, expandSelectIte bool
}

// A nonnil partial constructor result must be closed after an error.
func NewArrayRewriter(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef) (*ArrayRewriter, error) {
	if m == nil || c == nil || g == nil || c.SymbolsField != m.Symbols || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(ArrayRewriter{}).Size())
	if e := m.Budget().Reserve(n); e != nil {
		return nil, e
	}
	rw := &ArrayRewriter{manager: m, header: n}
	rw.self = rw
	fid, e := m.ArrayFamilyID()
	if e != nil {
		return rw, e
	}
	rw.family = fid
	rw.params = uparams.ModuleParamsOwner{Budget: g.Config.Budget}
	rw.params.Self = &rw.params
	if e = rw.params.Bind(c, g); e != nil {
		return rw, e
	}
	e = rw.updateParams(p)
	rw.ready = e == nil
	return rw, e
}
func (rw *ArrayRewriter) open() error {
	if rw == nil || rw.self != rw || rw.closed {
		return uparams.ErrParamsOwner
	}
	if !rw.ready {
		return uparams.ErrParamsPrecondition
	}
	if rw.equality.active {
		return ast.ErrASTPending
	}
	_, e := rw.manager.ProofsEnabled()
	return e
}
func (rw *ArrayRewriter) Close() error {
	if rw == nil || rw.self != rw {
		return uparams.ErrParamsOwner
	}
	if rw.closed {
		return nil
	}
	if e := rw.equality.close(rw.manager); e != nil {
		return e
	}
	if rw.params.Self != nil {
		if e := rw.params.CloseHelpers(); e != nil {
			return e
		}
	}
	rw.manager.Budget().Release(rw.header)
	rw.header = 0
	rw.closed = true
	return nil
}
func (rw *ArrayRewriter) UpdtParams(p *uparams.ParamsRef) error {
	if e := rw.open(); e != nil {
		return e
	}
	return rw.updateParams(p)
}
func (rw *ArrayRewriter) updateParams(local *uparams.ParamsRef) (err error) {
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
	b, e := o.FrameBool(f, "sort_store", false)
	if e != nil {
		return e
	}
	rw.sortStore = b
	b, e = o.FrameBool(f, "expand_select_store", false)
	if e != nil {
		return e
	}
	rw.expandSelectStore = b
	b, e = o.FrameBool(f, "expand_store_eq", false)
	if e != nil {
		return e
	}
	rw.expandStoreEq = b
	rw.expandSelectIte = false
	return nil
}
func (rw *ArrayRewriter) SetExpandSelectStore(b bool) error {
	if e := rw.open(); e != nil {
		return e
	}
	rw.expandSelectStore = b
	return nil
}
func (rw *ArrayRewriter) SetExpandSelectIte(b bool) error {
	if e := rw.open(); e != nil {
		return e
	}
	rw.expandSelectIte = b
	return nil
}
func (rw *ArrayRewriter) isOp(h ast.ASTHandle, op int32) (bool, error) {
	k, e := rw.manager.GetKind(h)
	if e != nil || k != ast.ASTApp {
		return false, e
	}
	fid, e := rw.manager.GetFamilyID(h)
	if e != nil || fid != rw.family {
		return false, e
	}
	kind, e := rw.manager.GetDeclKind(h)
	return kind == op, e
}

// Unlike a scan for any disequal pair, source compare_args stops at the first
// nonidentical slot. checkDisequal chooses its original two concrete variants.
func (rw *ArrayRewriter) compare(n uint32, a ast.AppArguments, ao uint32, b ast.AppArguments, bo uint32, checkDisequal bool) (int, error) {
	for i := uint32(0); i < n; i++ {
		x, e := a.At(ao + i)
		if e != nil {
			return 0, e
		}
		y, e := b.At(bo + i)
		if e != nil {
			return 0, e
		}
		if x == y {
			continue
		}
		if checkDisequal {
			distinct, e := rw.manager.AreDistinct(x, y)
			if e != nil {
				return 0, e
			}
			if distinct {
				return -1, nil
			}
		}
		return 0, nil
	}
	return 1, nil
}
func (rw *ArrayRewriter) appendArgs(buffer *[]ast.ASTHandle, args ast.AppArguments, start, count uint32) error {
	for i := uint32(0); i < count; i++ {
		h, e := args.At(start + i)
		if e != nil {
			return e
		}
		*buffer = append(*buffer, h)
	}
	return nil
}
func (rw *ArrayRewriter) MkAppCore(d ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	if e := rw.open(); e != nil {
		return RewriteFailed, e
	}
	fid, e := rw.manager.GetFamilyID(d)
	if e != nil {
		return RewriteFailed, e
	}
	if fid != rw.family {
		return RewriteFailed, ast.ErrASTDomain
	}
	if e := rw.traceAppCore(d, args); e != nil {
		return RewriteFailed, e
	}
	op, e := rw.manager.GetDeclKind(d)
	if e != nil {
		return RewriteFailed, e
	}
	switch op {
	case ast.ASTArrayOpSelect:
		return rw.selectCore(args, out)
	case ast.ASTArrayOpStore:
		return rw.storeCore(args, out)
	case ast.ASTArrayOpMap:
		f, e := rw.manager.ArrayMapFunction(d)
		if e != nil {
			return RewriteFailed, e
		}
		return rw.mapCore(f, args, out)
	case ast.ASTArrayOpSetUnion, ast.ASTArrayOpSetIntersect:
		return rw.setUnionIntersection(op, args, out)
	case ast.ASTArrayOpSetComplement:
		if args.Count() != 1 {
			return RewriteFailed, ast.ErrASTDomain
		}
		a, e := args.At(0)
		if e != nil {
			return RewriteFailed, e
		}
		return rw.setComplement(a, out)
	case ast.ASTArrayOpSetDifference, ast.ASTArrayOpSetSubset:
		if args.Count() != 2 {
			return RewriteFailed, ast.ErrASTDomain
		}
		a, e := args.At(0)
		if e != nil {
			return RewriteFailed, e
		}
		b, e := args.At(1)
		if e != nil {
			return RewriteFailed, e
		}
		if op == ast.ASTArrayOpSetDifference {
			return rw.setDifference(a, b, out)
		}
		return rw.setSubset(a, b, out)
	default:
		return RewriteFailed, nil
	}
}
func (rw *ArrayRewriter) storeCore(args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	m := rw.manager
	n := args.Count()
	if n < 3 {
		return RewriteFailed, ast.ErrASTDomain
	}
	base, e := args.At(0)
	if e != nil {
		return RewriteFailed, e
	}
	nested, e := rw.isOp(base, ast.ASTArrayOpStore)
	if e != nil {
		return RewriteFailed, e
	}
	if nested {
		old, e := m.AppArgumentView(base)
		if e != nil {
			return RewriteFailed, e
		}
		r, e := rw.compare(n-2, args, 1, old, 1, rw.sortStore)
		if e != nil {
			return RewriteFailed, e
		}
		reorder := false
		if r == -1 {
			for i := uint32(1); i < n-1; i++ {
				x, e := args.At(i)
				if e != nil {
					return RewriteFailed, e
				}
				y, e := old.At(i)
				if e != nil {
					return RewriteFailed, e
				}
				if x == y {
					continue
				}
				reorder, e = m.ASTLess(x, y)
				if e != nil {
					return RewriteFailed, e
				}
				break
			}
		}
		if r == 1 || reorder {
			var buffer []ast.ASTHandle
			h, e := old.At(0)
			if e != nil {
				return RewriteFailed, e
			}
			buffer = append(buffer, h)
			if e = rw.appendArgs(&buffer, args, 1, n-1); e != nil {
				return RewriteFailed, e
			}
			h, e = m.MkFamilyApp(rw.family, ast.ASTArrayOpStore, nil, buffer)
			if e != nil {
				return RewriteFailed, e
			}
			if r == 1 {
				return RewriteDone, m.SetRef(out, h)
			}
			clear(buffer)
			buffer = buffer[:0]
			buffer = append(buffer, h)
			if e = rw.appendArgs(&buffer, old, 1, n-1); e != nil {
				return RewriteFailed, e
			}
			h, e = m.MkFamilyApp(rw.family, ast.ASTArrayOpStore, nil, buffer)
			if e != nil {
				return RewriteFailed, e
			}
			return Rewrite2, m.SetRef(out, h)
		}
	}
	constant, e := rw.isOp(base, ast.ASTArrayOpConst)
	if e != nil {
		return RewriteFailed, e
	}
	value, e := args.At(n - 1)
	if e != nil {
		return RewriteFailed, e
	}
	if constant {
		old, e := m.GetArg(base, 0)
		if e != nil {
			return RewriteFailed, e
		}
		if old == value {
			return RewriteDone, m.SetRef(out, base)
		}
	}
	selected, e := rw.isOp(value, ast.ASTArrayOpSelect)
	if e != nil {
		return RewriteFailed, e
	}
	if selected {
		old, e := m.AppArgumentView(value)
		if e != nil {
			return RewriteFailed, e
		}
		r, e := rw.compare(n-1, args, 0, old, 0, false)
		if e != nil {
			return RewriteFailed, e
		}
		if r == 1 {
			return RewriteDone, m.SetRef(out, base)
		}
	}
	return RewriteFailed, nil
}
func (rw *ArrayRewriter) selectCore(args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	m := rw.manager
	n := args.Count()
	if n < 2 {
		return RewriteFailed, ast.ErrASTDomain
	}
	base, e := args.At(0)
	if e != nil {
		return RewriteFailed, e
	}
	stored, e := rw.isOp(base, ast.ASTArrayOpStore)
	if e != nil {
		return RewriteFailed, e
	}
	if stored {
		old, e := m.AppArgumentView(base)
		if e != nil {
			return RewriteFailed, e
		}
		if old.Count() != n+1 {
			return RewriteFailed, ast.ErrASTDomain
		}
		r, e := rw.compare(n-1, args, 1, old, 1, true)
		if e != nil {
			return RewriteFailed, e
		}
		if r == 1 {
			v, e := old.At(n)
			if e != nil {
				return RewriteFailed, e
			}
			return RewriteDone, m.SetRef(out, v)
		}
		if r == -1 || rw.expandSelectStore {
			var buffer []ast.ASTHandle
			h, e := old.At(0)
			if e != nil {
				return RewriteFailed, e
			}
			buffer = append(buffer, h)
			if e = rw.appendArgs(&buffer, args, 1, n-1); e != nil {
				return RewriteFailed, e
			}
			fallback, e := m.MkFamilyApp(rw.family, ast.ASTArrayOpSelect, nil, buffer)
			if e != nil {
				return RewriteFailed, e
			}
			if r == -1 {
				return Rewrite1, m.SetRef(out, fallback)
			}
			v, e := old.At(n)
			if e != nil {
				return RewriteFailed, e
			}
			var eqs []ast.ASTHandle
			for i := uint32(1); i < n; i++ {
				x, e := old.At(i)
				if e != nil {
					return RewriteFailed, e
				}
				y, e := args.At(i)
				if e != nil {
					return RewriteFailed, e
				}
				eq, e := m.MkEq(x, y)
				if e != nil {
					return RewriteFailed, e
				}
				eqs = append(eqs, eq)
			}
			cond := eqs[0]
			status := Rewrite2
			if n > 2 {
				cond, e = m.MkAndArgs(eqs)
				if e != nil {
					return RewriteFailed, e
				}
				status = Rewrite3
			}
			h, e = m.MkIte(cond, v, fallback)
			if e != nil {
				return RewriteFailed, e
			}
			return status, m.SetRef(out, h)
		}
		return RewriteFailed, nil
	}
	constant, e := rw.isOp(base, ast.ASTArrayOpConst)
	if e != nil {
		return RewriteFailed, e
	}
	if constant {
		v, e := m.GetArg(base, 0)
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteDone, m.SetRef(out, v)
	}
	asArray, e := rw.isOp(base, ast.ASTArrayOpAsArray)
	if e != nil {
		return RewriteFailed, e
	}
	if asArray {
		f, e := m.AsArrayFunction(base)
		if e != nil {
			return RewriteFailed, e
		}
		// The source passes the existing select-index array directly to mk_app.
		indices, e := args.Range(1, n-1)
		if e != nil {
			return RewriteFailed, e
		}
		h, e := m.RewriterMakeApp(f, indices)
		if e != nil {
			return RewriteFailed, e
		}
		return Rewrite1, m.SetRef(out, h)
	}
	if rw.expandSelectIte {
		kind, e := m.GetKind(base)
		if e != nil {
			return RewriteFailed, e
		}
		if kind == ast.ASTApp {
			fid, e := m.GetFamilyID(base)
			if e != nil {
				return RewriteFailed, e
			}
			op, e := m.GetDeclKind(base)
			if e != nil {
				return RewriteFailed, e
			}
			if fid == 0 && op == ast.ASTOpIte {
				var a, b []ast.ASTHandle
				th, e := m.GetArg(base, 1)
				if e != nil {
					return RewriteFailed, e
				}
				el, e := m.GetArg(base, 2)
				if e != nil {
					return RewriteFailed, e
				}
				a = append(a, th)
				for i := uint32(1); i < n; i++ {
					h, e := args.At(i)
					if e != nil {
						return RewriteFailed, e
					}
					a = append(a, h)
				}
				b = append(b, el)
				for i := uint32(1); i < n; i++ {
					h, e := args.At(i)
					if e != nil {
						return RewriteFailed, e
					}
					b = append(b, h)
				}
				c, e := m.GetArg(base, 0)
				if e != nil {
					return RewriteFailed, e
				}
				left, e := m.MkFamilyApp(rw.family, ast.ASTArrayOpSelect, nil, a)
				if e != nil {
					return RewriteFailed, e
				}
				right, e := m.MkFamilyApp(rw.family, ast.ASTArrayOpSelect, nil, b)
				if e != nil {
					return RewriteFailed, e
				}
				h, e := m.MkIte(c, left, right)
				if e != nil {
					return RewriteFailed, e
				}
				return Rewrite2, m.SetRef(out, h)
			}
		}
	}
	return RewriteFailed, nil
}

// Source: src/ast/rewriter/array_rewriter.h:56-61;
// src/ast/rewriter/array_rewriter.cpp:325-333.
func (rw *ArrayRewriter) MkStoreCore(args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	if e := rw.open(); e != nil {
		return RewriteFailed, e
	}
	return rw.storeCore(args, out)
}
func (rw *ArrayRewriter) MkSelectCore(args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	if e := rw.open(); e != nil {
		return RewriteFailed, e
	}
	return rw.selectCore(args, out)
}
func (rw *ArrayRewriter) MkStore(args ast.AppArguments, out ast.ASTRef) error {
	status, e := rw.MkStoreCore(args, out)
	if e != nil || status != RewriteFailed {
		return e
	}
	return rw.makeFallback(ast.ASTArrayOpStore, args, out)
}
func (rw *ArrayRewriter) MkSelect(args ast.AppArguments, out ast.ASTRef) error {
	status, e := rw.MkSelectCore(args, out)
	if e != nil || status != RewriteFailed {
		return e
	}
	return rw.makeFallback(ast.ASTArrayOpSelect, args, out)
}
func (rw *ArrayRewriter) makeFallback(op int32, args ast.AppArguments, out ast.ASTRef) error {
	h, e := rw.manager.MkArrayAppArguments(rw.family, op, args)
	if e != nil {
		return e
	}
	return rw.manager.SetRef(out, h)
}

// Source: src/ast/rewriter/array_rewriter.h:51.
func (rw *ArrayRewriter) FamilyID() (ast.FamilyID, error) {
	if e := rw.open(); e != nil {
		return ast.NullFamilyID, e
	}
	return rw.family, nil
}

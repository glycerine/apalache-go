package rewriter

// Source: src/ast/rewriter/distribute_forall.cpp;
// src/ast/rewriter/distribute_forall.h.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// DistributeForall rewrites each (forall X (not (or F1 ... Fn))) into the
// Boolean-rewriter representation of (and (forall X (not F1)) ...).
type DistributeForall struct {
	self    *DistributeForall
	manager *ast.ASTManager
	context *uparams.ParamsContext
	globals *uparams.GlobalParamsService
	cache   map[ast.ASTHandle]ast.ASTRef
	closed  bool
}

func NewDistributeForall(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService) (*DistributeForall, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if c == nil || g == nil {
		return nil, uparams.ErrParamsOwner
	}
	r := &DistributeForall{manager: m, context: c, globals: g, cache: make(map[ast.ASTHandle]ast.ASTRef)}
	r.self = r
	return r, nil
}

func (r *DistributeForall) open() error {
	if r == nil || r.self != r || r.closed || r.manager == nil || r.context == nil || r.globals == nil || r.cache == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (r *DistributeForall) flushCache() error {
	for h, ref := range r.cache {
		if err := r.manager.DelRef(&ref); err != nil {
			return err
		}
		delete(r.cache, h)
	}
	return nil
}

func (r *DistributeForall) Close() (result error) {
	if r == nil || r.self != r || r.manager == nil {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	if err := r.flushCache(); err != nil {
		return err
	}
	r.closed = true
	return nil
}

func (r *DistributeForall) cached(h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	ref, ok := r.cache[h]
	if !ok {
		return ast.ASTHandle{}, false, nil
	}
	got, err := r.manager.RefGet(ref)
	return got, true, err
}

func (r *DistributeForall) cacheResult(source, result ast.ASTHandle) error {
	if result == (ast.ASTHandle{}) {
		return ast.ErrASTHandle
	}
	ref, err := r.manager.NewRef(result)
	if err != nil {
		return err
	}
	if old, ok := r.cache[source]; ok {
		if err = r.manager.DelRef(&old); err != nil {
			_ = r.manager.DelRef(&ref)
			return err
		}
	}
	r.cache[source] = ref
	return nil
}

func (r *DistributeForall) children(h ast.ASTHandle) ([]ast.ASTHandle, error) {
	kind, err := r.manager.GetKind(h)
	if err != nil {
		return nil, err
	}
	switch kind {
	case ast.ASTVar:
		return nil, nil
	case ast.ASTApp:
		n, err := r.manager.GetNumArgs(h)
		if err != nil {
			return nil, err
		}
		children := make([]ast.ASTHandle, n)
		for i := uint32(0); i < n; i++ {
			children[i], err = r.manager.GetArg(h, i)
			if err != nil {
				return nil, err
			}
		}
		return children, nil
	case ast.ASTQuantifier:
		body, err := r.manager.GetQuantifierBody(h)
		if err != nil {
			return nil, err
		}
		return []ast.ASTHandle{body}, nil
	default:
		return nil, ast.ErrASTDomain
	}
}

func (r *DistributeForall) reduceApp(app ast.ASTHandle) error {
	n, err := r.manager.GetNumArgs(app)
	if err != nil {
		return err
	}
	args := make([]ast.ASTHandle, n)
	changed := false
	for j := n; j > 0; j-- {
		old, err := r.manager.GetArg(app, j-1)
		if err != nil {
			return err
		}
		args[j-1], _, err = r.cached(old)
		if err != nil {
			return err
		}
		if args[j-1] == (ast.ASTHandle{}) {
			return ast.ErrASTHandle
		}
		changed = changed || args[j-1] != old
	}
	result := app
	if changed {
		decl, err := r.manager.GetDecl(app)
		if err != nil {
			return err
		}
		result, err = r.manager.MkApp(decl, args)
		if err != nil {
			return err
		}
	}
	return r.cacheResult(app, result)
}

func (r *DistributeForall) isBasicOp(h ast.ASTHandle, op int32) (bool, error) {
	kind, err := r.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := r.manager.GetFamilyID(h)
	if err != nil || family != 0 {
		return false, err
	}
	declKind, err := r.manager.GetDeclKind(h)
	return declKind == op, err
}

func (r *DistributeForall) reduceQuantifier(q ast.ASTHandle) (result error) {
	forall, err := r.manager.IsForall(q)
	if err != nil {
		return err
	}
	if ast.ASTSourceDebug && !forall {
		return ast.ErrASTDomain
	}
	body, err := r.manager.GetQuantifierBody(q)
	if err != nil {
		return err
	}
	body, ok, err := r.cached(body)
	if err != nil {
		return err
	}
	if !ok {
		return ast.ErrASTHandle
	}
	isNot, err := r.isBasicOp(body, ast.ASTOpNot)
	if err != nil {
		return err
	}
	var disjunction ast.ASTHandle
	if isNot {
		disjunction, err = r.manager.GetArg(body, 0)
		if err != nil {
			return err
		}
	}
	isOr := false
	if isNot {
		isOr, err = r.isBasicOp(disjunction, ast.ASTOpOr)
		if err != nil {
			return err
		}
	}
	if !isOr {
		updated, err := r.manager.UpdateQuantifierBody(q, body)
		if err != nil {
			return err
		}
		return r.cacheResult(q, updated)
	}
	empty, err := r.context.NewValue()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, r.context.Del(&empty)) }()
	boolean, err := newBoolRewriter(r.manager, r.context, r.globals, &empty)
	if err != nil {
		if boolean != nil {
			return errors.Join(err, boolean.Close())
		}
		return err
	}
	defer func() { result = errors.Join(result, boolean.Close()) }()

	var owned []ast.ASTRef
	defer func() {
		for i := len(owned) - 1; i >= 0; i-- {
			result = errors.Join(result, r.manager.DelRef(&owned[i]))
		}
	}()
	scratch := func() (ast.ASTRef, error) {
		ref, e := r.manager.NewRef(ast.ASTHandle{})
		if e == nil {
			owned = append(owned, ref)
		}
		return ref, e
	}
	n, err := r.manager.GetNumArgs(disjunction)
	if err != nil {
		return err
	}
	conjuncts := make([]ast.ASTHandle, 0, n)
	for i := uint32(0); i < n; i++ {
		arg, err := r.manager.GetArg(disjunction, i)
		if err != nil {
			return err
		}
		notArg, err := scratch()
		if err != nil {
			return err
		}
		if err = boolean.mkNot(arg, notArg); err != nil {
			return err
		}
		negated, err := r.manager.RefGet(notArg)
		if err != nil {
			return err
		}
		updated, err := r.manager.UpdateQuantifierBody(q, negated)
		if err != nil {
			return err
		}
		eliminated, err := ElimUnusedVarsUFBV(r.manager, updated)
		if err != nil {
			return err
		}
		owned = append(owned, eliminated)
		conjunct, err := r.manager.RefGet(eliminated)
		if err != nil {
			return err
		}
		conjuncts = append(conjuncts, conjunct)
	}
	combined, err := scratch()
	if err != nil {
		return err
	}
	if err = boolean.mkAnd(conjuncts, combined); err != nil {
		return err
	}
	resultHandle, err := r.manager.RefGet(combined)
	if err != nil {
		return err
	}
	return r.cacheResult(q, resultHandle)
}

func (r *DistributeForall) reduce(h ast.ASTHandle) error {
	kind, err := r.manager.GetKind(h)
	if err != nil {
		return err
	}
	switch kind {
	case ast.ASTVar:
		return r.cacheResult(h, h)
	case ast.ASTApp:
		return r.reduceApp(h)
	case ast.ASTQuantifier:
		return r.reduceQuantifier(h)
	default:
		return ast.ErrASTDomain
	}
}

func (r *DistributeForall) trace(before, after ast.ASTHandle) error {
	tags := diagnostic.BorrowProcessTraceTags()
	return tags.EmitConditional("distribute_forall", "operator()", "src/ast/rewriter/distribute_forall.cpp", 160, func() bool { return true }, func(w io.Writer) (result error) {
		out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: r.manager.Budget()})
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, out.Close()) }()
		if err = r.manager.DisplayLowLevel(out, before, true, true); err != nil {
			return err
		}
		a, err := out.String()
		if err != nil {
			return err
		}
		if err = out.ResetText(); err != nil {
			return err
		}
		if err = r.manager.DisplayLowLevel(out, after, true, true); err != nil {
			return err
		}
		b, err := out.String()
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, a+"======>\n"+b)
		return err
	})
}

// Apply stores an independently owned result in out. Its traversal is
// iterative, matching the source visit_children/reduce1 discipline.
func (r *DistributeForall) Apply(root ast.ASTHandle, out ast.ASTRef) (result error) {
	if err := r.open(); err != nil {
		return err
	}
	if err := r.flushCache(); err != nil {
		return err
	}
	if _, err := r.manager.GetKind(root); err != nil {
		return err
	}
	todo := []ast.ASTHandle{root}
	for len(todo) != 0 {
		current := todo[len(todo)-1]
		children, err := r.children(current)
		if err != nil {
			return err
		}
		visited := true
		for i := len(children) - 1; i >= 0; i-- {
			if _, ok, err := r.cached(children[i]); err != nil {
				return err
			} else if !ok {
				todo = append(todo, children[i])
				visited = false
			}
		}
		if !visited {
			continue
		}
		todo = todo[:len(todo)-1]
		if err := r.reduce(current); err != nil {
			return err
		}
	}
	got, ok, err := r.cached(root)
	if err != nil {
		return err
	}
	if !ok {
		return ast.ErrASTHandle
	}
	if err = r.manager.SetRef(out, got); err != nil {
		return err
	}
	return r.trace(root, got)
}

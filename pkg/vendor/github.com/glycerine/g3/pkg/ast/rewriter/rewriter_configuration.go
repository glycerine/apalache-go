package rewriter

// Source: src/ast/rewriter/rewriter.h:365-408;
// src/ast/rewriter/bool_rewriter.h:181-200.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// Configurations are cold owners; frames, bindings and results use concrete
// handle storage shared by the source proof and no-proof specializations.

type rewriterConfiguration interface {
	cacheAllResults() bool
	cacheResults() bool
	flatAssoc(ast.ASTHandle) (bool, error)
	rewritePatterns() bool
	maxScopesExceeded(uint32) bool
	maxFramesExceeded(uint32) bool
	maxStepsExceeded(uint32) (bool, error)
	preVisit(ast.ASTHandle) (bool, error)
	reduceApp(ast.ASTHandle, ast.AppArguments, ast.ASTRef, ast.ASTRef) (brStatus, error)
	reduceVar(ast.ASTHandle, ast.ASTRef, ast.ASTRef) (bool, error)
	reduceQuantifier(ast.ASTHandle, ast.ASTHandle, ast.AppArguments, ast.AppArguments, ast.ASTRef, ast.ASTRef) (bool, error)
	getMacro(ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, ast.ASTHandle, bool, error)
	getSubst(ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error)
	reset()
	cleanup()
}
type defaultRewriterConfig struct{ manager *ast.ASTManager }

func (*defaultRewriterConfig) cacheAllResults() bool                 { return false }
func (*defaultRewriterConfig) cacheResults() bool                    { return true }
func (*defaultRewriterConfig) flatAssoc(ast.ASTHandle) (bool, error) { return false, nil }
func (*defaultRewriterConfig) rewritePatterns() bool                 { return true }
func (*defaultRewriterConfig) maxScopesExceeded(uint32) bool         { return false }
func (*defaultRewriterConfig) maxFramesExceeded(uint32) bool         { return false }
func (*defaultRewriterConfig) maxStepsExceeded(uint32) (bool, error) { return false, nil }
func (*defaultRewriterConfig) preVisit(ast.ASTHandle) (bool, error)  { return true, nil }
func (*defaultRewriterConfig) reduceApp(ast.ASTHandle, ast.AppArguments, ast.ASTRef, ast.ASTRef) (brStatus, error) {
	return brFailed, nil
}
func (*defaultRewriterConfig) reduceVar(ast.ASTHandle, ast.ASTRef, ast.ASTRef) (bool, error) {
	return false, nil
}
func (*defaultRewriterConfig) reduceQuantifier(ast.ASTHandle, ast.ASTHandle, ast.AppArguments, ast.AppArguments, ast.ASTRef, ast.ASTRef) (bool, error) {
	return false, nil
}
func (*defaultRewriterConfig) getMacro(ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, ast.ASTHandle, bool, error) {
	return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, nil
}
func (*defaultRewriterConfig) getSubst(ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
}
func (*defaultRewriterConfig) reset()   {}
func (*defaultRewriterConfig) cleanup() {}

type betaReducerConfig struct{ defaultRewriterConfig }

func (c *betaReducerConfig) preVisit(t ast.ASTHandle) (bool, error) {
	g, e := c.manager.IsGround(t)
	return !g, e
}

type betaReducer struct {
	engine rewriterEngine
	cfg    betaReducerConfig
}

func newBetaReducer(m *ast.ASTManager) (*betaReducer, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(betaReducer{}).Size())
	if e := m.BudgetField.Reserve(n); e != nil {
		return nil, e
	}
	r := &betaReducer{}
	r.cfg.manager = m
	r.engine.header = n
	return r, r.engine.init(m, &r.cfg)
}
func (r *betaReducer) Rewrite(t ast.ASTHandle, out ast.ASTRef) error { return r.engine.Rewrite(t, out) }
func (r *betaReducer) Close() error                                  { return r.engine.Close() }

type boolRewriterStarConfig struct {
	defaultRewriterConfig
	rw boolRewriter
}

func (c *boolRewriterStarConfig) rewritePatterns() bool { return false }
func (c *boolRewriterStarConfig) flatAssoc(d ast.ASTHandle) (bool, error) {
	if !c.rw.flat {
		return false, nil
	}
	fid, e := c.manager.GetFamilyID(d)
	if e != nil || fid != 0 {
		return false, e
	}
	op, e := c.manager.GetDeclKind(d)
	return op == ast.ASTOpAnd || op == ast.ASTOpOr, e
}
func (c *boolRewriterStarConfig) reduceApp(d ast.ASTHandle, args ast.AppArguments, out, pr ast.ASTRef) (brStatus, error) {
	if e := c.manager.ResetRef(pr); e != nil {
		return brFailed, e
	}
	fid, e := c.manager.GetFamilyID(d)
	if e != nil || fid != 0 {
		return brFailed, e
	}
	return c.rw.mkAppCoreArguments(d, args, out)
}

type boolRewriterStar struct {
	engine rewriterEngine
	cfg    boolRewriterStarConfig
}

func newBoolRewriterStar(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef) (*boolRewriterStar, error) {
	if m == nil || c == nil || g == nil || c.SymbolsField != m.Symbols || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(boolRewriterStar{}).Size())
	if e := m.BudgetField.Reserve(n); e != nil {
		return nil, e
	}
	r := &boolRewriterStar{}
	r.cfg.manager = m
	r.engine.header = n
	if e := r.engine.init(m, &r.cfg); e != nil {
		return r, e
	}
	return r, r.cfg.rw.init(m, c, g, p)
}
func (r *boolRewriterStar) Rewrite(t ast.ASTHandle, out ast.ASTRef) error {
	return r.engine.Rewrite(t, out)
}
func (r *boolRewriterStar) Close() error {
	if r.cfg.rw.self != nil {
		if e := r.cfg.rw.Close(); e != nil {
			return e
		}
	}
	return r.engine.Close()
}

func astSliceArguments(args []ast.ASTHandle) ast.AppArguments {
	return ast.SliceArguments(args)
}
func (rw *boolRewriter) mkAppCore(decl ast.ASTHandle, args []ast.ASTHandle, out ast.ASTRef) (brStatus, error) {
	return rw.mkAppCoreArguments(decl, astSliceArguments(args), out)
}
func (rw *boolRewriter) mkAppCoreArguments(decl ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) (brStatus, error) {
	if rw.self != nil {
		if e := rw.configuredOpen(); e != nil {
			return brFailed, e
		}
	}
	fid, e := rw.manager.GetFamilyID(decl)
	if e != nil {
		return brFailed, e
	}
	if ast.ASTSourceDebug && fid != 0 {
		return brFailed, ast.ErrASTDomain
	}
	op, e := rw.manager.GetDeclKind(decl)
	if e != nil {
		return brFailed, e
	}
	switch op {
	case ast.ASTOpAnd:
		if rw.elimAnd {
			return brDone, rw.mkAndAsOrArguments(args, out)
		}
		if rw.flat {
			return rw.mkFlatCore(args, out, false)
		}
		return rw.mkNFlatCore(args, out, false)
	case ast.ASTOpOr:
		if rw.flat {
			return rw.mkFlatCore(args, out, true)
		}
		return rw.mkNFlatCore(args, out, true)
	case ast.ASTOpDistinct:
		return rw.mkDistinctCoreArguments(args, out)
	}
	// Scalar callbacks have at most three arguments, read without a heap vector.
	if args.Count() > 3 {
		switch op {
		case ast.ASTOpEq, ast.ASTOpIff, ast.ASTOpNot, ast.ASTOpIte, ast.ASTOpImplies, ast.ASTOpXor:
			return brFailed, ast.ErrASTDomain
		}
		return brFailed, nil
	}
	var scalar [3]ast.ASTHandle
	for i := uint32(0); i < args.Count(); i++ {
		scalar[i], e = args.At(i)
		if e != nil {
			return brFailed, e
		}
	}
	switch op {
	case ast.ASTOpEq, ast.ASTOpIff:
		if args.Count() != 2 {
			return brFailed, ast.ErrASTDomain
		}
		return rw.mkEqCore(scalar[0], scalar[1], out)
	case ast.ASTOpNot:
		if args.Count() != 1 {
			return brFailed, ast.ErrASTDomain
		}
		return rw.mkNotCore(scalar[0], out)
	case ast.ASTOpIte:
		if args.Count() != 3 {
			return brFailed, ast.ErrASTDomain
		}
		return rw.mkITECore(scalar[0], scalar[1], scalar[2], out)
	case ast.ASTOpImplies:
		if args.Count() != 2 {
			return brFailed, ast.ErrASTDomain
		}
		return brDone, rw.mkImplies(scalar[0], scalar[1], out)
	case ast.ASTOpXor:
		switch args.Count() {
		case 0:
			return brFailed, nil
		case 1:
			return brDone, rw.manager.SetRef(out, scalar[0])
		case 2:
			return brDone, rw.mkXor(scalar[0], scalar[1], out)
		default:
			return brFailed, ast.ErrASTDomain
		}
	default:
		return brFailed, nil
	}
}

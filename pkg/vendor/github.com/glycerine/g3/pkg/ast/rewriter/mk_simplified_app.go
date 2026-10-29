package rewriter

// Source: src/ast/rewriter/mk_simplified_app.cpp;
// src/ast/rewriter/mk_simplified_app.h.

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// SimplifiedApp owns the source functor's concrete theory rewriters. The AST
// and parameter services are borrowed and must outlive it. Do not copy it.
type SimplifiedApp struct {
	self       *SimplifiedApp
	manager    *ast.ASTManager
	boolean    *BoolRewriter
	arithmetic *ArithRewriter
	bitVector  *BVRewriter
	array      *ArrayRewriter
	datatype   *DatatypeRewriter
	floating   *FPARewriter
	header     uint64
	closed     bool
}

// NewSimplifiedApp constructs the source members in declaration order. A
// nonnil partial owner returned with an error must be closed.
func NewSimplifiedApp(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, local *uparams.ParamsRef) (*SimplifiedApp, error) {
	if m == nil || c == nil || g == nil || c.SymbolsField != m.Symbols || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	legacy, err := g.Manager()
	if err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(SimplifiedApp{}).Size())
	if err = m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	r := &SimplifiedApp{manager: m, header: n}
	r.self = r
	if r.boolean, err = NewBoolRewriter(m, c, g, local); err != nil {
		return r, fmt.Errorf("construct Boolean rewriter: %w", err)
	}
	if r.arithmetic, err = NewArithRewriter(m, c, g, local); err != nil {
		return r, fmt.Errorf("construct arithmetic rewriter: %w", err)
	}
	if r.bitVector, err = NewBVRewriter(m, c, legacy, local); err != nil {
		return r, fmt.Errorf("construct bit-vector rewriter: %w", err)
	}
	if r.array, err = NewArrayRewriter(m, c, g, local); err != nil {
		return r, fmt.Errorf("construct array rewriter: %w", err)
	}
	if r.datatype, err = NewDatatypeRewriter(m); err != nil {
		return r, fmt.Errorf("construct datatype rewriter: %w", err)
	}
	if r.floating, err = NewFPARewriter(m); err != nil {
		return r, fmt.Errorf("construct floating-point rewriter: %w", err)
	}
	params, err := NewFPARewriterParams(c, g, local)
	if err != nil {
		return r, err
	}
	high, readErr := params.HighFPUnspecified()
	closeErr := params.Close()
	if err = errors.Join(readErr, closeErr); err != nil {
		return r, err
	}
	if err = r.floating.SetHighFPUnspecified(high); err != nil {
		return r, err
	}
	return r, nil
}

func (r *SimplifiedApp) open() error {
	if r == nil || r.self != r || r.closed || r.manager == nil {
		return ast.ErrASTHandle
	}
	_, err := r.manager.ProofsEnabled()
	return err
}

// MkCore applies one theory reducer after the caller has rewritten the
// arguments. RewriteFailed leaves out unchanged.
func (r *SimplifiedApp) MkCore(decl ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	if err := r.open(); err != nil {
		return RewriteFailed, err
	}
	family, err := r.manager.GetFamilyID(decl)
	if err != nil {
		return RewriteFailed, err
	}
	if family == ast.NullFamilyID {
		return RewriteFailed, nil
	}
	if family == r.manager.BasicFamilyID() {
		kind, err := r.manager.GetDeclKind(decl)
		if err != nil {
			return RewriteFailed, err
		}
		if kind == ast.ASTOpEq {
			if args.Count() != 2 {
				return RewriteFailed, ast.ErrASTDomain
			}
			left, err := args.At(0)
			if err != nil {
				return RewriteFailed, err
			}
			right, err := args.At(1)
			if err != nil {
				return RewriteFailed, err
			}
			sort, err := r.manager.GetSort(left)
			if err != nil {
				return RewriteFailed, err
			}
			sortFamily, err := r.manager.GetFamilyID(sort)
			if err != nil {
				return RewriteFailed, err
			}
			status := RewriteFailed
			switch sortFamily {
			case r.arithmetic.family:
				status, err = r.arithmetic.MkEqCore(left, right, out)
			case r.bitVector.bvFamily:
				status, err = r.bitVector.RewriteEquality(left, right, out)
			case r.datatype.family:
				status, err = r.datatype.RewriteEquality(left, right, out)
			case r.floating.family:
				status, err = r.floating.MkEqCore(left, right, out)
			}
			if err != nil || status != RewriteFailed {
				return status, err
			}
		}
		return r.boolean.MkAppCore(decl, args, out)
	}
	switch family {
	case r.arithmetic.family:
		return r.arithmetic.MkAppCore(decl, args, out)
	case r.bitVector.bvFamily:
		return r.bitVector.RewriteApp(decl, args, out)
	case r.array.family:
		return r.array.MkAppCore(decl, args, out)
	case r.datatype.family:
		return r.datatype.RewriteApp(decl, args, out)
	case r.floating.family:
		return r.floating.MkAppCore(decl, args, out)
	default:
		return RewriteFailed, nil
	}
}

// Make mirrors operator(): it clears out, tries a core reduction, and builds
// the original application when the core leaves no result.
func (r *SimplifiedApp) Make(decl ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	if err := r.manager.SetRef(out, ast.ASTHandle{}); err != nil {
		return err
	}
	if _, err := r.MkCore(decl, args, out); err != nil {
		return err
	}
	result, err := r.manager.RefGet(out)
	if err != nil || !result.IsNull() {
		return err
	}
	result, err = r.manager.RewriterMakeApp(decl, args)
	if err != nil {
		return err
	}
	return r.manager.SetRef(out, result)
}

// Close destroys the owned rewriters in reverse declaration order.
func (r *SimplifiedApp) Close() error {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	if r.floating != nil {
		if err := r.floating.Close(); err != nil {
			return err
		}
		r.floating = nil
	}
	r.datatype = nil
	if r.array != nil {
		if err := r.array.Close(); err != nil {
			return err
		}
		r.array = nil
	}
	if r.bitVector != nil {
		if err := r.bitVector.Close(); err != nil {
			return err
		}
		r.bitVector = nil
	}
	if r.arithmetic != nil {
		if err := r.arithmetic.Close(); err != nil {
			return err
		}
		r.arithmetic = nil
	}
	if r.boolean != nil {
		if err := r.boolean.Close(); err != nil {
			return err
		}
		r.boolean = nil
	}
	r.manager.Budget().Release(r.header)
	r.header = 0
	r.closed = true
	return nil
}

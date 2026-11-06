package rewriter

// Source: src/ast/rewriter/dl_rewriter.cpp; src/ast/rewriter/dl_rewriter.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type DLRewriter struct {
	self    *DLRewriter
	manager *ast.ASTManager
	family  ast.FamilyID
	closed  bool
}

func NewDLRewriter(m *ast.ASTManager) (*DLRewriter, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if err := m.RegisterDeclarationPlugins(); err != nil {
		return nil, err
	}
	family, err := m.RegisterDatalogPlugin()
	if err != nil {
		return nil, err
	}
	r := &DLRewriter{manager: m, family: family}
	r.self = r
	return r, nil
}

func (r *DLRewriter) open() error {
	if r == nil || r.self != r || r.closed || r.manager == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (r *DLRewriter) FamilyID() (ast.FamilyID, error) {
	if err := r.open(); err != nil {
		return ast.NullFamilyID, err
	}
	return r.family, nil
}

func (r *DLRewriter) numeralExt(h ast.ASTHandle) (value uint64, matched bool, result error) {
	if value, matched, result = r.manager.DatalogNumeralValue(h); result != nil || matched {
		return
	}
	q := r.manager.NumeralService()
	n, err := q.NewValue()
	if err != nil {
		return 0, false, err
	}
	defer func() { result = errors.Join(result, q.Del(&n)) }()
	var width uint32
	matched, err = r.manager.BitVectorNumeralValue(h, &n, &width)
	if err != nil {
		return 0, false, err
	}
	if matched && width < 64 {
		valid, err := q.IsUint64(&n)
		if err != nil {
			return 0, false, err
		}
		if !valid {
			return 0, false, ast.ErrASTDomain
		}
		value, err = q.Uint64(&n)
		return value, true, err
	}
	if r.manager.IsTrue(h) {
		return 1, true, nil
	}
	if r.manager.IsFalse(h) {
		return 0, true, nil
	}
	return 0, false, nil
}

// RewriteApp is the complete mk_app_core callback. It borrows decl and args
// and publishes a result only for a source BR_DONE case.
func (r *DLRewriter) RewriteApp(decl ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	if err := r.open(); err != nil {
		return RewriteFailed, err
	}
	family, err := r.manager.GetFamilyID(decl)
	if err != nil || family != r.family {
		return RewriteFailed, err
	}
	kind, err := r.manager.GetDeclKind(decl)
	if err != nil || kind != ast.DatalogOpLT {
		return RewriteFailed, err
	}
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
	v1, n1, err := r.numeralExt(left)
	if err != nil {
		return RewriteFailed, err
	}
	if n1 {
		v2, n2, err := r.numeralExt(right)
		if err != nil {
			return RewriteFailed, err
		}
		if n2 {
			value, err := r.manager.MkBoolVal(v1 < v2)
			if err != nil {
				return RewriteFailed, err
			}
			return RewriteDone, r.manager.SetRef(out, value)
		}
	}
	if left == right {
		value, err := r.manager.MkFalse()
		if err != nil {
			return RewriteFailed, err
		}
		return RewriteDone, r.manager.SetRef(out, value)
	}
	v2, rightNumeral, err := r.numeralExt(right)
	if err != nil {
		return RewriteFailed, err
	}
	if rightNumeral && v2 == 0 {
		value, err := r.manager.MkFalse()
		if err != nil {
			return RewriteFailed, err
		}
		return RewriteDone, r.manager.SetRef(out, value)
	}
	// The source's commented 0 < x rule probes args[1] again. Since the
	// preceding branch handles that same zero, this retained branch cannot fire.
	if v1, rightNumeral, err := r.numeralExt(right); err != nil {
		return RewriteFailed, err
	} else if rightNumeral && v1 == 0 {
		eq, err := r.manager.MkEq(left, right)
		if err != nil {
			return RewriteFailed, err
		}
		value, err := r.manager.MkNot(eq)
		if err != nil {
			return RewriteFailed, err
		}
		return RewriteDone, r.manager.SetRef(out, value)
	}
	return RewriteFailed, nil
}

func (r *DLRewriter) Close() error {
	if r == nil || r.self != r || r.manager == nil {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	r.closed = true
	return nil
}

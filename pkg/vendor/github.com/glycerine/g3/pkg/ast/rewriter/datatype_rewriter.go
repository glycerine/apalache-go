package rewriter

// Source: src/ast/rewriter/datatype_rewriter.cpp:24-140;
// src/ast/rewriter/datatype_rewriter.h:24-35.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// DatatypeRewriter borrows its manager and performs the retained datatype
// reductions after traversal has rewritten the arguments.
type DatatypeRewriter struct {
	manager *ast.ASTManager
	family  ast.FamilyID
}

func NewDatatypeRewriter(manager *ast.ASTManager) (*DatatypeRewriter, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	family, err := manager.DatatypeFamilyID()
	if err != nil {
		return nil, err
	}
	return &DatatypeRewriter{manager: manager, family: family}, nil
}

func (r *DatatypeRewriter) RewriteApp(decl ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	if r == nil || r.manager == nil {
		return RewriteFailed, ast.ErrASTHandle
	}
	family, err := r.manager.GetFamilyID(decl)
	if err != nil {
		return RewriteFailed, err
	}
	if family != r.family {
		return RewriteFailed, nil
	}
	kind, err := r.manager.GetDeclKind(decl)
	if err != nil {
		return RewriteFailed, err
	}
	switch kind {
	case ast.ASTDatatypeConstructor:
		return RewriteFailed, nil
	case ast.ASTDatatypeRecognizer:
		if args.Count() != 1 {
			return RewriteFailed, nil
		}
		constructor, matched, err := r.manager.DatatypeRecognizerConstructor(decl)
		if err != nil || !matched {
			return RewriteFailed, err
		}
		parameter, err := r.manager.NewParameterAST(constructor)
		if err != nil {
			return RewriteFailed, err
		}
		argument, err := args.At(0)
		if err != nil {
			return RewriteFailed, errors.Join(err, r.manager.DelParameter(&parameter))
		}
		result, err := r.manager.MkFamilyApp(r.family, ast.ASTDatatypeIs, []ast.ASTParameter{parameter}, []ast.ASTHandle{argument})
		if err != nil {
			return RewriteFailed, errors.Join(err, r.manager.DelParameter(&parameter))
		}
		if err = r.manager.DelParameter(&parameter); err != nil {
			return RewriteFailed, err
		}
		return Rewrite1, r.manager.SetRef(out, result)
	case ast.ASTDatatypeIs:
		if args.Count() != 1 {
			return RewriteFailed, nil
		}
		argument, err := args.At(0)
		if err != nil {
			return RewriteFailed, err
		}
		actual, matched, err := r.manager.DatatypeConstructorApp(argument)
		if err != nil || !matched {
			return RewriteFailed, err
		}
		expected, matched, err := r.manager.DatatypeRecognizerConstructor(decl)
		if err != nil || !matched {
			return RewriteFailed, err
		}
		return r.assignBoolean(actual == expected, out)
	case ast.ASTDatatypeAccessor:
		return r.rewriteAccessor(decl, args, out)
	case ast.ASTDatatypeUpdateField:
		return r.rewriteUpdate(decl, args, out)
	default:
		return RewriteFailed, nil
	}
}

func (r *DatatypeRewriter) rewriteAccessor(accessor ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	if args.Count() != 1 {
		return RewriteFailed, nil
	}
	value, err := args.At(0)
	if err != nil {
		return RewriteFailed, err
	}
	constructor, matched, err := r.manager.DatatypeConstructorApp(value)
	if err != nil || !matched {
		return RewriteFailed, err
	}
	index, matched, err := r.manager.DatatypeAccessorIndex(accessor, constructor)
	if err != nil || !matched {
		return RewriteFailed, err
	}
	field, err := r.manager.GetArg(value, index)
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, field)
}

func (r *DatatypeRewriter) rewriteUpdate(decl ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	if args.Count() != 2 {
		return RewriteFailed, nil
	}
	value, err := args.At(0)
	if err != nil {
		return RewriteFailed, err
	}
	replacement, err := args.At(1)
	if err != nil {
		return RewriteFailed, err
	}
	constructor, matched, err := r.manager.DatatypeConstructorApp(value)
	if err != nil || !matched {
		return RewriteFailed, err
	}
	accessor, matched, err := r.manager.DatatypeUpdateAccessor(decl)
	if err != nil || !matched {
		return RewriteFailed, err
	}
	index, sameConstructor, err := r.manager.DatatypeAccessorIndex(accessor, constructor)
	if err != nil {
		return RewriteFailed, err
	}
	if !sameConstructor {
		return RewriteDone, r.manager.SetRef(out, value)
	}
	count, err := r.manager.GetNumArgs(value)
	if err != nil {
		return RewriteFailed, err
	}
	fields := make([]ast.ASTHandle, count)
	for i := uint32(0); i < count; i++ {
		fields[i], err = r.manager.GetArg(value, i)
		if err != nil {
			return RewriteFailed, err
		}
	}
	fields[index] = replacement
	result, err := r.manager.MkApp(constructor, fields)
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, result)
}

func (r *DatatypeRewriter) RewriteEquality(left, right ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if r == nil || r.manager == nil {
		return RewriteFailed, ast.ErrASTHandle
	}
	leftConstructor, leftMatched, err := r.manager.DatatypeConstructorApp(left)
	if err != nil || !leftMatched {
		return RewriteFailed, err
	}
	rightConstructor, rightMatched, err := r.manager.DatatypeConstructorApp(right)
	if err != nil || !rightMatched {
		return RewriteFailed, err
	}
	if leftConstructor != rightConstructor {
		return r.assignBoolean(false, out)
	}
	count, err := r.manager.GetNumArgs(left)
	if err != nil {
		return RewriteFailed, err
	}
	rightCount, err := r.manager.GetNumArgs(right)
	if err != nil {
		return RewriteFailed, err
	}
	if count != rightCount {
		return RewriteFailed, ast.ErrASTDomain
	}
	equalities := make([]ast.ASTHandle, count)
	for i := uint32(0); i < count; i++ {
		lhs, e := r.manager.GetArg(left, i)
		if e != nil {
			return RewriteFailed, e
		}
		rhs, e := r.manager.GetArg(right, i)
		if e != nil {
			return RewriteFailed, e
		}
		equalities[i], e = r.manager.MkEq(lhs, rhs)
		if e != nil {
			return RewriteFailed, e
		}
	}
	result, err := r.manager.MkAndArgs(equalities)
	if err != nil {
		return RewriteFailed, err
	}
	return Rewrite2, r.manager.SetRef(out, result)
}

func (r *DatatypeRewriter) assignBoolean(value bool, out ast.ASTRef) (RewriteStatus, error) {
	var result ast.ASTHandle
	var err error
	if value {
		result, err = r.manager.MkTrue()
	} else {
		result, err = r.manager.MkFalse()
	}
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, result)
}

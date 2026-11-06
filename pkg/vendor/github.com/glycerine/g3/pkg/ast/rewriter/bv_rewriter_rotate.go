package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2000-2021.

import "github.com/glycerine/g3/pkg/ast"

func rotationManager(extract *MkExtractProc) (*ast.ASTManager, error) {
	if extract == nil || extract.self != extract || extract.closed {
		return nil, ast.ErrASTHandle
	}
	if extract.running || extract.failure != nil {
		return nil, ast.ErrASTPending
	}
	return extract.manager, nil
}

// Fixed rotation core reducers borrow the existing extraction procedure, as
// the source BV rewriter does. Right rotation delegates back to left rotation.
func RewriteBitVectorRotateLeft(extract *MkExtractProc, n uint32, arg ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	m, e := rotationManager(extract)
	if e != nil {
		return RewriteFailed, e
	}
	size, e := m.BitVectorSize(arg)
	if e != nil {
		return RewriteFailed, e
	}
	if size == 0 {
		return RewriteFailed, ast.ErrASTDomain
	}
	n %= size
	if n == 0 || size == 1 {
		return RewriteDone, m.SetRef(out, arg)
	}
	first, e := extract.Apply(size-n-1, 0, arg)
	if e != nil {
		return RewriteFailed, e
	}
	second, e := extract.Apply(size-1, size-n, arg)
	if e != nil {
		return RewriteFailed, e
	}
	h, e := m.MkBitVectorConcatPair(first, second)
	if e != nil {
		return RewriteFailed, e
	}
	return Rewrite2, m.SetRef(out, h)
}

func RewriteBitVectorRotateRight(extract *MkExtractProc, n uint32, arg ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	m, e := rotationManager(extract)
	if e != nil {
		return RewriteFailed, e
	}
	size, e := m.BitVectorSize(arg)
	if e != nil {
		return RewriteFailed, e
	}
	if size == 0 {
		return RewriteFailed, ast.ErrASTDomain
	}
	n %= size
	return RewriteBitVectorRotateLeft(extract, size-n, arg, out)
}

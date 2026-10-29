package rewriter

// Source: src/ast/rewriter/bool_rewriter.cpp:89-280;
// src/ast/rewriter/bool_rewriter.h:60-69; src/util/buffer.h:25-48.

import (
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// Original Boolean rewriter's AND/OR core fragment. This inline helper borrows
// its AST manager; boolRewriterStar supplies the original traversal.
type boolRewriter struct {
	self                                *boolRewriter
	params                              uparams.ModuleParamsOwner
	header                              uint64
	ready, closed                       bool
	elimAnd                             bool
	blastDistinct                       bool
	blastDistinctThreshold              uint32
	manager                             *ast.ASTManager
	flat, iteExtraRules                 bool
	localContext                        bool
	localContextCost, localContextLimit uint32
	localOld, localNew                  []ast.ASTHandle
	localArg                            ast.ASTRef
	localActive                         bool
	andArgs                             [][]ast.ASTHandle
	andDepth                            uint32
}

func (rw *boolRewriter) isOperator(h ast.ASTHandle, op int32) (bool, error) {
	k, e := rw.manager.GetKind(h)
	if e != nil || k != ast.ASTApp {
		return false, e
	}
	fid, e := rw.manager.GetFamilyID(h)
	if e != nil || fid != 0 {
		return false, e
	}
	kind, e := rw.manager.GetDeclKind(h)
	return kind == op, e
}

func (rw *boolRewriter) mkNFlatAndCore(args []ast.ASTHandle, out ast.ASTRef) (brStatus, error) {
	return rw.mkNFlatCore(astSliceArguments(args), out, false)
}
func (rw *boolRewriter) mkNFlatOrCore(args []ast.ASTHandle, out ast.ASTRef) (brStatus, error) {
	return rw.mkNFlatCore(astSliceArguments(args), out, true)
}

// The two original loops are dual except for OR's optional local-context call.
// Keep that source branch after mark reset and before the changed-result test.
func (rw *boolRewriter) mkNFlatCore(args ast.AppArguments, out ast.ASTRef, disjunction bool) (status brStatus, err error) {
	m := rw.manager
	var buffer []ast.ASTHandle
	neg := make(map[ast.ASTHandle]bool)
	pos := make(map[ast.ASTHandle]bool)
	unit, e := m.MkBoolVal(!disjunction)
	if e != nil {
		return brFailed, e
	}
	absorb, e := m.MkBoolVal(disjunction)
	if e != nil {
		return brFailed, e
	}
	changed := false
	for i := uint32(0); i < args.Count(); i++ {
		arg, e := args.At(i)
		if e != nil {
			return brFailed, e
		}
		if arg == unit {
			changed = true
			continue
		}
		if arg == absorb {
			return brDone, m.SetRef(out, absorb)
		}
		negative, atom, e := m.IsNot(arg)
		if e != nil {
			return brFailed, e
		}
		if negative {
			marked := neg[atom]
			if marked {
				changed = true
				continue
			}
			marked = pos[atom]
			if marked {
				return brDone, m.SetRef(out, absorb)
			}
			neg[atom] = true
		} else {
			marked := pos[arg]
			if marked {
				changed = true
				continue
			}
			marked = neg[arg]
			if marked {
				return brDone, m.SetRef(out, absorb)
			}
			pos[arg] = true
		}
		buffer = append(buffer, arg)
	}
	remaining := buffer
	switch len(remaining) {
	case 0:
		return brDone, m.SetRef(out, unit)
	case 1:
		return brDone, m.SetRef(out, remaining[0])
	}
	if disjunction && rw.localContext && rw.localContextCost <= rw.localContextLimit {
		clear(neg)
		clear(pos)
		simplified, e := rw.localContextSimplify(remaining, out)
		if e != nil {
			return brFailed, e
		}
		if simplified {
			return brDone, nil
		}
	}
	if !changed {
		return brFailed, nil
	}
	var result ast.ASTHandle
	if disjunction {
		result, e = m.MkOrArgs(remaining)
	} else {
		result, e = m.MkAndArgs(remaining)
	}
	if e != nil {
		return brFailed, e
	}
	return brDone, m.SetRef(out, result)
}

func (rw *boolRewriter) mkFlatAndCore(args []ast.ASTHandle, out ast.ASTRef) (brStatus, error) {
	return rw.mkFlatCore(astSliceArguments(args), out, false)
}
func (rw *boolRewriter) mkFlatOrCore(args []ast.ASTHandle, out ast.ASTRef) (brStatus, error) {
	return rw.mkFlatCore(astSliceArguments(args), out, true)
}
func (rw *boolRewriter) mkFlatCore(args ast.AppArguments, out ast.ASTRef, disjunction bool) (brStatus, error) {
	op := int32(ast.ASTOpAnd)
	if disjunction {
		op = ast.ASTOpOr
	}
	i := uint32(0)
	for ; i < args.Count(); i++ {
		arg, e := args.At(i)
		if e != nil {
			return brFailed, e
		}
		nested, e := rw.isOperator(arg, op)
		if e != nil {
			return brFailed, e
		}
		if nested {
			break
		}
	}
	if i == args.Count() {
		return rw.mkNFlatCore(args, out, disjunction)
	}
	m := rw.manager
	var flat []ast.ASTHandle
	for j := uint32(0); j < i; j++ {
		arg, e := args.At(j)
		if e != nil {
			return brFailed, e
		}
		flat = append(flat, arg)
	}
	for ; i < args.Count(); i++ {
		arg, e := args.At(i)
		if e != nil {
			return brFailed, e
		}
		nested, e := rw.isOperator(arg, op)
		if e != nil {
			return brFailed, e
		}
		if !nested {
			flat = append(flat, arg)
			continue
		}
		n, e := m.GetNumArgs(arg)
		if e != nil {
			return brFailed, e
		}
		for j := uint32(0); j < n; j++ {
			h, e := m.GetArg(arg, j)
			if e != nil {
				return brFailed, e
			}
			flat = append(flat, h)
		}
	}
	status, e := rw.mkNFlatCore(astSliceArguments(flat), out, disjunction)
	if e != nil {
		return brFailed, e
	}
	if status == brFailed {
		var result ast.ASTHandle
		if disjunction {
			result, e = m.MkOrArgs(flat)
		} else {
			result, e = m.MkAndArgs(flat)
		}
		if e != nil {
			return brFailed, e
		}
		if e = m.SetRef(out, result); e != nil {
			return brFailed, e
		}
	}
	return brDone, nil
}

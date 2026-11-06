package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:590-666;
// src/ast/rewriter/bv_rewriter.h:84.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

type bvExtractPropagation struct {
	numerals         *ast.BitVectorExtractNumerals
	args, concatArgs []ast.ASTHandle
	header           uint64
}

func (v *bvExtractPropagation) close(m *ast.ASTManager) error {
	if v.numerals != nil {
		if e := v.numerals.CloseExpression(); e != nil {
			return e
		}
	}
	clear(v.concatArgs)
	v.concatArgs = nil
	clear(v.args)
	v.args = nil
	if v.numerals != nil {
		if e := v.numerals.Close(); e != nil {
			return e
		}
		v.numerals = nil
	}
	m.Budget().Release(v.header)
	v.header = 0
	return nil
}
func (p *BVRewriter) PropagateExtract(high uint32, arg ast.ASTHandle, out ast.ASTRef) (removed uint32, err error) {
	if e := p.open(); e != nil {
		return 0, e
	}
	p.running = true
	defer func() { p.running = false; p.fail(err) }()
	return p.propagateExtract(high, arg, out)
}
func (p *BVRewriter) propagateExtract(high uint32, arg ast.ASTHandle, out ast.ASTRef) (uint32, error) {
	m := p.manager
	matched, e := m.IsBitVectorOperation(arg, 4)
	if e == nil && !matched {
		matched, e = m.IsBitVectorOperation(arg, 6)
	}
	if e != nil || !matched {
		return 0, e
	}
	width, e := m.BitVectorSize(arg)
	if e != nil {
		return 0, e
	}
	var toRemove uint32
	if high+1 < width {
		toRemove = width - high - 1
	}
	if toRemove == 0 {
		return 0, nil
	}
	args, e := m.AppArgumentView(arg)
	if e != nil {
		return 0, e
	}
	bytes := uint64(reflect.TypeOf(bvExtractPropagation{}).Size())
	if e = m.Budget().Reserve(bytes); e != nil {
		return 0, e
	}
	v := &bvExtractPropagation{header: bytes}
	p.propagation = v
	v.numerals, e = ast.NewBitVectorExtractNumerals(m)
	if e != nil {
		return 0, e
	}
	node, removed, e := p.bvPropagateExtractBody(v, args, arg, toRemove)
	if e != nil {
		return 0, e
	}
	if removed != 0 {
		if e = m.SetRef(out, node); e != nil {
			return removed, e
		}
	}
	clear(v.concatArgs)
	v.concatArgs = nil
	clear(v.args)
	v.args = nil
	if e = v.numerals.SourceEnd(); e != nil {
		return removed, e
	}
	if e = v.close(m); e != nil {
		return removed, e
	}
	p.propagation = nil
	return removed, nil
}

func (p *BVRewriter) bvPropagateExtractBody(v *bvExtractPropagation, args ast.AppArguments, arg ast.ASTHandle, toRemove uint32) (ast.ASTHandle, uint32, error) {
	m := p.manager
	failed := ast.ASTHandle{}
	allNumerals, removable := true, toRemove
	firstSize := ^uint32(0)
	first := func(current ast.ASTHandle) (ast.ASTHandle, ast.AppArguments, bool, error) {
		concat, e := m.IsBitVectorOperation(current, 37)
		if e != nil || !concat {
			return current, ast.AppArguments{}, false, e
		}
		children, e := m.AppArgumentView(current)
		if e != nil || children.Count() == 0 {
			return ast.ASTHandle{}, children, true, e
		}
		h, e := children.At(0)
		return h, children, true, e
	}
	for i := uint32(0); i < args.Count(); i++ {
		current, e := args.At(i)
		if e != nil {
			return failed, 0, e
		}
		head, children, concat, e := first(current)
		if e != nil {
			return failed, 0, e
		}
		if concat && children.Count() == 0 {
			continue
		}
		if !allNumerals {
			size, e := m.BitVectorSize(head)
			if e != nil || removable != size {
				return failed, 0, e
			}
			continue
		}
		numeric, e := v.numerals.Read(head, &firstSize)
		if e != nil {
			return failed, 0, e
		}
		if numeric {
			if firstSize < removable {
				removable = firstSize
			}
		} else {
			allNumerals = false
			firstSize, e = m.BitVectorSize(head)
			if e != nil || firstSize > removable {
				return failed, 0, e
			}
			removable = firstSize
		}
		if removable == 0 {
			return failed, 0, nil
		}
	}
	if ast.ASTSourceDebug && removable > toRemove {
		return failed, 0, ast.ErrASTDomain
	}
	for i := uint32(0); i < args.Count(); i++ {
		current, e := args.At(i)
		if e != nil {
			return failed, 0, e
		}
		head, children, concat, e := first(current)
		if e != nil {
			return failed, 0, e
		}
		if concat && children.Count() == 0 {
			continue
		}
		var newFirst ast.ASTHandle
		numeric, e := v.numerals.Read(head, &firstSize)
		if e != nil {
			return failed, 0, e
		}
		if numeric {
			if ast.ASTSourceDebug && firstSize < removable {
				return failed, 0, ast.ErrASTDomain
			}
			if newSize := firstSize - removable; newSize != 0 {
				newFirst, e = v.numerals.Constant(0, newSize)
				if e != nil {
					return failed, 0, e
				}
			}
		}
		newArg := newFirst
		if concat {
			if !newFirst.IsNull() {
				clear(v.concatArgs)
				v.concatArgs = v.concatArgs[:0]
				v.concatArgs = append(v.concatArgs, newFirst)
				for j := uint32(1); j < children.Count(); j++ {
					h, e := children.At(j)
					if e == nil {
						v.concatArgs = append(v.concatArgs, h)
					}
					if e != nil {
						return failed, 0, e
					}
				}
				newArg, e = m.MkBitVectorConcat(v.concatArgs)
			} else {
				switch children.Count() {
				case 1:
					newArg = ast.ASTHandle{}
				case 2:
					newArg, e = children.At(1)
				default:
					var suffix ast.AppArguments
					suffix, e = children.Range(1, children.Count()-1)
					if e == nil {
						newArg, e = m.MkBitVectorConcatArguments(suffix)
					}
				}
			}
			if e != nil {
				return failed, 0, e
			}
		}
		if !newArg.IsNull() {
			v.args = append(v.args, newArg)
		}
	}
	kind, e := m.GetDeclKind(arg)
	if e != nil {
		return failed, 0, e
	}
	node, e := m.MkFamilyApp(p.bvFamily, kind, nil, v.args)
	if e == nil && ast.ASTSourceDebug {
		sort, er := m.GetSort(node)
		if er != nil {
			return failed, 0, er
		}
		bv, er := m.IsBitVectorSort(sort)
		if er != nil {
			return failed, 0, er
		}
		if !bv {
			return failed, 0, ast.ErrASTDomain
		}
	}
	return node, removable, e
}

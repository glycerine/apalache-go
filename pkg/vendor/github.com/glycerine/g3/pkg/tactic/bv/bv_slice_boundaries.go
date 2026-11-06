package bv

// Source: src/ast/simplifiers/bv_slice.cpp register_slice/slice_eq.

import "github.com/glycerine/g3/pkg/ast"

func (f *bvSliceFrame) register(low, high uint32, h ast.ASTHandle) error {
	var base ast.ASTHandle
	var lo, hi uint32
	for {
		yes, e := f.owner.manager.BitVectorExtractValue(h, &lo, &hi, &base)
		if e != nil {
			return e
		}
		if !yes {
			break
		}
		low += lo
		high += lo
		h = base
	}
	width, e := f.owner.manager.BitVectorSize(h)
	if e != nil {
		return e
	}
	if high-low+1 == width {
		return nil
	}
	if e = f.pinKey(h); e != nil {
		return e
	}
	set := f.boundaries[h]
	if set == nil {
		set = make(map[uint32]struct{})
		f.boundaries[h] = set
	}
	if low > 0 {
		set[low] = struct{}{}
	}
	if high+1 < width {
		set[high+1] = struct{}{}
	}
	return nil
}
func (f *bvSliceFrame) align(xs, ys []ast.ASTHandle) error {
	i, j := len(xs), len(ys)
	var ox, oy uint32
	for i > 0 && j > 0 {
		x, y := xs[i-1], ys[j-1]
		wx, e := f.owner.manager.BitVectorSize(x)
		if e != nil {
			return e
		}
		wy, e := f.owner.manager.BitVectorSize(y)
		if e != nil {
			return e
		}
		rx, ry := wx-ox, wy-oy
		switch {
		case rx == ry:
			if e = f.register(ox, wx-1, x); e != nil {
				return e
			}
			if e = f.register(oy, wy-1, y); e != nil {
				return e
			}
			i--
			j--
			ox = 0
			oy = 0
		case rx < ry:
			if e = f.register(ox, wx-1, x); e != nil {
				return e
			}
			if e = f.register(oy, oy+rx-1, y); e != nil {
				return e
			}
			i--
			ox = 0
			oy += rx
		default:
			if e = f.register(oy, wy-1, y); e != nil {
				return e
			}
			if e = f.register(ox, ox+ry-1, x); e != nil {
				return e
			}
			j--
			oy = 0
			ox += ry
		}
	}
	if i != 0 || j != 0 {
		return ast.ErrASTDomain
	}
	return nil
}

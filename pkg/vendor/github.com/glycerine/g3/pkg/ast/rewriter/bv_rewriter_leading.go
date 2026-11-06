package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:1409-1446.

import "github.com/glycerine/g3/pkg/ast"

func (p *BVRewriter) LeadingZeroBits(arg ast.ASTHandle) (result uint32, err error) {
	if e := p.open(); e != nil {
		return 0, e
	}
	p.running = true
	defer func() { p.running = false; p.fail(err) }()
	return p.leadingZeroBits(arg)
}
func (p *BVRewriter) leadingZeroBits(arg ast.ASTHandle) (uint32, error) {
	m := p.manager
	parent := p.leadingZero
	n, e := ast.NewBitVectorLeadingZeroNumerals(m, parent)
	// A refused child header must not erase the older recursive frame.
	if n != nil {
		p.leadingZero = n
	}
	if e != nil {
		return 0, e
	}
	size, e := m.BitVectorSize(arg)
	if e != nil {
		return 0, e
	}
	var ignoredWidth uint32
	numeral, e := n.Read(arg, &ignoredWidth)
	if e != nil {
		return 0, e
	}
	var result uint32
	if numeral {
		for {
			positive, e := n.IsPositive()
			if e != nil {
				return 0, e
			}
			if !positive {
				break
			}
			if ast.ASTSourceDebug && size == 0 {
				return 0, ast.ErrASTDomain
			}
			size--
			if e = n.Step(); e != nil {
				return 0, e
			}
		}
		result = size
	} else {
		concat, e := m.IsBitVectorOperation(arg, 37)
		if e != nil {
			return 0, e
		}
		if concat {
			first, e := m.GetArg(arg, 0)
			if e != nil {
				return 0, e
			}
			width, e := m.BitVectorSize(first)
			if e != nil {
				return 0, e
			}
			result, e = p.leadingZeroBits(first)
			if e != nil {
				return 0, e
			}
			if width == result {
				second, e := m.GetArg(arg, 1)
				if e != nil {
					return 0, e
				}
				extra, e := p.leadingZeroBits(second)
				if e != nil {
					return 0, e
				}
				result += extra
			}
		}
	}
	if e = n.SourceEnd(); e != nil {
		return 0, e
	}
	if e = n.Close(); e != nil {
		return 0, e
	}
	p.leadingZero = parent
	return result, nil
}
func (p *BVRewriter) IsMultiplyWithoutOverflow(arg ast.ASTHandle) (matched bool, err error) {
	if e := p.open(); e != nil {
		return false, e
	}
	p.running = true
	defer func() { p.running = false; p.fail(err) }()
	return p.isMultiplyWithoutOverflow(arg)
}
func (p *BVRewriter) isMultiplyWithoutOverflow(arg ast.ASTHandle) (bool, error) {
	m := p.manager
	yes, e := m.IsBitVectorOperation(arg, 6)
	if e != nil || !yes {
		return false, e
	}
	width, e := m.BitVectorSize(arg)
	if e != nil {
		return false, e
	}
	count, e := m.GetNumArgs(arg)
	if e != nil {
		return false, e
	}
	var sum uint32
	for i := uint32(0); i < count; i++ {
		x, e := m.GetArg(arg, i)
		if e != nil {
			return false, e
		}
		leading, e := p.leadingZeroBits(x)
		if e != nil {
			return false, e
		}
		sum += width - leading
	}
	return sum < width, nil
}
func (p *BVRewriter) IsAddWithoutOverflow(arg ast.ASTHandle) (matched bool, err error) {
	if e := p.open(); e != nil {
		return false, e
	}
	p.running = true
	defer func() { p.running = false; p.fail(err) }()
	return p.isAddWithoutOverflow(arg)
}
func (p *BVRewriter) isAddWithoutOverflow(arg ast.ASTHandle) (bool, error) {
	m := p.manager
	yes, e := m.IsBitVectorOperation(arg, 4)
	if e != nil || !yes {
		return false, e
	}
	count, e := m.GetNumArgs(arg)
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < count; i++ {
		x, e := m.GetArg(arg, i)
		if e != nil {
			return false, e
		}
		leading, e := p.leadingZeroBits(x)
		if e != nil || leading == 0 {
			return false, e
		}
	}
	return true, nil
}

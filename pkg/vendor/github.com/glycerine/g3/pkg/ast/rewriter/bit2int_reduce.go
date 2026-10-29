package rewriter

// Source: src/ast/rewriter/bit2int.cpp:109-183.

import "github.com/glycerine/g3/pkg/ast"

type bit2IntComparison uint8

const (
	bit2IntLT bit2IntComparison = iota
	bit2IntLE
	bit2IntEQ
)

func (b *Bit2Int) simplifiedResult(c *bit2IntCall, rewrite func(ast.ASTRef) (RewriteStatus, error), fallback func() (ast.ASTHandle, error)) (ast.ASTHandle, error) {
	out, err := b.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	status, err := rewrite(out)
	if err != nil {
		_ = b.manager.DelRef(&out)
		return ast.ASTHandle{}, err
	}
	if status != RewriteFailed {
		return c.adopt(out)
	}
	if err = b.manager.DelRef(&out); err != nil {
		return ast.ASTHandle{}, err
	}
	h, err := fallback()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.hold(h)
}

func (b *Bit2Int) simplifyZeroExtend(c *bit2IntCall, amount uint32, value ast.ASTHandle) (ast.ASTHandle, error) {
	return b.simplifiedResult(c, func(out ast.ASTRef) (RewriteStatus, error) {
		return b.bv.RewriteZeroExtend(amount, value, out)
	}, func() (ast.ASTHandle, error) {
		return b.manager.MkBitVectorZeroExtend(amount, value)
	})
}

func (b *Bit2Int) simplifyBVAdd(c *bit2IntCall, left, right ast.ASTHandle) (ast.ASTHandle, error) {
	args := ast.SliceArguments([]ast.ASTHandle{left, right})
	return b.simplifiedResult(c, func(out ast.ASTRef) (RewriteStatus, error) {
		return b.bv.RewriteAddition(args, out)
	}, func() (ast.ASTHandle, error) {
		return b.manager.MkBitVectorAdd(left, right)
	})
}

func (b *Bit2Int) simplifyBVMul(c *bit2IntCall, left, right ast.ASTHandle) (ast.ASTHandle, error) {
	args := ast.SliceArguments([]ast.ASTHandle{left, right})
	return b.simplifiedResult(c, func(out ast.ASTRef) (RewriteStatus, error) {
		return b.bv.RewriteMultiplication(args, out)
	}, func() (ast.ASTHandle, error) {
		return b.manager.MkBitVectorMul(left, right)
	})
}

func (b *Bit2Int) simplifyBVSub(c *bit2IntCall, left, right ast.ASTHandle) (ast.ASTHandle, error) {
	args := ast.SliceArguments([]ast.ASTHandle{left, right})
	return b.simplifiedResult(c, func(out ast.ASTRef) (RewriteStatus, error) {
		return b.bv.polynomial.RewriteSubtraction(args, out)
	}, func() (ast.ASTHandle, error) {
		return b.manager.MkBitVectorSub(left, right)
	})
}

func (b *Bit2Int) simplifyBVURem(c *bit2IntCall, left, right ast.ASTHandle) (ast.ASTHandle, error) {
	return b.simplifiedResult(c, func(out ast.ASTRef) (RewriteStatus, error) {
		return b.bv.RewriteUnsignedRemainder(left, right, out)
	}, func() (ast.ASTHandle, error) {
		return b.manager.MkBitVectorURem(left, right)
	})
}

func (b *Bit2Int) simplifyBVULE(c *bit2IntCall, left, right ast.ASTHandle) (ast.ASTHandle, error) {
	return b.simplifiedResult(c, func(out ast.ASTRef) (RewriteStatus, error) {
		return b.bv.RewriteULE(left, right, out)
	}, func() (ast.ASTHandle, error) {
		return b.manager.MkBitVectorULE(left, right)
	})
}

func (b *Bit2Int) holdCall(c *bit2IntCall, h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.hold(h)
}

func (b *Bit2Int) mkBV2Int(c *bit2IntCall, value ast.ASTHandle) (ast.ASTHandle, error) {
	return b.simplifiedResult(c, func(out ast.ASTRef) (RewriteStatus, error) {
		return b.bv.RewriteBV2Int(value, out)
	}, func() (ast.ASTHandle, error) {
		return b.manager.MkBitVectorToInt(value)
	})
}

func (b *Bit2Int) mkArithmetic(c *bit2IntCall, kind int32, args ...ast.ASTHandle) (ast.ASTHandle, error) {
	h, err := b.manager.MkFamilyApp(b.arithFamily, kind, nil, args)
	return b.holdCall(c, h, err)
}

func (b *Bit2Int) mkAdd(c *bit2IntCall, left, right ast.ASTHandle) (ast.ASTHandle, bool, error) {
	l, lok, err := b.extractBV(c, left)
	if err != nil || !lok || l.sign {
		return ast.ASTHandle{}, false, err
	}
	r, rok, err := b.extractBV(c, right)
	if err != nil || !rok || r.sign {
		return ast.ASTHandle{}, false, err
	}
	zero, err := b.manager.IsBitVectorZero(l.bits)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if zero {
		return right, true, nil
	}
	zero, err = b.manager.IsBitVectorZero(r.bits)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if zero {
		return left, true, nil
	}
	a, d, err := b.alignSizes(c, l.bits, r.bits)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	width, err := b.manager.BitVectorSize(a)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	a, err = b.alignSize(c, a, width+1)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	d, err = b.alignSize(c, d, width+1)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	sum, err := b.simplifyBVAdd(c, a, d)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	result, err := b.mkBV2Int(c, sum)
	return result, err == nil, err
}

func (b *Bit2Int) mkComp(c *bit2IntCall, mode bit2IntComparison, left, right ast.ASTHandle) (ast.ASTHandle, bool, error) {
	l, lok, err := b.extractBV(c, left)
	if err != nil || !lok || l.sign {
		return ast.ASTHandle{}, false, err
	}
	r, rok, err := b.extractBV(c, right)
	if err != nil || !rok || r.sign {
		return ast.ASTHandle{}, false, err
	}
	a, d, err := b.alignSizes(c, l.bits, r.bits)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	var result ast.ASTHandle
	switch mode {
	case bit2IntLT:
		le, err := b.simplifyBVULE(c, d, a)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		result, err = b.manager.MkNot(le)
		result, err = b.holdCall(c, result, err)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
	case bit2IntLE:
		result, err = b.simplifyBVULE(c, a, d)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
	case bit2IntEQ:
		result, err = b.manager.MkEq(a, d)
		result, err = b.holdCall(c, result, err)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
	default:
		return ast.ASTHandle{}, false, ast.ErrASTDomain
	}
	return result, true, nil
}

func (b *Bit2Int) mkMul(c *bit2IntCall, left, right ast.ASTHandle) (ast.ASTHandle, bool, error) {
	l, lok, err := b.extractBV(c, left)
	if err != nil || !lok {
		return ast.ASTHandle{}, false, err
	}
	r, rok, err := b.extractBV(c, right)
	if err != nil || !rok {
		return ast.ASTHandle{}, false, err
	}
	a, d, err := b.alignSizes(c, l.bits, r.bits)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	width, err := b.manager.BitVectorSize(a)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	a, err = b.alignSize(c, a, width*2)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	d, err = b.alignSize(c, d, width*2)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	product, err := b.simplifyBVMul(c, a, d)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	result, err := b.mkBV2Int(c, product)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if l.sign != r.sign {
		result, err = b.mkArithmetic(c, 8, result)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
	}
	return result, true, nil
}

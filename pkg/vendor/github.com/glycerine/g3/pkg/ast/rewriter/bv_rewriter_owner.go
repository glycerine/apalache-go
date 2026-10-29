package rewriter

// Source: src/ast/rewriter/bv_rewriter.h:158-170;
// src/ast/rewriter/bv_rewriter.cpp:26-51,2076-2079,2170-2205;
// src/ast/rewriter/bv_trailing.cpp:26-48,365-388.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// BVRewriter owns the derived BV object's persistent members. The polynomial
// base owns the expression map/configuration, the embedded extraction member
// owns its cached declaration, and trailing owns its original lazy caches.
// AST and parameter services are borrowed. Do not copy this owner.
type BVRewriter struct {
	self                  *BVRewriter
	manager               *ast.ASTManager
	trace                 *diagnostic.TraceTags
	context               *uparams.ParamsContext
	bvFamily, arithFamily ast.FamilyID
	polynomial            *BVPolynomialRewriter
	extract               MkExtractProc
	extraction            *bvExtractInvocation
	leConcat              *bvLEConcatInvocation
	leOverflow            *bvLEOverflowInvocation
	leq                   *bvLEInvocation
	propagation           *bvExtractPropagation
	signExtension         *bvSignExtensionInvocation
	trailing              *BVTrailing
	eqRemainder           *ast.BitVectorRemainderEqualityNumerals
	mkbv                  *ast.BitVectorMkBVNumerals
	ite                   *ast.BitVectorIteNumerals
	leadingZero           *ast.BitVectorLeadingZeroNumerals
	int2bv                *ast.BitVectorInt2BVNumerals
	extendedRotate        *ast.BitVectorExtendedRotateNumerals
	bv2int                *bv2IntInvocation
	signedModulo          *ast.BitVectorSignedModuloNumerals
	unsignedRemainder     *ast.BitVectorUnsignedRemainderNumerals
	uremArgs              [2]ast.ASTHandle
	scalarNumeral         ast.ASTBitVectorNumeralInvocation
	repeatArgs            []ast.ASTHandle
	bitwiseArgs           []ast.ASTHandle
	bvor                  *bvOrInvocation
	bvxor                 *bvXorInvocation
	extensionPair         [2]ast.ASTHandle
	rotationArgs          [2]ast.ASTHandle
	minusOne              *ast.BitVectorMinusOneNumerals
	negatable             *ast.BitVectorNegatableNumerals
	bvnot                 *bvNotInvocation
	unsignedDivision      *ast.BitVectorUnsignedDivisionNumerals
	signedDivision        *ast.BitVectorSignedDivisionNumerals
	signedRemainder       *ast.BitVectorSignedRemainderNumerals
	arithmeticShift       *ast.BitVectorArithmeticShiftNumerals
	shift                 *ast.BitVectorLogicalShiftNumerals
	shiftArgs             [2]ast.ASTHandle
	concat                *bvConcatInvocation
	mulBound              *ast.BitVectorMultiplyBoundNumerals
	eqRefs                [2]ast.ASTRef
	defaultParams         uparams.ParamsRef
	defaultDecrement      uparams.ParamsDecrementReceipt
	defaultPlaced         bool
	header                uint64
	running, closed       bool
	cause                 error
}

func NewBVRewriter(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParams, local *uparams.ParamsRef) (*BVRewriter, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	if c == nil || g == nil || g.ContextField != c {
		return nil, uparams.ErrParamsOwner
	}
	if local != nil {
		if e := c.Check(local); e != nil {
			return nil, e
		}
	}
	bytes := uint64(reflect.TypeOf(BVRewriter{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	p := &BVRewriter{manager: m, context: c, header: bytes}
	p.self = p
	if local == nil {
		v, e := c.NewValue()
		if e != nil {
			return p, p.fail(e)
		}
		p.defaultParams = v
		p.defaultPlaced = true
		local = &p.defaultParams
	}
	var e error
	p.bvFamily, e = m.BitVectorUtilityFamilyID()
	if e != nil {
		return p, p.fail(e)
	}
	p.polynomial, e = NewBVPolynomialRewriter(m, c, g, local)
	if e != nil {
		return p, p.fail(e)
	}
	// Source base's two parameter updates precede each derived member.
	p.extract.constructSource(m)
	p.trailing, e = NewBVTrailing(&p.extract)
	if e != nil {
		return p, p.fail(e)
	}
	// arith_util creates its family identity and borrows the manager, with
	// an initially null lazy declaration-plugin pointer.
	p.arithFamily, e = m.ArithmeticFamilyID()
	if e != nil {
		return p, p.fail(e)
	}
	if e = p.polynomial.config.UpdateBitVectorLocal(local); e != nil {
		return p, p.fail(e)
	}
	if e = p.endDefault(c); e != nil {
		return p, p.fail(e)
	}
	return p, nil
}
func (p *BVRewriter) open() error {
	if p == nil || p.self != p || p.closed {
		return ast.ErrASTHandle
	}
	if p.cause != nil {
		return p.cause
	}
	if p.running {
		return ast.ErrASTPending
	}
	return nil
}
func (p *BVRewriter) fail(e error) error {
	if e != nil && p.cause == nil {
		p.cause = e
	}
	return e
}
func (p *BVRewriter) endDefault(c *uparams.ParamsContext) error {
	if !p.defaultPlaced {
		return nil
	}
	var e error
	if p.defaultDecrement.Invocation() != 0 {
		e = c.CompleteDecrement(&p.defaultParams, p.defaultDecrement)
	} else {
		e = c.DelTracked(&p.defaultParams, &p.defaultDecrement)
	}
	if e == nil {
		p.defaultPlaced = false
		p.defaultDecrement = uparams.ParamsDecrementReceipt{}
	}
	return e
}
func (p *BVRewriter) State() (BVRewriterConfigurationState, error) {
	if e := p.open(); e != nil {
		return BVRewriterConfigurationState{}, e
	}
	return p.polynomial.config.BitVectorState()
}
func (p *BVRewriter) Update(local *uparams.ParamsRef) error {
	if e := p.open(); e != nil {
		return e
	}
	if e := p.polynomial.config.UpdateBitVector(local); e != nil {
		return p.fail(e)
	}
	return nil
}
func (p *BVRewriter) SetMkBVToNumeral(v bool) error {
	if e := p.open(); e != nil {
		return e
	}
	return p.polynomial.config.SetMkBVToNumeral(v)
}
func (p *BVRewriter) RewriteAddition(args ast.AppArguments, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	defer func() { p.running = false; p.fail(err) }()
	return p.polynomial.RewriteBitVectorAddition(args, out)
}
func (p *BVRewriter) RewriteMultiplication(args ast.AppArguments, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	defer func() { p.running = false; p.fail(err) }()
	return p.polynomial.RewriteBitVectorMultiplication(args, out, p.polynomial.config.bitVector.MultiplyToConcat, &p.extract)
}

func (p *BVRewriter) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if p.running {
		return ast.ErrASTPending
	}
	// Stopped invocation frames are younger than persistent derived members.
	if p.polynomial != nil {
		if e := p.polynomial.closeInvocations(); e != nil {
			return e
		}
	}
	if e := p.manager.CleanupBitVectorNumeralInvocation(&p.scalarNumeral); e != nil {
		return e
	}
	clear(p.repeatArgs)
	p.repeatArgs = nil
	clear(p.bitwiseArgs)
	p.bitwiseArgs = nil
	if p.leOverflow != nil {
		if e := p.leOverflow.close(p.manager); e != nil {
			return e
		}
		p.leOverflow = nil
	}
	if p.leConcat != nil {
		if e := p.leConcat.close(p.manager); e != nil {
			return e
		}
		p.leConcat = nil
	}
	if p.leq != nil {
		if e := p.leq.close(p.manager); e != nil {
			return e
		}
		p.leq = nil
	}
	if p.signExtension != nil {
		if e := p.signExtension.close(p.manager); e != nil {
			return e
		}
		p.signExtension = nil
	}
	if p.propagation != nil {
		if e := p.propagation.close(p.manager); e != nil {
			return e
		}
		p.propagation = nil
	}
	if p.extraction != nil {
		if e := p.extraction.close(p.manager); e != nil {
			return e
		}
		p.extraction = nil
	}
	if p.bvxor != nil {
		if e := p.bvxor.close(p.manager); e != nil {
			return e
		}
		p.bvxor = nil
	}
	if p.bvor != nil {
		if e := p.bvor.close(p.manager); e != nil {
			return e
		}
		p.bvor = nil
	}
	p.extensionPair = [2]ast.ASTHandle{}
	p.rotationArgs = [2]ast.ASTHandle{}
	if p.extendedRotate != nil {
		if e := p.extendedRotate.Close(); e != nil {
			return e
		}
		p.extendedRotate = nil
	}
	if p.negatable != nil {
		if e := p.negatable.Close(); e != nil {
			return e
		}
		p.negatable = nil
	}
	if p.bvnot != nil {
		if e := p.bvnot.close(p.manager); e != nil {
			return e
		}
		p.bvnot = nil
	}
	if p.minusOne != nil {
		if e := p.minusOne.Close(); e != nil {
			return e
		}
		p.minusOne = nil
	}
	for p.leadingZero != nil {
		n := p.leadingZero
		if e := n.Close(); e != nil {
			return e
		}
		p.leadingZero = n.Parent()
	}
	if p.bv2int != nil {
		if e := p.bv2int.close(p.manager); e != nil {
			return e
		}
		p.bv2int = nil
	}
	if p.int2bv != nil {
		if e := p.int2bv.Close(); e != nil {
			return e
		}
		p.int2bv = nil
	}
	if p.signedModulo != nil {
		if e := p.signedModulo.Close(); e != nil {
			return e
		}
		p.signedModulo = nil
	}
	if p.unsignedRemainder != nil {
		if e := p.unsignedRemainder.Close(); e != nil {
			return e
		}
		p.unsignedRemainder = nil
	}
	if p.unsignedDivision != nil {
		if e := p.unsignedDivision.Close(); e != nil {
			return e
		}
		p.unsignedDivision = nil
	}
	if p.signedDivision != nil {
		if e := p.signedDivision.Close(); e != nil {
			return e
		}
		p.signedDivision = nil
	}
	if p.signedRemainder != nil {
		if e := p.signedRemainder.Close(); e != nil {
			return e
		}
		p.signedRemainder = nil
	}
	if p.arithmeticShift != nil {
		if e := p.arithmeticShift.Close(); e != nil {
			return e
		}
		p.arithmeticShift = nil
	}
	if p.shift != nil {
		if e := p.shift.Close(); e != nil {
			return e
		}
		p.shift = nil
	}
	if p.concat != nil {
		if e := p.concat.close(p.manager); e != nil {
			return e
		}
		p.concat = nil
	}
	if p.mulBound != nil {
		if e := p.mulBound.Close(); e != nil {
			return e
		}
		p.mulBound = nil
	}
	if p.ite != nil {
		if e := p.ite.Close(); e != nil {
			return e
		}
		p.ite = nil
	}
	if p.mkbv != nil {
		if e := p.mkbv.Close(); e != nil {
			return e
		}
		p.mkbv = nil
	}
	if p.eqRemainder != nil {
		if e := p.eqRemainder.Close(); e != nil {
			return e
		}
		p.eqRemainder = nil
	}
	if e := p.endEqualityRefs(); e != nil {
		return e
	}
	if p.trailing != nil {
		if e := p.trailing.Close(); e != nil {
			return e
		}
		p.trailing = nil
	}
	if p.extract.self != nil {
		if e := p.extract.Close(); e != nil {
			return e
		}
	}
	// The caller's default parameter full-expression outlives the constructor.
	// The outer owner retains its context even if base construction failed.
	if p.polynomial != nil {
		if e := p.polynomial.Close(); e != nil {
			return e
		}
		p.polynomial = nil
	}
	if e := p.endDefault(p.context); e != nil {
		return e
	}
	p.manager.Budget().Release(p.header)
	p.header = 0
	p.closed = true
	return nil
}

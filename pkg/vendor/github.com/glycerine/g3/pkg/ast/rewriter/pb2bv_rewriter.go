package rewriter

// Source: src/ast/rewriter/pb2bv_rewriter.cpp;
// src/ast/rewriter/pb2bv_rewriter.h.

import (
	"errors"
	"math/big"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type pb2BVConfiguration struct {
	DefaultConfiguration
	owner *PB2BVRewriter
}

func (c *pb2BVConfiguration) RewritePatterns() bool { return false }
func (c *pb2BVConfiguration) FlatAssoc(ast.ASTHandle) (bool, error) {
	return false, nil
}
func (c *pb2BVConfiguration) ReduceApp(decl ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (RewriteStatus, error) {
	if err := c.owner.manager.SetRef(proof, ast.ASTHandle{}); err != nil {
		return RewriteFailed, err
	}
	return c.owner.rewriteApp(decl, args, out)
}

// PB2BVRewriter owns the concrete pseudo-Boolean traversal, sorting network,
// auxiliary trail, declarations and side constraints. The AST manager is
// borrowed. Do not copy this owner.
type PB2BVRewriter struct {
	paramsContext       *uparams.ParamsContext
	parameters          uparams.ParamsRef
	parametersDecrement uparams.ParamsDecrementReceipt
	cleanupParameters   uparams.ParamsRef
	cleanupDecrement    uparams.ParamsDecrementReceipt
	closing             bool
	self                *PB2BVRewriter
	manager             *ast.ASTManager
	pseudo              *PseudoBooleanRewriter
	arithFamily         ast.FamilyID
	runtime             *umemory.Runtime
	context             *ast.PsortASTContext
	network             *ast.PsortASTNetwork
	configuration       pb2BVConfiguration
	traversal           *Traversal
	fresh               []ast.ASTHandle // owning declaration references
	freshLimits         []uint32
	freshSeen           uint32
	numTranslated       uint32
	header              uint64
	closed              bool
}

type PB2BVStatistics struct {
	AuxiliaryVariables uint32
	AuxiliaryClauses   uint32
}

// NewPB2BVRewriter constructs the source implementation graph. A nonnil
// partial owner returned with an error must be closed.
func NewPB2BVRewriter(m *ast.ASTManager, c *uparams.ParamsContext, p *uparams.ParamsRef) (*PB2BVRewriter, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if c == nil || c.SymbolsField != m.Symbols || c.NumeralsField != m.NumeralService() || c.BudgetField != m.Budget() {
		return nil, uparams.ErrParamsOwner
	}
	n := uint64(reflect.TypeOf(PB2BVRewriter{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	r := &PB2BVRewriter{manager: m, paramsContext: c, header: n, closing: true}
	r.self = r
	if err := r.constructMembers(p); err != nil {
		return r, err
	}
	return r, nil
}

func (r *PB2BVRewriter) constructMembers(p *uparams.ParamsRef) (err error) {
	r.parameters, err = r.paramsContext.NewValue()
	if err != nil {
		return err
	}
	if err = r.paramsContext.Assign(&r.parameters, p); err != nil {
		return err
	}
	m := r.manager
	if r.pseudo, err = NewPseudoBooleanRewriter(m); err != nil {
		return err
	}
	if r.arithFamily, err = m.ArithmeticFamilyID(); err != nil {
		return err
	}
	if r.runtime, err = umemory.NewRuntime(umemory.RuntimeConfig{Budget: m.Budget()}); err != nil {
		return err
	}
	if r.context, err = ast.NewPsortASTContext(m); err != nil {
		return err
	}
	if err = r.context.SetFreshPrefix("sn"); err != nil {
		return err
	}
	if r.network, err = ast.NewPsortASTNetwork(r.context, r.runtime, m.Budget(), nil); err != nil {
		return err
	}
	r.configuration = pb2BVConfiguration{owner: r}
	if r.traversal, err = NewTraversal(m, &r.configuration); err != nil {
		return err
	}
	r.closing = false
	return nil
}

func (r *PB2BVRewriter) open() error {
	if r == nil || r.self != r || r.closed || r.closing || r.manager == nil || r.traversal == nil {
		return ast.ErrASTHandle
	}
	_, err := r.manager.ProofsEnabled()
	return err
}

func pb2BVUnsigned(q *big.Rat) (uint32, bool) {
	if q == nil || q.Sign() < 0 || !q.IsInt() || !q.Num().IsUint64() {
		return 0, false
	}
	v := q.Num().Uint64()
	if v > uint64(^uint32(0)) {
		return 0, false
	}
	return uint32(v), true
}

type pb2BVComparison uint8

const (
	pb2BVLE pb2BVComparison = iota
	pb2BVEQ
	pb2BVGE
)

func (r *PB2BVRewriter) bitVectorNumeral(q *big.Rat, width uint32) (result ast.ASTHandle, err error) {
	if q == nil || !q.IsInt() || q.Sign() < 0 || width == 0 {
		return result, ast.ErrASTDomain
	}
	sort, err := r.manager.BitVectorSort(width)
	if err != nil {
		return result, err
	}
	service := r.manager.NumeralService()
	value, err := service.NewValue()
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, service.Del(&value)) }()
	if err = service.SetString(&value, q.Num().String()); err != nil {
		return result, err
	}
	return r.manager.BitVectorNumeralForSort(&value, sort)
}

func (r *PB2BVRewriter) negate(h ast.ASTHandle) (ast.ASTHandle, error) {
	yes, child, err := r.manager.IsNot(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if yes {
		return child, nil
	}
	return r.manager.MkNot(h)
}

func (r *PB2BVRewriter) simplifiedITE(condition, high, low ast.ASTHandle) (ast.ASTHandle, error) {
	for {
		negated, child, err := r.manager.IsNot(condition)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if !negated {
			break
		}
		condition, high, low = child, low, high
	}
	if high == low {
		return high, nil
	}
	switch {
	case r.manager.IsTrue(high) && r.manager.IsFalse(low):
		return condition, nil
	case r.manager.IsFalse(high) && r.manager.IsTrue(low):
		return r.negate(condition)
	case r.manager.IsTrue(high):
		return r.manager.MkOr(condition, low)
	case r.manager.IsFalse(low):
		return r.manager.MkAnd(condition, high)
	case r.manager.IsFalse(high):
		notCondition, err := r.negate(condition)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return r.manager.MkAnd(notCondition, low)
	case r.manager.IsTrue(low):
		return r.manager.MkImplies(condition, high)
	default:
		return r.manager.MkIte(condition, high, low)
	}
}

func (r *PB2BVRewriter) weightedPair(mode pb2BVComparison, formulas *[]ast.ASTHandle, left, right, bound ast.ASTHandle, width uint32) (ast.ASTHandle, error) {
	x, err := r.manager.MkBitVectorZeroExtend(1, left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	y, err := r.manager.MkBitVectorZeroExtend(1, right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sum, err := r.manager.MkBitVectorAdd(x, y)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	overflow, err := r.manager.MkBitVectorExtract(width, width, sum)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	low, err := r.manager.MkBitVectorExtract(width-1, 0, sum)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bit := uint32(0)
	if mode == pb2BVGE {
		bit = 1
	}
	want, err := r.manager.BitVectorNumeralUnsigned(bit, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	equal, err := r.manager.MkEq(overflow, want)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	*formulas = append(*formulas, equal)
	var bounded ast.ASTHandle
	if mode == pb2BVGE {
		bounded, err = r.manager.MkBitVectorULE(bound, low)
	} else {
		bounded, err = r.manager.MkBitVectorULE(low, bound)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	*formulas = append(*formulas, bounded)
	return low, nil
}

func (r *PB2BVRewriter) weighted(mode pb2BVComparison, args []ast.ASTHandle, coefficients []*big.Rat, boundValue *big.Rat) (ast.ASTHandle, error) {
	if boundValue.Sign() == 0 {
		if mode == pb2BVGE {
			return r.manager.MkTrue()
		}
		disjunction, err := r.manager.MkOrArray(args)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return r.negate(disjunction)
	}
	if boundValue.Sign() < 0 {
		return r.manager.MkBoolVal(mode == pb2BVGE)
	}
	if !boundValue.IsInt() || boundValue.Num().BitLen() > int(^uint32(0)) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	width := uint32(boundValue.Num().BitLen())
	zero, err := r.bitVectorNumeral(big.NewRat(0, 1), width)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bound, err := r.bitVectorNumeral(boundValue, width)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var expressions, formulas []ast.ASTHandle
	for i, coefficient := range coefficients {
		if coefficient.Sign() < 0 || !coefficient.IsInt() {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		if coefficient.Cmp(boundValue) > 0 {
			term := args[i]
			if mode != pb2BVGE {
				term, err = r.negate(term)
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
			formulas = append(formulas, term)
			continue
		}
		value, err := r.bitVectorNumeral(coefficient, width)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		term, err := r.simplifiedITE(args[i], value, zero)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		expressions = append(expressions, term)
	}
	for len(expressions) > 1 {
		next := expressions[:0]
		for i := 0; i+1 < len(expressions); i += 2 {
			sum, err := r.weightedPair(mode, &formulas, expressions[i], expressions[i+1], bound, width)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			next = append(next, sum)
		}
		if len(expressions)%2 != 0 {
			next = append(next, expressions[len(expressions)-1])
		}
		expressions = next
	}
	switch mode {
	case pb2BVLE:
		return r.manager.MkAndArray(formulas)
	case pb2BVGE:
		if len(expressions) != 0 {
			last, err := r.manager.MkBitVectorULE(bound, expressions[0])
			if err != nil {
				return ast.ASTHandle{}, err
			}
			formulas = append(formulas, last)
		}
		return r.manager.MkOrArray(formulas)
	case pb2BVEQ:
		if len(expressions) == 0 {
			falsity, err := r.manager.MkBoolVal(boundValue.Sign() == 0)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			formulas = append(formulas, falsity)
		} else {
			equal, err := r.manager.MkEq(bound, expressions[0])
			if err != nil {
				return ast.ASTHandle{}, err
			}
			formulas = append(formulas, equal)
		}
		return r.manager.MkAndArray(formulas)
	default:
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
}

func (r *PB2BVRewriter) weightedPB(kind int32, decl ast.ASTHandle, arguments ast.AppArguments, bound *big.Rat) (ast.ASTHandle, error) {
	args := make([]ast.ASTHandle, arguments.Count())
	coefficients := make([]*big.Rat, arguments.Count())
	for i := range args {
		var err error
		args[i], err = arguments.At(uint32(i))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		coefficients[i], err = r.pseudo.coefficient(decl, uint32(i))
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	mode := pb2BVLE
	if kind == ast.ASTPBOpEQ {
		mode = pb2BVEQ
	} else if kind == ast.ASTPBOpAtLeast || kind == ast.ASTPBOpGE {
		// Source dualizes >= into <= over negated literals.
		dual := new(big.Rat).Neg(bound)
		for i := range args {
			dual.Add(dual, coefficients[i])
			var err error
			args[i], err = r.negate(args[i])
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		bound = dual
	}
	return r.weighted(mode, args, coefficients, bound)
}

type pb2BVArithmeticForm struct {
	args         []ast.ASTHandle
	coefficients []*big.Rat
	bound        big.Rat
}

func (r *PB2BVRewriter) arithmeticNumeral(h ast.ASTHandle) (*big.Rat, bool, error) {
	text, _, matched, err := r.manager.ArithmeticNumeralText(h)
	if err != nil || !matched {
		return nil, matched, err
	}
	q, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, true, ast.ErrASTDomain
	}
	return q, true, nil
}

func (r *PB2BVRewriter) collectArithmeticPB(form *pb2BVArithmeticForm, expression ast.ASTHandle, multiplier *big.Rat) (bool, error) {
	kind, err := r.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := r.manager.GetFamilyID(expression)
	if err != nil {
		return false, err
	}
	declKind, err := r.manager.GetDeclKind(expression)
	if err != nil {
		return false, err
	}
	count, err := r.manager.GetNumArgs(expression)
	if err != nil {
		return false, err
	}
	if family == r.arithFamily {
		switch declKind {
		case arithOpAdd:
			for i := uint32(0); i < count; i++ {
				arg, e := r.manager.GetArg(expression, i)
				if e != nil {
					return false, e
				}
				matched, e := r.collectArithmeticPB(form, arg, multiplier)
				if e != nil || !matched {
					return matched, e
				}
			}
			return true, nil
		case arithOpSub:
			if count == 0 {
				return false, nil
			}
			first, e := r.manager.GetArg(expression, 0)
			if e != nil {
				return false, e
			}
			matched, e := r.collectArithmeticPB(form, first, multiplier)
			if e != nil || !matched {
				return matched, e
			}
			negative := new(big.Rat).Neg(multiplier)
			for i := uint32(1); i < count; i++ {
				// The source repeatedly reads argument 1 for n-ary subtraction.
				arg, e := r.manager.GetArg(expression, 1)
				if e != nil {
					return false, e
				}
				matched, e = r.collectArithmeticPB(form, arg, negative)
				if e != nil || !matched {
					return matched, e
				}
			}
			return true, nil
		case arithOpUMinus:
			if count != 1 {
				return false, nil
			}
			arg, e := r.manager.GetArg(expression, 0)
			if e != nil {
				return false, e
			}
			return r.collectArithmeticPB(form, arg, new(big.Rat).Neg(multiplier))
		case arithOpNum:
			value, matched, e := r.arithmeticNumeral(expression)
			if e != nil || !matched {
				return matched, e
			}
			form.bound.Sub(&form.bound, new(big.Rat).Mul(multiplier, value))
			return true, nil
		case arithOpMul:
			if count != 2 {
				return false, nil
			}
			left, e := r.manager.GetArg(expression, 0)
			if e != nil {
				return false, e
			}
			right, e := r.manager.GetArg(expression, 1)
			if e != nil {
				return false, e
			}
			if value, matched, e := r.arithmeticNumeral(left); e != nil {
				return false, e
			} else if matched {
				return r.collectArithmeticPB(form, right, new(big.Rat).Mul(value, multiplier))
			}
			if value, matched, e := r.arithmeticNumeral(right); e != nil {
				return false, e
			} else if matched {
				return r.collectArithmeticPB(form, left, new(big.Rat).Mul(value, multiplier))
			}
			return false, nil
		default:
			return false, nil
		}
	}
	if family != r.manager.BasicFamilyID() || declKind != ast.ASTOpIte || count != 3 {
		return false, nil
	}
	condition, err := r.manager.GetArg(expression, 0)
	if err != nil {
		return false, err
	}
	high, err := r.manager.GetArg(expression, 1)
	if err != nil {
		return false, err
	}
	low, err := r.manager.GetArg(expression, 2)
	if err != nil {
		return false, err
	}
	highValue, highMatched, err := r.arithmeticNumeral(high)
	if err != nil || !highMatched {
		return false, err
	}
	lowValue, lowMatched, err := r.arithmeticNumeral(low)
	if err != nil || !lowMatched {
		return false, err
	}
	highValue.Mul(highValue, multiplier)
	lowValue.Mul(lowValue, multiplier)
	if highValue.Cmp(lowValue) < 0 {
		condition, err = r.negate(condition)
		if err != nil {
			return false, err
		}
		form.args = append(form.args, condition)
		form.coefficients = append(form.coefficients, new(big.Rat).Sub(lowValue, highValue))
		form.bound.Sub(&form.bound, highValue)
	} else {
		form.args = append(form.args, condition)
		form.coefficients = append(form.coefficients, new(big.Rat).Sub(highValue, lowValue))
		form.bound.Sub(&form.bound, lowValue)
	}
	return true, nil
}

func (r *PB2BVRewriter) arithmeticPB(left, right ast.ASTHandle) (*pb2BVArithmeticForm, bool, error) {
	form := &pb2BVArithmeticForm{}
	matched, err := r.collectArithmeticPB(form, left, big.NewRat(1, 1))
	if err != nil || !matched {
		return nil, false, err
	}
	matched, err = r.collectArithmeticPB(form, right, big.NewRat(-1, 1))
	if err != nil || !matched {
		return nil, false, err
	}
	return form, true, nil
}

func (r *PB2BVRewriter) rewriteArithmeticComparison(kind int32, args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	if args.Count() != 2 {
		return RewriteFailed, nil
	}
	left, err := args.At(0)
	if err != nil {
		return RewriteFailed, err
	}
	right, err := args.At(1)
	if err != nil {
		return RewriteFailed, err
	}
	mode := pb2BVLE
	if kind == arithOpGE || kind == arithOpGT {
		left, right = right, left
	}
	form, matched, err := r.arithmeticPB(left, right)
	if err != nil || !matched {
		return RewriteFailed, err
	}
	if kind == arithOpLT || kind == arithOpGT {
		form.bound.Add(&form.bound, big.NewRat(1, 1))
	}
	result, err := r.weighted(mode, form.args, form.coefficients, &form.bound)
	if err != nil {
		return RewriteFailed, err
	}
	r.numTranslated++
	return RewriteDone, r.manager.SetRef(out, result)
}

func (r *PB2BVRewriter) rewriteArithmeticEquality(args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	if args.Count() != 2 {
		return RewriteFailed, nil
	}
	left, err := args.At(0)
	if err != nil {
		return RewriteFailed, err
	}
	right, err := args.At(1)
	if err != nil {
		return RewriteFailed, err
	}
	form, matched, err := r.arithmeticPB(left, right)
	if err != nil || !matched {
		return RewriteFailed, err
	}
	result, err := r.weighted(pb2BVEQ, form.args, form.coefficients, &form.bound)
	if err != nil {
		return RewriteFailed, err
	}
	r.numTranslated++
	return RewriteDone, r.manager.SetRef(out, result)
}

func (r *PB2BVRewriter) syncFresh() error {
	n, err := r.context.FreshCount()
	if err != nil {
		return err
	}
	for r.freshSeen < n {
		app, err := r.context.FreshApplication(r.freshSeen)
		if err != nil {
			return err
		}
		decl, err := r.manager.GetDecl(app)
		if err != nil {
			return err
		}
		if err = r.manager.IncRef(decl); err != nil {
			return err
		}
		r.fresh = append(r.fresh, decl)
		r.freshSeen++
	}
	return nil
}

func (r *PB2BVRewriter) cardinality(full bool, kind int32, k uint32, args ast.AppArguments) (result ast.ASTHandle, err error) {
	v, err := ast.NewASTLiteralVector(r.manager.Budget())
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, v.Close()) }()
	for i := uint32(0); i < args.Count(); i++ {
		h, e := args.At(i)
		if e != nil {
			return result, e
		}
		if e = v.Push(h); e != nil {
			return result, e
		}
	}
	span, err := v.Span()
	if err != nil {
		return result, err
	}
	switch kind {
	case ast.ASTPBOpAtMost, ast.ASTPBOpLE:
		result, err = r.network.Le(full, k, args.Count(), span)
	case ast.ASTPBOpAtLeast, ast.ASTPBOpGE:
		result, err = r.network.Ge(full, k, args.Count(), span)
	case ast.ASTPBOpEQ:
		result, err = r.network.Eq(full, k, args.Count(), span)
	default:
		return result, ast.ErrASTDomain
	}
	if err == nil {
		err = r.syncFresh()
	}
	return result, err
}

func (r *PB2BVRewriter) rewriteApp(decl ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	family, err := r.manager.GetFamilyID(decl)
	if err != nil {
		return RewriteFailed, err
	}
	kind, err := r.manager.GetDeclKind(decl)
	if err != nil {
		return RewriteFailed, err
	}
	if family == r.arithFamily {
		switch kind {
		case arithOpLE, arithOpLT, arithOpGE, arithOpGT:
			return r.rewriteArithmeticComparison(kind, args, out)
		default:
			return RewriteFailed, nil
		}
	}
	if family == r.manager.BasicFamilyID() && kind == ast.ASTOpEq {
		return r.rewriteArithmeticEquality(args, out)
	}
	if family != r.pseudo.family || kind < ast.ASTPBOpAtMost || kind > ast.ASTPBOpEQ {
		return RewriteFailed, nil
	}
	bound, err := r.pseudo.bound(decl)
	if err != nil {
		return RewriteFailed, err
	}
	k, unsigned := pb2BVUnsigned(bound)
	if (kind == ast.ASTPBOpAtLeast || kind == ast.ASTPBOpGE) && bound.Cmp(big.NewRat(1, 1)) == 0 {
		handles := make([]ast.ASTHandle, args.Count())
		for i := range handles {
			handles[i], err = args.At(uint32(i))
			if err != nil {
				return RewriteFailed, err
			}
		}
		result, err := r.manager.MkOrArgs(handles)
		if err != nil {
			return RewriteFailed, err
		}
		r.numTranslated++
		return RewriteDone, r.manager.SetRef(out, result)
	}
	unit := kind == ast.ASTPBOpAtMost || kind == ast.ASTPBOpAtLeast
	if !unit {
		unit, err = r.manager.PseudoBooleanHasUnitCoefficients(decl)
		if err != nil {
			return RewriteFailed, err
		}
	}
	if !unsigned || !unit {
		result, err := r.weightedPB(kind, decl, args, bound)
		if err != nil {
			return RewriteFailed, err
		}
		r.numTranslated++
		return RewriteDone, r.manager.SetRef(out, result)
	}
	result, err := r.cardinality(true, kind, k, args)
	if err != nil {
		return RewriteFailed, err
	}
	r.numTranslated++
	return RewriteDone, r.manager.SetRef(out, result)
}

// Rewrite converts reached cardinality constraints. The source traversal
// always disables proof generation and leaves the caller proof slot untouched.
func (r *PB2BVRewriter) Rewrite(expression ast.ASTHandle, out, _ ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.traversal.Rewrite(expression, out)
}

func (r *PB2BVRewriter) NumSteps() (uint32, error) {
	if err := r.open(); err != nil {
		return 0, err
	}
	return r.traversal.NumSteps()
}

func (r *PB2BVRewriter) Manager() (*ast.ASTManager, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	return r.manager, nil
}

// UpdateParams is intentionally empty, as in the source implementation.
func (r *PB2BVRewriter) UpdateParams(_ *uparams.ParamsRef) error { return r.open() }

func (r *PB2BVRewriter) NumTranslated() (uint32, error) {
	if err := r.open(); err != nil {
		return 0, err
	}
	return r.numTranslated, nil
}

// FreshConstants returns borrowed declaration handles retained by this owner.
func (r *PB2BVRewriter) FreshConstants() ([]ast.ASTHandle, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), r.fresh...), nil
}

// FlushSideConstraints transfers owning clause references to the caller.
func (r *PB2BVRewriter) FlushSideConstraints() ([]ast.ASTHandle, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	return r.context.TakeClauses()
}

func (r *PB2BVRewriter) Push() error {
	if err := r.open(); err != nil {
		return err
	}
	r.freshLimits = append(r.freshLimits, uint32(len(r.fresh)))
	return nil
}

func (r *PB2BVRewriter) Pop(scopes uint32) error {
	if err := r.open(); err != nil {
		return err
	}
	clauses, err := r.context.ClauseCount()
	if err != nil {
		return err
	}
	if clauses != 0 || scopes > uint32(len(r.freshLimits)) {
		return ast.ErrASTDomain
	}
	if scopes != 0 {
		newSize := uint32(len(r.freshLimits)) - scopes
		limit := r.freshLimits[newSize]
		for i := int(limit); i < len(r.fresh); i++ {
			if r.fresh[i].IsNull() {
				continue
			}
			consumed, e := r.manager.DecRefConsumed(r.fresh[i])
			if consumed {
				r.fresh[i] = ast.ASTHandle{}
			}
			if e != nil {
				r.closing = true
				return e
			}
			if !consumed {
				r.closing = true
				return ast.ErrASTPending
			}
		}
		r.fresh = r.fresh[:limit]
		r.freshLimits = r.freshLimits[:newSize]
	}
	return r.traversal.Reset()
}

func (r *PB2BVRewriter) Statistics() (ast.PsortASTStats, error) {
	if err := r.open(); err != nil {
		return ast.PsortASTStats{}, err
	}
	return r.network.Stats()
}

func (r *PB2BVRewriter) CollectStatistics() (PB2BVStatistics, error) {
	if err := r.open(); err != nil {
		return PB2BVStatistics{}, err
	}
	stats, err := r.network.Stats()
	if err != nil {
		return PB2BVStatistics{}, err
	}
	return PB2BVStatistics{AuxiliaryVariables: uint32(len(r.fresh)), AuxiliaryClauses: stats.CompiledClauses}, nil
}

func (r *PB2BVRewriter) destroyMembers() error {
	if r.traversal != nil {
		if err := r.traversal.Close(); err != nil {
			return err
		}
		r.traversal = nil
	}
	if r.network != nil {
		if err := r.network.Close(); err != nil {
			return err
		}
		r.network = nil
	}
	if r.context != nil {
		if err := r.context.Close(); err != nil {
			return err
		}
		r.context = nil
	}
	for i, h := range r.fresh {
		if h.IsNull() {
			continue
		}
		consumed, err := r.manager.DecRefConsumed(h)
		if consumed {
			r.fresh[i] = ast.ASTHandle{}
		}
		if err != nil {
			return err
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	r.fresh = nil
	r.freshLimits = nil
	if r.runtime != nil {
		if err := r.runtime.Close(); err != nil {
			return err
		}
		r.runtime = nil
	}
	if r.pseudo != nil {
		if err := r.pseudo.Close(); err != nil {
			return err
		}
		r.pseudo = nil
	}
	return r.endParameters(&r.parameters, &r.parametersDecrement)
}

// Cleanup reconstructs the complete source implementation while retaining the
// outer owner and borrowed manager.
func (r *PB2BVRewriter) Cleanup() error {
	if r == nil || r.self != r || r.closed {
		return ast.ErrASTHandle
	}
	r.closing = true
	if r.cleanupParameters == (uparams.ParamsRef{}) {
		var err error
		r.cleanupParameters, err = r.paramsContext.NewValue()
		if err != nil {
			return err
		}
		if err = r.paramsContext.Assign(&r.cleanupParameters, &r.parameters); err != nil {
			return errors.Join(err, r.endParameters(&r.cleanupParameters, &r.cleanupDecrement))
		}
	}
	if err := r.destroyMembers(); err != nil {
		return err
	}
	r.manager.Budget().Release(r.header)
	r.header = 0
	n := uint64(reflect.TypeOf(PB2BVRewriter{}).Size())
	if err := r.manager.Budget().Reserve(n); err != nil {
		return err
	}
	r.header = n
	r.freshSeen = 0
	r.numTranslated = 0
	r.arithFamily = ast.NullFamilyID
	if err := r.constructMembers(&r.cleanupParameters); err != nil {
		return err
	}
	r.closing = true
	if err := r.endParameters(&r.cleanupParameters, &r.cleanupDecrement); err != nil {
		return err
	}
	r.closing = false
	return nil
}

func (r *PB2BVRewriter) Close() error {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	r.closing = true
	if err := r.destroyMembers(); err != nil {
		return err
	}
	r.manager.Budget().Release(r.header)
	r.header = 0
	if err := r.endParameters(&r.cleanupParameters, &r.cleanupDecrement); err != nil {
		return err
	}
	r.closed = true
	return nil
}

func (r *PB2BVRewriter) endParameters(p *uparams.ParamsRef, receipt *uparams.ParamsDecrementReceipt) error {
	if *p == (uparams.ParamsRef{}) {
		return nil
	}
	var err error
	if receipt.Invocation() != 0 {
		err = r.paramsContext.CompleteDecrement(p, *receipt)
	} else {
		err = r.paramsContext.DelTracked(p, receipt)
	}
	if err == nil {
		*receipt = uparams.ParamsDecrementReceipt{}
	}
	return err
}

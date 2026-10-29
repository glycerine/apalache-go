package rewriter

// Source: src/ast/rewriter/th_rewriter.cpp;
// src/ast/rewriter/th_rewriter.h.

import (
	"errors"
	"math/big"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

const (
	theoryBVOpAdd int32 = 4
	theoryBVOpMul int32 = 6
)

type theoryRewriterConfiguration struct {
	DefaultConfiguration
	owner                *TheoryRewriter
	boolean              *BoolRewriter
	arithmetic           *ArithRewriter
	bitVector            *BVRewriter
	array                *ArrayRewriter
	datatype             *DatatypeRewriter
	floating             *FPARewriter
	datalog              *DLRewriter
	pseudoBoolean        *PseudoBooleanRewriter
	sequence             *SequenceRewriter
	maxMemory            uint64
	maxSteps             uint32
	pullCheapITE         bool
	flat                 bool
	cacheAll             bool
	pushITEArithmetic    bool
	pushITEBitVector     bool
	ignoreGroundPatterns bool
	rewritePatternTerms  bool
	substitution         ast.ASTExprSubstitution
	hasSubstitution      bool
	usedDependencies     ast.ASTDependency
}

func (c *theoryRewriterConfiguration) CacheAllResults() bool { return c.cacheAll }
func (c *theoryRewriterConfiguration) RewritePatterns() bool { return c.rewritePatternTerms }
func (c *theoryRewriterConfiguration) MaxStepsExceeded(steps uint32) (bool, error) {
	if c.owner.manager.Budget().Used() > c.maxMemory {
		return false, newRewriterException("max. memory exceeded")
	}
	return steps > c.maxSteps, nil
}
func (c *theoryRewriterConfiguration) Reset() {
	c.substitution = ast.ASTExprSubstitution{}
	c.hasSubstitution = false
}
func (c *theoryRewriterConfiguration) GetSubst(term ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	if !c.hasSubstitution {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
	}
	var value, proof ast.ASTHandle
	var dependency ast.ASTDependency
	found, err := c.owner.manager.ExprSubstitutionFindDependency(c.substitution, term, &value, &proof, &dependency)
	if err != nil || !found || dependency.IsNull() {
		return value, proof, found, err
	}
	joined, err := c.owner.manager.DependencyJoin(c.usedDependencies, dependency)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if err = c.owner.manager.DependencyInc(joined); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if !c.usedDependencies.IsNull() {
		if err = c.owner.manager.DependencyDec(c.usedDependencies); err != nil {
			_ = c.owner.manager.DependencyDec(joined)
			return ast.ASTHandle{}, ast.ASTHandle{}, false, err
		}
	}
	c.usedDependencies = joined
	return value, proof, true, nil
}
func (c *theoryRewriterConfiguration) FlatAssoc(decl ast.ASTHandle) (bool, error) {
	if !c.flat {
		return false, nil
	}
	family, err := c.owner.manager.GetFamilyID(decl)
	if err != nil || family == ast.NullFamilyID {
		return false, err
	}
	kind, err := c.owner.manager.GetDeclKind(decl)
	if err != nil {
		return false, err
	}
	if family == c.owner.manager.BasicFamilyID() {
		return kind == ast.ASTOpAnd || kind == ast.ASTOpOr, nil
	}
	if family == c.arithmetic.family {
		return kind == arithOpAdd, nil
	}
	if family == c.bitVector.bvFamily {
		return kind == 4 || kind == 31 || kind == 30 || kind == 33, nil
	}
	return false, nil
}

func theoryUniqueArguments(arguments ast.AppArguments) ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, 0, arguments.Count())
	seen := make(map[ast.ASTHandle]struct{}, arguments.Count())
	for i := uint32(0); i < arguments.Count(); i++ {
		value, err := arguments.At(i)
		if err != nil {
			return nil, err
		}
		if _, found := seen[value]; found {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func (c *theoryRewriterConfiguration) ReduceQuantifier(old, body ast.ASTHandle, patterns, noPatterns ast.AppArguments, out, proof ast.ASTRef) (result bool, err error) {
	m := c.owner.manager
	if err = m.SetRef(proof, ast.ASTHandle{}); err != nil {
		return false, err
	}
	oldForall, err := m.IsForall(old)
	if err != nil {
		return false, err
	}
	oldPatterns, err := m.GetNumPatterns(old)
	if err != nil {
		return false, err
	}
	oldNoPatterns, err := m.GetNumNoPatterns(old)
	if err != nil {
		return false, err
	}
	bodyKind, err := m.GetKind(body)
	if err != nil {
		return false, err
	}
	var updated ast.ASTHandle
	if bodyKind == ast.ASTQuantifier && oldPatterns == 0 && oldNoPatterns == 0 {
		innerForall, e := m.IsForall(body)
		if e != nil {
			return false, e
		}
		innerPatterns, e := m.GetNumPatterns(body)
		if e != nil {
			return false, e
		}
		innerNoPatterns, e := m.GetNumNoPatterns(body)
		if e != nil {
			return false, e
		}
		if innerForall == oldForall && innerPatterns == 0 && innerNoPatterns == 0 {
			outerCount, e := m.GetNumDeclarations(old)
			if e != nil {
				return false, e
			}
			innerCount, e := m.GetNumDeclarations(body)
			if e != nil {
				return false, e
			}
			sorts := make([]ast.ASTHandle, 0, outerCount+innerCount)
			names := make([]usymbol.Symbol, 0, outerCount+innerCount)
			for _, item := range []struct {
				quantifier ast.ASTHandle
				count      uint32
			}{{old, outerCount}, {body, innerCount}} {
				for i := uint32(0); i < item.count; i++ {
					sort, e := m.GetDeclarationSort(item.quantifier, i)
					if e != nil {
						return false, e
					}
					name, e := m.GetDeclarationName(item.quantifier, i)
					if e != nil {
						return false, e
					}
					sorts = append(sorts, sort)
					names = append(names, name)
				}
			}
			outerWeight, e := m.GetQuantifierWeight(old)
			if e != nil {
				return false, e
			}
			innerWeight, e := m.GetQuantifierWeight(body)
			if e != nil {
				return false, e
			}
			if innerWeight < outerWeight {
				outerWeight = innerWeight
			}
			qid, skid, e := m.GetQuantifierIDs(old)
			if e != nil {
				return false, e
			}
			innerBody, e := m.GetQuantifierBody(body)
			if e != nil {
				return false, e
			}
			updated, err = m.MkQuantifier(ast.ASTQuantifierSpec{Forall: oldForall, Sorts: sorts, Names: names,
				Body: innerBody, Weight: outerWeight, QID: qid, SkolemID: skid})
		}
	}
	if updated.IsNull() {
		uniquePatterns, e := theoryUniqueArguments(patterns)
		if e != nil {
			return false, e
		}
		uniqueNoPatterns, e := theoryUniqueArguments(noPatterns)
		if e != nil {
			return false, e
		}
		updated, err = m.UpdateQuantifierAnnotations(old, uniquePatterns, uniqueNoPatterns, body)
	}
	if err != nil {
		return false, err
	}
	updatedRef, err := m.NewRef(updated)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&updatedRef)) }()
	eliminated, err := elimUnusedVars(m, updated, c.ignoreGroundPatterns)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&eliminated)) }()
	value, err := m.RefGet(eliminated)
	if err != nil {
		return false, err
	}
	return true, m.SetRef(out, value)
}

func (c *theoryRewriterConfiguration) bitNumeral(h ast.ASTHandle) (uint32, bool, error) {
	service := c.owner.manager.NumeralService()
	value, err := service.NewValue()
	if err != nil {
		return 0, false, err
	}
	defer func() { _ = service.Del(&value) }()
	var width uint32
	matched, err := c.owner.manager.BitVectorNumeralValue(h, &value, &width)
	if err != nil || !matched {
		return 0, matched, err
	}
	if width != 1 {
		return 0, false, nil
	}
	valid, err := service.IsUint64(&value)
	if err != nil || !valid {
		return 0, false, err
	}
	v, err := service.Uint64(&value)
	if err != nil || v > 1 {
		return 0, false, err
	}
	return uint32(v), true, nil
}

func (c *theoryRewriterConfiguration) equalityBit(term ast.ASTHandle) (x ast.ASTHandle, value uint32, matched bool, err error) {
	kind, err := c.owner.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return x, 0, false, err
	}
	family, err := c.owner.manager.GetFamilyID(term)
	if err != nil || family != c.owner.manager.BasicFamilyID() {
		return x, 0, false, err
	}
	op, err := c.owner.manager.GetDeclKind(term)
	if err != nil || op != ast.ASTOpEq {
		return x, 0, false, err
	}
	if count, e := c.owner.manager.GetNumArgs(term); e != nil || count != 2 {
		return x, 0, false, e
	}
	left, err := c.owner.manager.GetArg(term, 0)
	if err != nil {
		return x, 0, false, err
	}
	isBV, err := c.owner.manager.IsBitVector(left)
	if err != nil || !isBV {
		return x, 0, false, err
	}
	width, err := c.owner.manager.BitVectorSize(left)
	if err != nil || width != 1 {
		return x, 0, false, err
	}
	right, err := c.owner.manager.GetArg(term, 1)
	if err != nil {
		return x, 0, false, err
	}
	if v, yes, e := c.bitNumeral(left); e != nil || yes {
		return right, v, yes, e
	}
	if v, yes, e := c.bitNumeral(right); e != nil || yes {
		return left, v, yes, e
	}
	return x, 0, false, nil
}

func (c *theoryRewriterConfiguration) tamagotchi(left, right ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	x, value, matched, err := c.equalityBit(left)
	condition := right
	if err != nil {
		return RewriteFailed, err
	}
	if !matched {
		x, value, matched, err = c.equalityBit(right)
		condition = left
		if err != nil || !matched {
			return RewriteFailed, err
		}
	}
	thenValue, err := c.owner.manager.BitVectorNumeralUnsigned(value, 1)
	if err != nil {
		return RewriteFailed, err
	}
	elseValue, err := c.owner.manager.BitVectorNumeralUnsigned(1-value, 1)
	if err != nil {
		return RewriteFailed, err
	}
	ite, err := c.owner.manager.MkIte(condition, thenValue, elseValue)
	if err != nil {
		return RewriteFailed, err
	}
	equality, err := c.owner.manager.MkEq(x, ite)
	if err != nil {
		return RewriteFailed, err
	}
	return Rewrite2, c.owner.manager.SetRef(out, equality)
}

func (c *theoryRewriterConfiguration) reduceAppCore(decl ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	m := c.owner.manager
	family, err := m.GetFamilyID(decl)
	if err != nil || family == ast.NullFamilyID {
		return RewriteFailed, err
	}
	if family == m.BasicFamilyID() {
		kind, err := m.GetDeclKind(decl)
		if err != nil {
			return RewriteFailed, err
		}
		if kind == ast.ASTOpEq {
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
			sort, err := m.GetSort(left)
			if err != nil {
				return RewriteFailed, err
			}
			sortFamily, err := m.GetFamilyID(sort)
			if err != nil {
				return RewriteFailed, err
			}
			status := RewriteFailed
			switch sortFamily {
			case c.arithmetic.family:
				status, err = c.arithmetic.MkEqCore(left, right, out)
			case c.bitVector.bvFamily:
				status, err = c.bitVector.RewriteEquality(left, right, out)
			case c.datatype.family:
				status, err = c.datatype.RewriteEquality(left, right, out)
			case c.floating.family:
				status, err = c.floating.MkEqCore(left, right, out)
			case c.sequence.family:
				status, err = c.sequence.RewriteEquality(left, right, out)
			}
			if err != nil || status != RewriteFailed {
				return status, err
			}
		}
		if kind == ast.ASTOpEq || kind == ast.ASTOpIff {
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
			if status, err := c.tamagotchi(left, right, out); err != nil || status != RewriteFailed {
				return status, err
			}
		}
		if kind == ast.ASTOpIte {
			if args.Count() != 3 {
				return RewriteFailed, ast.ErrASTDomain
			}
			condition, err := args.At(0)
			if err != nil {
				return RewriteFailed, err
			}
			thenValue, err := args.At(1)
			if err != nil {
				return RewriteFailed, err
			}
			elseValue, err := args.At(2)
			if err != nil {
				return RewriteFailed, err
			}
			sort, err := m.GetSort(thenValue)
			if err != nil {
				return RewriteFailed, err
			}
			sortFamily, err := m.GetFamilyID(sort)
			if err != nil {
				return RewriteFailed, err
			}
			if sortFamily == c.bitVector.bvFamily {
				if status, err := c.bitVector.RewriteIte(condition, thenValue, elseValue, out); err != nil || status != RewriteFailed {
					return status, err
				}
			}
		}
		return c.boolean.MkAppCore(decl, args, out)
	}
	switch family {
	case c.arithmetic.family:
		return c.arithmetic.MkAppCore(decl, args, out)
	case c.bitVector.bvFamily:
		return c.bitVector.RewriteApp(decl, args, out)
	case c.array.family:
		return c.array.MkAppCore(decl, args, out)
	case c.datatype.family:
		return c.datatype.RewriteApp(decl, args, out)
	case c.floating.family:
		return c.floating.MkAppCore(decl, args, out)
	case c.datalog.family:
		return c.datalog.RewriteApp(decl, args, out)
	case c.pseudoBoolean.family:
		return c.pseudoBoolean.RewriteApp(decl, args, out)
	case c.sequence.family:
		return c.sequence.RewriteApp(decl, args, out)
	default:
		return RewriteFailed, nil
	}
}

func (c *theoryRewriterConfiguration) iteView(term ast.ASTHandle) (condition, thenValue, elseValue ast.ASTHandle, matched bool, err error) {
	kind, err := c.owner.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return condition, thenValue, elseValue, false, err
	}
	family, err := c.owner.manager.GetFamilyID(term)
	if err != nil || family != c.owner.manager.BasicFamilyID() {
		return condition, thenValue, elseValue, false, err
	}
	op, err := c.owner.manager.GetDeclKind(term)
	if err != nil || op != ast.ASTOpIte {
		return condition, thenValue, elseValue, false, err
	}
	if count, e := c.owner.manager.GetNumArgs(term); e != nil || count != 3 {
		return condition, thenValue, elseValue, false, e
	}
	condition, err = c.owner.manager.GetArg(term, 0)
	if err == nil {
		thenValue, err = c.owner.manager.GetArg(term, 1)
	}
	if err == nil {
		elseValue, err = c.owner.manager.GetArg(term, 2)
	}
	return condition, thenValue, elseValue, err == nil, err
}

func (c *theoryRewriterConfiguration) equalityValue(left, value ast.ASTHandle) (ast.ASTHandle, error) {
	equal, err := c.owner.manager.AreEqual(left, value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if equal {
		return c.owner.manager.MkTrue()
	}
	distinct, err := c.owner.manager.AreDistinct(left, value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if distinct {
		return c.owner.manager.MkFalse()
	}
	return c.owner.manager.MkEq(left, value)
}

func (c *theoryRewriterConfiguration) pullITECore(decl, ite, value ast.ASTHandle, swap bool, out ast.ASTRef) (RewriteStatus, error) {
	condition, thenValue, elseValue, matched, err := c.iteView(ite)
	if err != nil || !matched {
		return RewriteFailed, err
	}
	family, err := c.owner.manager.GetFamilyID(decl)
	if err != nil {
		return RewriteFailed, err
	}
	kind, err := c.owner.manager.GetDeclKind(decl)
	if err != nil {
		return RewriteFailed, err
	}
	var high, low ast.ASTHandle
	if family == c.owner.manager.BasicFamilyID() && kind == ast.ASTOpEq {
		high, err = c.equalityValue(thenValue, value)
		if err == nil {
			low, err = c.equalityValue(elseValue, value)
		}
	} else {
		if swap {
			high, err = c.owner.manager.RewriterMakeApp(decl, ast.SliceArguments([]ast.ASTHandle{value, thenValue}))
			if err == nil {
				low, err = c.owner.manager.RewriterMakeApp(decl, ast.SliceArguments([]ast.ASTHandle{value, elseValue}))
			}
		} else {
			high, err = c.owner.manager.RewriterMakeApp(decl, ast.SliceArguments([]ast.ASTHandle{thenValue, value}))
			if err == nil {
				low, err = c.owner.manager.RewriterMakeApp(decl, ast.SliceArguments([]ast.ASTHandle{elseValue, value}))
			}
		}
	}
	if err != nil {
		return RewriteFailed, err
	}
	result, err := c.owner.manager.MkIte(condition, high, low)
	if err != nil {
		return RewriteFailed, err
	}
	return Rewrite2, c.owner.manager.SetRef(out, result)
}

func (c *theoryRewriterConfiguration) iteValueTree(term ast.ASTHandle) (bool, error) {
	if _, _, _, matched, err := c.iteView(term); err != nil || !matched {
		return false, err
	}
	stack := []ast.ASTHandle{term}
	for len(stack) != 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		_, high, low, _, err := c.iteView(current)
		if err != nil {
			return false, err
		}
		for _, branch := range []ast.ASTHandle{high, low} {
			_, _, _, nested, err := c.iteView(branch)
			if err != nil {
				return false, err
			}
			if nested {
				refs, err := c.owner.manager.GetRefCount(branch)
				if err != nil {
					return false, err
				}
				if refs == 1 {
					stack = append(stack, branch)
					continue
				}
			}
			value, err := c.owner.manager.IsValue(branch)
			if err != nil || !value {
				return false, err
			}
		}
	}
	return true, nil
}

func (c *theoryRewriterConfiguration) pullITE(decl ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
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
	rangeSort, err := c.owner.manager.GetRange(decl)
	if err != nil {
		return RewriteFailed, err
	}
	boolSort, err := c.owner.manager.MkBoolSort()
	if err != nil {
		return RewriteFailed, err
	}
	leftSort, err := c.owner.manager.GetSort(left)
	if err != nil {
		return RewriteFailed, err
	}
	if rangeSort == boolSort && leftSort != boolSort {
		leftCondition, leftHigh, leftLow, leftITE, err := c.iteView(left)
		if err != nil {
			return RewriteFailed, err
		}
		if leftITE {
			rightValue, err := c.owner.manager.IsValue(right)
			if err != nil {
				return RewriteFailed, err
			}
			if rightValue {
				return c.pullITECore(decl, left, right, false, out)
			}
			rightCondition, rightHigh, rightLow, rightITE, err := c.iteView(right)
			if err != nil {
				return RewriteFailed, err
			}
			if rightITE && leftCondition == rightCondition {
				high, err := c.owner.manager.RewriterMakeApp(decl, ast.SliceArguments([]ast.ASTHandle{leftHigh, rightHigh}))
				if err != nil {
					return RewriteFailed, err
				}
				low, err := c.owner.manager.RewriterMakeApp(decl, ast.SliceArguments([]ast.ASTHandle{leftLow, rightLow}))
				if err != nil {
					return RewriteFailed, err
				}
				result, err := c.owner.manager.MkIte(leftCondition, high, low)
				if err != nil {
					return RewriteFailed, err
				}
				return Rewrite2, c.owner.manager.SetRef(out, result)
			}
		}
		if _, _, _, rightITE, err := c.iteView(right); err != nil {
			return RewriteFailed, err
		} else if rightITE {
			leftValue, err := c.owner.manager.IsValue(left)
			if err != nil {
				return RewriteFailed, err
			}
			if leftValue {
				return c.pullITECore(decl, right, left, true, out)
			}
		}
	}
	family, err := c.owner.manager.GetFamilyID(decl)
	if err != nil {
		return RewriteFailed, err
	}
	if family != c.owner.manager.BasicFamilyID() && family != c.arithmetic.family && family != c.bitVector.bvFamily {
		return RewriteFailed, nil
	}
	leftValue, err := c.owner.manager.IsValue(left)
	if err != nil {
		return RewriteFailed, err
	}
	if leftValue {
		tree, err := c.iteValueTree(right)
		if err != nil {
			return RewriteFailed, err
		}
		if tree {
			return c.pullITECore(decl, right, left, true, out)
		}
	}
	rightValue, err := c.owner.manager.IsValue(right)
	if err != nil {
		return RewriteFailed, err
	}
	if rightValue {
		tree, err := c.iteValueTree(left)
		if err != nil {
			return RewriteFailed, err
		}
		if tree {
			return c.pullITECore(decl, left, right, false, out)
		}
	}
	return RewriteFailed, nil
}

func (c *theoryRewriterConfiguration) pullITEResult(out ast.ASTRef) (RewriteStatus, error) {
	term, err := c.owner.manager.RefGet(out)
	if err != nil {
		return RewriteFailed, err
	}
	kind, err := c.owner.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return RewriteDone, err
	}
	decl, err := c.owner.manager.GetDecl(term)
	if err != nil {
		return RewriteFailed, err
	}
	args, err := c.owner.manager.AppArgumentView(term)
	if err != nil {
		return RewriteFailed, err
	}
	status, err := c.pullITE(decl, args, out)
	if err != nil || status != RewriteFailed {
		return status, err
	}
	return RewriteDone, nil
}

func (c *theoryRewriterConfiguration) arithmeticBVApp(term ast.ASTHandle) (bool, error) {
	kind, err := c.owner.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := c.owner.manager.GetFamilyID(term)
	if err != nil {
		return false, err
	}
	return family == c.arithmetic.family && c.pushITEArithmetic || family == c.bitVector.bvFamily && c.pushITEBitVector, nil
}

func (c *theoryRewriterConfiguration) neutralElement(term ast.ASTHandle) (ast.ASTHandle, bool, error) {
	family, err := c.owner.manager.GetFamilyID(term)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	kind, err := c.owner.manager.GetDeclKind(term)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	value := int64(0)
	switch {
	case family == c.arithmetic.family && kind == arithOpAdd:
	case family == c.arithmetic.family && kind == arithOpMul:
		value = 1
	case family == c.bitVector.bvFamily && kind == theoryBVOpAdd:
	case family == c.bitVector.bvFamily && kind == theoryBVOpMul:
		value = 1
	default:
		return ast.ASTHandle{}, false, nil
	}
	if family == c.bitVector.bvFamily {
		width, err := c.owner.manager.BitVectorSize(term)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		result, err := c.owner.manager.BitVectorNumeralUnsigned(uint32(value), width)
		return result, err == nil, err
	}
	sort, err := c.owner.manager.GetSort(term)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	sortKind, err := c.owner.manager.GetDeclKind(sort)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	result, err := c.arithmetic.makeNumeral(big.NewRat(value, 1), sortKind == 1)
	return result, err == nil, err
}

func (c *theoryRewriterConfiguration) unifyCore(firstTerm, secondTerm ast.ASTHandle) (newFirst, newSecond, common ast.ASTHandle, commonFirst, matched bool, err error) {
	count, err := c.owner.manager.GetNumArgs(firstTerm)
	if err != nil || count != 2 {
		return newFirst, newSecond, common, false, false, err
	}
	a1, err := c.owner.manager.GetArg(firstTerm, 0)
	if err != nil {
		return newFirst, newSecond, common, false, false, err
	}
	b1, err := c.owner.manager.GetArg(firstTerm, 1)
	if err != nil {
		return newFirst, newSecond, common, false, false, err
	}
	if secondTerm == b1 || secondTerm == a1 {
		neutral, ok, err := c.neutralElement(firstTerm)
		if err != nil || !ok {
			return newFirst, newSecond, common, false, false, err
		}
		if secondTerm == b1 {
			return a1, neutral, b1, false, true, nil
		}
		return b1, neutral, a1, true, true, nil
	}
	secondKind, err := c.owner.manager.GetKind(secondTerm)
	if err != nil || secondKind != ast.ASTApp {
		return newFirst, newSecond, common, false, false, err
	}
	firstDecl, err := c.owner.manager.GetDecl(firstTerm)
	if err != nil {
		return newFirst, newSecond, common, false, false, err
	}
	secondDecl, err := c.owner.manager.GetDecl(secondTerm)
	if err != nil || secondDecl != firstDecl {
		return newFirst, newSecond, common, false, false, err
	}
	secondCount, err := c.owner.manager.GetNumArgs(secondTerm)
	if err != nil || secondCount != 2 {
		return newFirst, newSecond, common, false, false, err
	}
	a2, err := c.owner.manager.GetArg(secondTerm, 0)
	if err != nil {
		return newFirst, newSecond, common, false, false, err
	}
	b2, err := c.owner.manager.GetArg(secondTerm, 1)
	if err != nil {
		return newFirst, newSecond, common, false, false, err
	}
	if b1 == b2 {
		return a1, a2, b2, false, true, nil
	}
	if a1 == a2 {
		return b1, b2, a1, true, true, nil
	}
	commutative, err := c.owner.manager.RewriterCommutative(firstDecl)
	if err != nil || !commutative {
		return newFirst, newSecond, common, false, false, err
	}
	if a1 == b2 {
		return b1, a2, a1, true, true, nil
	}
	if b1 == a2 {
		return a1, b2, b1, false, true, nil
	}
	return newFirst, newSecond, common, false, false, nil
}

func (c *theoryRewriterConfiguration) unify(firstTerm, secondTerm ast.ASTHandle) (decl, newFirst, newSecond, common ast.ASTHandle, commonFirst, matched bool, err error) {
	firstApp, err := c.arithmeticBVApp(firstTerm)
	if err != nil {
		return decl, newFirst, newSecond, common, false, false, err
	}
	if firstApp {
		decl, err = c.owner.manager.GetDecl(firstTerm)
		if err != nil {
			return decl, newFirst, newSecond, common, false, false, err
		}
		newFirst, newSecond, common, commonFirst, matched, err = c.unifyCore(firstTerm, secondTerm)
		return
	}
	secondApp, err := c.arithmeticBVApp(secondTerm)
	if err != nil || !secondApp {
		return decl, newFirst, newSecond, common, false, false, err
	}
	decl, err = c.owner.manager.GetDecl(secondTerm)
	if err != nil {
		return decl, newFirst, newSecond, common, false, false, err
	}
	var secondNew, firstNew ast.ASTHandle
	secondNew, firstNew, common, commonFirst, matched, err = c.unifyCore(secondTerm, firstTerm)
	return decl, firstNew, secondNew, common, commonFirst, matched, err
}

func (c *theoryRewriterConfiguration) pushITE(decl ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	family, err := c.owner.manager.GetFamilyID(decl)
	if err != nil {
		return RewriteFailed, err
	}
	kind, err := c.owner.manager.GetDeclKind(decl)
	if err != nil || family != c.owner.manager.BasicFamilyID() || kind != ast.ASTOpIte || args.Count() != 3 {
		return RewriteFailed, err
	}
	condition, err := args.At(0)
	if err != nil {
		return RewriteFailed, err
	}
	thenValue, err := args.At(1)
	if err != nil {
		return RewriteFailed, err
	}
	elseValue, err := args.At(2)
	if err != nil {
		return RewriteFailed, err
	}
	outerDecl, newThen, newElse, common, commonFirst, matched, err := c.unify(thenValue, elseValue)
	if err != nil || !matched {
		return RewriteFailed, err
	}
	inner, err := c.owner.manager.MkIte(condition, newThen, newElse)
	if err != nil {
		return RewriteFailed, err
	}
	outerArgs := []ast.ASTHandle{inner, common}
	if commonFirst {
		outerArgs[0], outerArgs[1] = common, inner
	}
	result, err := c.owner.manager.RewriterMakeApp(outerDecl, ast.SliceArguments(outerArgs))
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, c.owner.manager.SetRef(out, result)
}

func (c *theoryRewriterConfiguration) pushITEResult(out ast.ASTRef) (RewriteStatus, error) {
	term, err := c.owner.manager.RefGet(out)
	if err != nil {
		return RewriteFailed, err
	}
	_, _, _, matched, err := c.iteView(term)
	if err != nil || !matched {
		return RewriteDone, err
	}
	decl, err := c.owner.manager.GetDecl(term)
	if err != nil {
		return RewriteFailed, err
	}
	args, err := c.owner.manager.AppArgumentView(term)
	if err != nil {
		return RewriteFailed, err
	}
	status, err := c.pushITE(decl, args, out)
	if err != nil || status != RewriteFailed {
		return status, err
	}
	return RewriteDone, nil
}

func (c *theoryRewriterConfiguration) ReduceApp(decl ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (RewriteStatus, error) {
	if err := c.owner.manager.SetRef(proof, ast.ASTHandle{}); err != nil {
		return RewriteFailed, err
	}
	status, err := c.reduceAppCore(decl, args, out)
	if err != nil || status != RewriteDone && status != RewriteFailed {
		return status, err
	}
	if c.pushITEArithmetic || c.pushITEBitVector {
		if status == RewriteFailed {
			status, err = c.pushITE(decl, args, out)
		} else {
			status, err = c.pushITEResult(out)
		}
		if err != nil {
			return status, err
		}
	}
	if c.pullCheapITE {
		if status == RewriteFailed {
			return c.pullITE(decl, args, out)
		}
		return c.pullITEResult(out)
	}
	return status, nil
}

// TheoryRewriter applies all retained cheap theory reducers through one
// traversal. This first source slice supplies construction and core dispatch;
// the source pull/push-ITE and substitution extensions are added separately.
type TheoryRewriter struct {
	self          *TheoryRewriter
	manager       *ast.ASTManager
	context       *uparams.ParamsContext
	globals       *uparams.GlobalParamsService
	local         *uparams.ParamsRef
	params        uparams.ParamsRef
	paramsPlaced  bool
	configuration theoryRewriterConfiguration
	traversal     *Traversal
	header        uint64
	closed        bool
}

func NewTheoryRewriter(m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, local *uparams.ParamsRef) (*TheoryRewriter, error) {
	if m == nil || context == nil || globals == nil || context.SymbolsField != m.Symbols || globals.Context != context {
		return nil, uparams.ErrParamsOwner
	}
	if err := m.RegisterDeclarationPlugins(); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(TheoryRewriter{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	r := &TheoryRewriter{manager: m, context: context, globals: globals, header: n}
	r.self = r
	var err error
	if r.params, err = context.NewValue(); err != nil {
		return r, err
	}
	r.paramsPlaced = true
	if local != nil {
		if err = context.Assign(&r.params, local); err != nil {
			return r, err
		}
	}
	r.local = &r.params
	r.configuration.owner = r
	if err := r.constructConfiguration(); err != nil {
		return r, err
	}
	proofs, err := m.ProofsEnabled()
	if err != nil {
		return r, err
	}
	if proofs {
		r.traversal, err = NewProofTraversal(m, &r.configuration)
	} else {
		r.traversal, err = NewTraversal(m, &r.configuration)
	}
	if err != nil {
		return r, err
	}
	return r, nil
}

func (r *TheoryRewriter) constructConfiguration() (err error) {
	c := &r.configuration
	legacy, err := r.globals.Manager()
	if err != nil {
		return err
	}
	if c.boolean, err = NewBoolRewriter(r.manager, r.context, r.globals, r.local); err != nil {
		return err
	}
	if c.arithmetic, err = NewArithRewriter(r.manager, r.context, r.globals, r.local); err != nil {
		return err
	}
	if c.bitVector, err = NewBVRewriter(r.manager, r.context, legacy, r.local); err != nil {
		return err
	}
	if c.array, err = NewArrayRewriter(r.manager, r.context, r.globals, r.local); err != nil {
		return err
	}
	if c.datatype, err = NewDatatypeRewriter(r.manager); err != nil {
		return err
	}
	if c.floating, err = NewFPARewriter(r.manager); err != nil {
		return err
	}
	if err = r.updateFloatingParams(); err != nil {
		return err
	}
	if c.datalog, err = NewDLRewriter(r.manager); err != nil {
		return err
	}
	if c.pseudoBoolean, err = NewPseudoBooleanRewriter(r.manager); err != nil {
		return err
	}
	if c.sequence, err = NewSequenceRewriter(r.manager); err != nil {
		return err
	}
	return r.readLocalParams()
}

func (r *TheoryRewriter) updateFloatingParams() (result error) {
	p, err := NewFPARewriterParams(r.context, r.globals, r.local)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, p.Close()) }()
	high, err := p.HighFPUnspecified()
	if err != nil {
		return err
	}
	return r.configuration.floating.SetHighFPUnspecified(high)
}

func (r *TheoryRewriter) readLocalParams() (result error) {
	p, err := NewRewriterParams(r.context, r.globals, r.local)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, p.Close()) }()
	c := &r.configuration
	megabytes, err := p.MaxMemory()
	if err != nil {
		return err
	}
	c.maxMemory = uint64(megabytes) * 1024 * 1024
	if c.maxSteps, err = p.MaxSteps(); err != nil {
		return err
	}
	if c.pullCheapITE, err = p.PullCheapITE(); err != nil {
		return err
	}
	if c.flat, err = p.Flat(); err != nil {
		return err
	}
	if c.cacheAll, err = p.CacheAll(); err != nil {
		return err
	}
	if c.pushITEArithmetic, err = p.PushITEArith(); err != nil {
		return err
	}
	if c.pushITEBitVector, err = p.PushITEBV(); err != nil {
		return err
	}
	if c.ignoreGroundPatterns, err = p.IgnorePatternsOnGroundQBody(); err != nil {
		return err
	}
	c.rewritePatternTerms, err = p.RewritePatterns()
	return err
}

func (r *TheoryRewriter) open() error {
	if r == nil || r.self != r || r.closed || r.traversal == nil {
		return ast.ErrASTHandle
	}
	_, err := r.manager.ProofsEnabled()
	return err
}

func (r *TheoryRewriter) Manager() *ast.ASTManager {
	if r == nil || r.self != r || r.closed {
		return nil
	}
	return r.manager
}

func TheoryRewriterParameterDescriptions(descriptions *uparams.ParamDescrs) error {
	if descriptions == nil {
		return uparams.ErrParamsOwner
	}
	for _, collect := range []func(*uparams.ParamDescrs) error{
		catalogCollectBoolRewriterParams,
		catalogCollectPolyRewriterParams,
		catalogCollectArithRewriterParams,
		catalogCollectBvRewriterParams,
		catalogCollectArrayRewriterParams,
		catalogCollectRewriterParams,
	} {
		if err := collect(descriptions); err != nil {
			return err
		}
	}
	return nil
}

func (r *TheoryRewriter) Rewrite(term ast.ASTHandle, out ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.traversal.Rewrite(term, out)
}

// FreshDefault realizes a reached th_rewriter(manager) local constructor
// using the same borrowed parameter services. Local options, substitutions,
// solver callbacks and traversal state belong only to the original rewriter.
func (r *TheoryRewriter) FreshDefault() (*TheoryRewriter, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	return NewTheoryRewriter(r.manager, r.context, r.globals, nil)
}

func (r *TheoryRewriter) RewriteWithProof(term ast.ASTHandle, out, proof ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.traversal.RewriteWithProof(term, out, proof)
}

func (r *TheoryRewriter) SetSolver(solver SequenceExpressionSolver) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.configuration.sequence.SetSolver(solver)
}

func (r *TheoryRewriter) RewriteWithBindings(term ast.ASTHandle, bindings []ast.ASTHandle, out ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	if err := r.traversal.Reset(); err != nil {
		return err
	}
	if err := r.traversal.SetInvBindings(bindings); err != nil {
		return err
	}
	return r.traversal.Rewrite(term, out)
}

func (r *TheoryRewriter) MakeApp(decl ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	proof, err := r.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer r.manager.DelRef(&proof)
	status, err := r.configuration.ReduceApp(decl, args, out, proof)
	if err != nil {
		return err
	}
	if status != RewriteFailed {
		return nil
	}
	h, err := r.manager.RewriterMakeApp(decl, args)
	if err != nil {
		return err
	}
	return r.manager.SetRef(out, h)
}

func (r *TheoryRewriter) NumSteps() (uint32, error) {
	if err := r.open(); err != nil {
		return 0, err
	}
	return r.traversal.NumSteps()
}

func (r *TheoryRewriter) CacheSize() (uint32, error) {
	if err := r.open(); err != nil {
		return 0, err
	}
	return r.traversal.CacheSize()
}

func (r *TheoryRewriter) UpdateParams(params *uparams.ParamsRef) (result error) {
	if err := r.open(); err != nil {
		return err
	}
	if params == nil {
		empty, err := r.context.NewValue()
		if err != nil {
			return err
		}
		params = &empty
		defer func() { result = errors.Join(result, r.context.Del(&empty)) }()
	}
	if err := r.context.Assign(&r.params, params); err != nil {
		return err
	}
	if err := r.configuration.boolean.UpdtParams(r.local); err != nil {
		return err
	}
	if err := r.configuration.arithmetic.updateParams(r.context, r.globals, r.local); err != nil {
		return err
	}
	if err := r.configuration.bitVector.polynomial.config.UpdateBitVector(r.local); err != nil {
		return err
	}
	if err := r.configuration.array.UpdtParams(r.local); err != nil {
		return err
	}
	if err := r.updateFloatingParams(); err != nil {
		return err
	}
	return r.readLocalParams()
}

func (r *TheoryRewriter) Reset() error {
	if err := r.open(); err != nil {
		return err
	}
	return r.traversal.Reset()
}

// SetSubstitution installs a borrowed substitution after clearing the rewrite
// cache. A nil argument clears the current substitution.
func (r *TheoryRewriter) SetSubstitution(substitution *ast.ASTExprSubstitution) error {
	if err := r.open(); err != nil {
		return err
	}
	if substitution != nil {
		manager, err := r.manager.ExprSubstitutionManager(*substitution)
		if err != nil {
			return err
		}
		if manager != r.manager {
			return ast.ErrASTHandle
		}
	}
	if err := r.traversal.Reset(); err != nil {
		return err
	}
	r.configuration.hasSubstitution = substitution != nil
	if substitution != nil {
		r.configuration.substitution = *substitution
	}
	return nil
}

// UsedDependencies returns the borrowed dependency accumulated by substitutions.
func (r *TheoryRewriter) UsedDependencies() (ast.ASTDependency, error) {
	if err := r.open(); err != nil {
		return ast.ASTDependency{}, err
	}
	return r.configuration.usedDependencies, nil
}

// ResetUsedDependencies clears accumulated dependencies while preserving the
// current substitution and clearing cached rewrite results.
func (r *TheoryRewriter) ResetUsedDependencies() error {
	if err := r.open(); err != nil {
		return err
	}
	used := r.configuration.usedDependencies
	if used.IsNull() {
		return nil
	}
	substitution := r.configuration.substitution
	hasSubstitution := r.configuration.hasSubstitution
	if err := r.traversal.Reset(); err != nil {
		return err
	}
	r.configuration.substitution = substitution
	r.configuration.hasSubstitution = hasSubstitution
	if err := r.manager.DependencyDec(used); err != nil {
		return err
	}
	r.configuration.usedDependencies = ast.ASTDependency{}
	return nil
}

func (r *TheoryRewriter) Cleanup() error {
	if err := r.open(); err != nil {
		return err
	}
	if !r.configuration.usedDependencies.IsNull() {
		if err := r.manager.DependencyDec(r.configuration.usedDependencies); err != nil {
			return err
		}
		r.configuration.usedDependencies = ast.ASTDependency{}
	}
	if err := r.traversal.Close(); err != nil {
		return err
	}
	r.traversal = nil
	if err := r.closeConfiguration(); err != nil {
		return err
	}
	r.configuration = theoryRewriterConfiguration{owner: r}
	if err := r.constructConfiguration(); err != nil {
		return err
	}
	proofs, err := r.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	var traversal *Traversal
	if proofs {
		traversal, err = NewProofTraversal(r.manager, &r.configuration)
	} else {
		traversal, err = NewTraversal(r.manager, &r.configuration)
	}
	if err != nil {
		return err
	}
	r.traversal = traversal
	return nil
}

func (r *TheoryRewriter) closeConfiguration() error {
	c := &r.configuration
	c.sequence = nil
	if c.pseudoBoolean != nil {
		if err := c.pseudoBoolean.Close(); err != nil {
			return err
		}
		c.pseudoBoolean = nil
	}
	if c.datalog != nil {
		if err := c.datalog.Close(); err != nil {
			return err
		}
		c.datalog = nil
	}
	if c.floating != nil {
		if err := c.floating.Close(); err != nil {
			return err
		}
		c.floating = nil
	}
	c.datatype = nil
	if c.array != nil {
		if err := c.array.Close(); err != nil {
			return err
		}
		c.array = nil
	}
	if c.bitVector != nil {
		if err := c.bitVector.Close(); err != nil {
			return err
		}
		c.bitVector = nil
	}
	if c.arithmetic != nil {
		if err := c.arithmetic.Close(); err != nil {
			return err
		}
		c.arithmetic = nil
	}
	if c.boolean != nil {
		if err := c.boolean.Close(); err != nil {
			return err
		}
		c.boolean = nil
	}
	return nil
}

func (r *TheoryRewriter) Close() error {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	if !r.configuration.usedDependencies.IsNull() {
		if err := r.manager.DependencyDec(r.configuration.usedDependencies); err != nil {
			return err
		}
		r.configuration.usedDependencies = ast.ASTDependency{}
	}
	if r.traversal != nil {
		if err := r.traversal.Close(); err != nil {
			return err
		}
		r.traversal = nil
	}
	if err := r.closeConfiguration(); err != nil {
		return err
	}
	if r.paramsPlaced {
		if err := r.context.Del(&r.params); err != nil {
			return err
		}
		r.paramsPlaced = false
		r.local = nil
	}
	r.manager.Budget().Release(r.header)
	r.header = 0
	r.closed = true
	return nil
}

package rewriter

// Source: src/ast/fpa/fpa2bv_rewriter.cpp/.h.

import (
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	astfpa "github.com/glycerine/g3/pkg/ast/fpa"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type fpa2BVConfiguration struct {
	DefaultConfiguration
	owner     *FPA2BVRewriter
	bindings  []ast.ASTHandle
	maxSteps  uint32
	maxMemory uint64
}

type FPA2BVRewriter struct {
	self      *FPA2BVRewriter
	manager   *ast.ASTManager
	converter *astfpa.Converter
	config    fpa2BVConfiguration
	traversal *Traversal
	header    uint64
	closed    bool
}

// NewFPA2BVRewriter borrows the manager and converter. The converter must
// outlive the rewriter. A nonnil partial result must close even on error.
func NewFPA2BVRewriter(m *ast.ASTManager, converter *astfpa.Converter) (owner *FPA2BVRewriter, result error) {
	if m == nil || converter == nil {
		return nil, ast.ErrASTHandle
	}
	bytes := uint64(reflect.TypeOf(FPA2BVRewriter{}).Size())
	if err := m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	owner = &FPA2BVRewriter{manager: m, converter: converter, header: bytes}
	owner.self = owner
	owner.config.owner = owner
	owner.config.defaults.manager = m
	owner.config.maxSteps = math.MaxUint32
	owner.config.maxMemory = math.MaxUint64
	traversal, err := NewTraversal(m, &owner.config)
	owner.traversal = traversal
	if err != nil {
		return owner, err
	}
	return owner, nil
}

func (r *FPA2BVRewriter) open() error {
	if r == nil || r.self != r || r.closed || r.manager == nil || r.converter == nil || r.traversal == nil {
		return ast.ErrASTHandle
	}
	_, err := r.manager.ProofsEnabled()
	return err
}

func (r *FPA2BVRewriter) SetLimits(maxSteps uint32, maxMemory uint64) error {
	if err := r.open(); err != nil {
		return err
	}
	r.config.maxSteps, r.config.maxMemory = maxSteps, maxMemory
	return nil
}

func (r *FPA2BVRewriter) SetHighUnspecified(value bool) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.converter.SetHighUnspecified(value)
}

func (r *FPA2BVRewriter) Rewrite(root ast.ASTHandle, out ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.traversal.Rewrite(root, out)
}

func (r *FPA2BVRewriter) RewriteWithProof(root ast.ASTHandle, out, proof ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.traversal.RewriteWithProof(root, out, proof)
}

func (r *FPA2BVRewriter) NumSteps() (uint32, error) {
	if err := r.open(); err != nil {
		return 0, err
	}
	return r.traversal.NumSteps()
}

func (r *FPA2BVRewriter) Reset() error {
	if err := r.open(); err != nil {
		return err
	}
	r.config.bindings = nil
	return r.traversal.Reset()
}

func (r *FPA2BVRewriter) Close() (result error) {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	if r.traversal != nil {
		if err := r.traversal.Close(); err != nil {
			return err
		}
		r.traversal = nil
	}
	r.manager.Budget().Release(r.header)
	r.header = 0
	r.config.bindings = nil
	r.traversal = nil
	r.closed = true
	return result
}

func (c *fpa2BVConfiguration) MaxStepsExceeded(steps uint32) (bool, error) {
	return steps > c.maxSteps || c.owner.manager.Budget().Used() > c.maxMemory, nil
}

func fpa2BVSlice(args ast.AppArguments) ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, args.Count())
	for i := range result {
		value, err := args.At(uint32(i))
		if err != nil {
			return nil, err
		}
		result[i] = value
	}
	return result, nil
}

func (c *fpa2BVConfiguration) expressionSorts(h ast.ASTHandle) (floating, rounding bool, err error) {
	sort, err := c.owner.manager.GetSort(h)
	if err != nil {
		return false, false, err
	}
	_, _, floating, err = c.owner.manager.FloatingPointSortShape(sort)
	if err != nil {
		return false, false, err
	}
	rounding, err = c.owner.manager.IsRoundingModeSort(sort)
	return floating, rounding, err
}

func (c *fpa2BVConfiguration) ReduceApp(decl ast.ASTHandle, args ast.AppArguments, out, _ ast.ASTRef) (RewriteStatus, error) {
	r := c.owner
	m, converter := r.manager, r.converter
	family, err := m.GetFamilyID(decl)
	if err != nil {
		return RewriteFailed, err
	}
	kind, err := m.GetDeclKind(decl)
	if err != nil {
		return RewriteFailed, err
	}
	values, err := fpa2BVSlice(args)
	if err != nil {
		return RewriteFailed, err
	}
	if family == ast.NullFamilyID && args.Count() == 0 {
		rangeSort, e := m.GetRange(decl)
		if e != nil {
			return RewriteFailed, e
		}
		_, _, floating, e := m.FloatingPointSortShape(rangeSort)
		if e != nil {
			return RewriteFailed, e
		}
		if floating {
			value, e := converter.MkConst(decl)
			if e != nil {
				return RewriteFailed, e
			}
			return RewriteDone, m.SetRef(out, value)
		}
		rounding, e := m.IsRoundingModeSort(rangeSort)
		if e != nil {
			return RewriteFailed, e
		}
		if rounding {
			value, e := converter.MkRMConst(decl)
			if e != nil {
				return RewriteFailed, e
			}
			return RewriteDone, m.SetRef(out, value)
		}
	}
	if family == m.BasicFamilyID() {
		switch kind {
		case ast.ASTOpEq, ast.ASTOpIff:
			if len(values) != 2 {
				return RewriteFailed, ast.ErrASTDomain
			}
			floating, rounding, e := c.expressionSorts(values[0])
			if e != nil {
				return RewriteFailed, e
			}
			if floating {
				value, e := converter.MkEq(values[0], values[1])
				if e != nil {
					return RewriteFailed, e
				}
				return RewriteDone, m.SetRef(out, value)
			}
			if rounding {
				value, e := m.MkEq(values[0], values[1])
				if e != nil {
					return RewriteFailed, e
				}
				return RewriteDone, m.SetRef(out, value)
			}
		case ast.ASTOpIte:
			if len(values) != 3 {
				return RewriteFailed, ast.ErrASTDomain
			}
			floating, rounding, e := c.expressionSorts(values[1])
			if e != nil {
				return RewriteFailed, e
			}
			if floating || rounding {
				value, e := converter.MkITE(values[0], values[1], values[2])
				if e != nil {
					return RewriteFailed, e
				}
				return RewriteDone, m.SetRef(out, value)
			}
		case ast.ASTOpDistinct:
			if len(values) == 0 {
				return RewriteFailed, nil
			}
			floating, rounding, e := c.expressionSorts(values[0])
			if e != nil {
				return RewriteFailed, e
			}
			if floating || rounding {
				value, e := converter.MkDistinct(values)
				if e != nil {
					return RewriteFailed, e
				}
				return RewriteDone, m.SetRef(out, value)
			}
		}
	}
	fpaFamily, err := m.ExistingFloatingPointFamilyID()
	if err != nil {
		return RewriteFailed, err
	}
	if family == fpaFamily {
		var value ast.ASTHandle
		done := true
		switch kind {
		case ast.FPAOpRMNearestEven, ast.FPAOpRMNearestAway, ast.FPAOpRMTowardPositive, ast.FPAOpRMTowardNegative, ast.FPAOpRMTowardZero:
			value, err = converter.MkRoundingMode(kind)
		case ast.FPAOpNumeral:
			var original ast.ASTHandle
			original, err = m.MkApp(decl, nil)
			if err == nil {
				var found bool
				value, found, err = converter.MkNumeral(original)
				if !found && err == nil {
					done = false
				}
			}
		case ast.FPAOpPositiveInf, ast.FPAOpNegativeInf, ast.FPAOpNaN, ast.FPAOpPositiveZero, ast.FPAOpNegativeZero:
			var sort ast.ASTHandle
			sort, err = m.GetRange(decl)
			if err == nil {
				special := astfpa.PositiveInfinity
				switch kind {
				case ast.FPAOpNegativeInf:
					special = astfpa.NegativeInfinity
				case ast.FPAOpNaN:
					special = astfpa.NaN
				case ast.FPAOpPositiveZero:
					special = astfpa.PositiveZero
				case ast.FPAOpNegativeZero:
					special = astfpa.NegativeZero
				}
				value, err = converter.MkSpecial(sort, special)
			}
		case ast.FPAOpNeg:
			if len(values) != 1 {
				err = ast.ErrASTDomain
			} else {
				value, err = converter.Negate(values[0])
			}
		case ast.FPAOpAbs:
			if len(values) != 1 {
				err = ast.ErrASTDomain
			} else {
				value, err = converter.Abs(values[0])
			}
		case ast.FPAOpAdd, ast.FPAOpSub, ast.FPAOpMul, ast.FPAOpDiv:
			if len(values) != 3 {
				err = ast.ErrASTDomain
			} else if kind == ast.FPAOpAdd {
				value, err = converter.Add(values[0], values[1], values[2])
			} else if kind == ast.FPAOpSub {
				value, err = converter.Sub(values[0], values[1], values[2])
			} else if kind == ast.FPAOpMul {
				value, err = converter.Mul(values[0], values[1], values[2])
			} else {
				value, err = converter.Div(values[0], values[1], values[2])
			}
		case ast.FPAOpRem:
			if len(values) != 2 {
				err = ast.ErrASTDomain
			} else {
				value, err = converter.Rem(values[0], values[1])
			}
		case ast.FPAOpSqrt, ast.FPAOpRoundToIntegral:
			if len(values) != 2 {
				err = ast.ErrASTDomain
			} else if kind == ast.FPAOpSqrt {
				value, err = converter.Sqrt(values[0], values[1])
			} else {
				value, err = converter.RoundToIntegral(values[0], values[1])
			}
		case ast.FPAOpFMA:
			if len(values) != 4 {
				err = ast.ErrASTDomain
			} else {
				value, err = converter.FMA(values[0], values[1], values[2], values[3])
			}
		case ast.FPAOpMin, ast.FPAOpMax:
			if len(values) != 2 {
				err = ast.ErrASTDomain
			} else {
				value, err = converter.Extremum(decl, kind, values[0], values[1])
			}
		case ast.FPAOpEq, ast.FPAOpLt, ast.FPAOpGt, ast.FPAOpLe, ast.FPAOpGe:
			if len(values) != 2 {
				err = ast.ErrASTDomain
			} else {
				value, err = converter.Compare(kind, values[0], values[1])
			}
		case int32(ast.FPAIsNaN), int32(ast.FPAIsInfinite), int32(ast.FPAIsZero), int32(ast.FPAIsNormal), int32(ast.FPAIsSubnormal), int32(ast.FPAIsNegative), int32(ast.FPAIsPositive):
			if len(values) != 1 {
				err = ast.ErrASTDomain
			} else {
				value, err = converter.Classify(values[0], ast.FPAClassificationKind(kind))
			}
		case ast.FPAOpFP:
			if len(values) != 3 {
				err = ast.ErrASTDomain
			} else {
				value, err = converter.MkFP(values[0], values[1], values[2])
			}
		case ast.FPAOpToIEEEBV:
			if len(values) != 1 {
				err = ast.ErrASTDomain
			} else {
				value, err = converter.ToIEEEBVFor(decl, values[0])
			}
		case ast.FPAOpToFP:
			var targetSort ast.ASTHandle
			targetSort, err = m.GetRange(decl)
			if err != nil {
				break
			}
			switch len(values) {
			case 1:
				value, err = converter.FromIEEEBV(targetSort, values[0])
			case 2:
				var sourceSort ast.ASTHandle
				sourceSort, err = m.GetSort(values[1])
				if err != nil {
					break
				}
				var floating bool
				_, _, floating, err = m.FloatingPointSortShape(sourceSort)
				if err == nil && floating {
					value, err = converter.ToFPFloat(targetSort, values[0], values[1])
				} else if err == nil {
					if _, sizeErr := m.BitVectorSize(values[1]); sizeErr == nil {
						value, err = converter.ToFPSigned(targetSort, values[0], values[1])
					} else {
						value, err = converter.ToFPReal(decl, targetSort, values[0], values[1])
					}
				}
			case 3:
				var rounding bool
				_, rounding, err = c.expressionSorts(values[0])
				if err == nil && rounding {
					value, err = converter.ToFPRealInt(decl, values[0], values[1], values[2])
				} else if err == nil {
					value, err = converter.MkFP(values[0], values[1], values[2])
				}
			default:
				err = ast.ErrASTDomain
			}
		case ast.FPAOpToFPUnsigned:
			if len(values) != 2 {
				err = ast.ErrASTDomain
			} else {
				var targetSort ast.ASTHandle
				targetSort, err = m.GetRange(decl)
				if err == nil {
					value, err = converter.ToFPUnsigned(targetSort, values[0], values[1])
				}
			}
		case ast.FPAOpToUBV, ast.FPAOpToSBV:
			if len(values) != 2 {
				err = ast.ErrASTDomain
			} else {
				var rangeSort ast.ASTHandle
				rangeSort, err = m.GetRange(decl)
				if err != nil {
					break
				}
				var width uint32
				width, err = m.BitVectorSize(rangeSort)
				if err == nil && kind == ast.FPAOpToUBV {
					value, err = converter.ToUBVFor(decl, width, values[0], values[1])
				} else if err == nil {
					value, err = converter.ToSBVFor(decl, width, values[0], values[1])
				}
			}
		case ast.FPAOpToReal:
			if len(values) != 1 {
				err = ast.ErrASTDomain
			} else {
				value, err = converter.ToRealFor(decl, values[0])
			}
		default:
			done = false
		}
		if err != nil {
			return RewriteFailed, err
		}
		if done {
			return RewriteDone, m.SetRef(out, value)
		}
		return RewriteFailed, nil
	}
	contains, err := m.ContainsFloatingPoint(decl)
	if err != nil {
		return RewriteFailed, err
	}
	if contains {
		value, err := converter.MkUF(decl, values)
		if err != nil {
			return RewriteFailed, err
		}
		return RewriteDone, m.SetRef(out, value)
	}
	return RewriteFailed, nil
}

func (c *fpa2BVConfiguration) PreVisit(term ast.ASTHandle) (bool, error) {
	kind, err := c.owner.manager.GetKind(term)
	if err != nil {
		return false, err
	}
	if kind != ast.ASTQuantifier {
		return true, nil
	}
	count, err := c.owner.manager.GetNumDeclarations(term)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < count; i++ {
		sort, e := c.owner.manager.GetDeclarationSort(term, i)
		if e != nil {
			return false, e
		}
		c.bindings = append(c.bindings, sort)
	}
	return true, nil
}

func (c *fpa2BVConfiguration) ReduceVar(variable ast.ASTHandle, out, _ ast.ASTRef) (bool, error) {
	m := c.owner.manager
	index, err := m.GetVarIndex(variable)
	if err != nil {
		return false, err
	}
	if index >= uint32(len(c.bindings)) {
		return false, nil
	}
	sort, err := m.GetSort(variable)
	if err != nil {
		return false, err
	}
	ebits, sbits, floating, err := m.FloatingPointSortShape(sort)
	if err != nil {
		return false, err
	}
	if !floating {
		value, e := m.MkVar(index, sort)
		if e != nil {
			return false, e
		}
		return true, m.SetRef(out, value)
	}
	width := ebits + sbits
	bvSort, err := m.BitVectorSort(width)
	if err != nil {
		return false, err
	}
	packed, err := m.MkVar(index, bvSort)
	if err != nil {
		return false, err
	}
	sign, err := m.MkBitVectorExtract(width-1, width-1, packed)
	if err != nil {
		return false, err
	}
	exponent, err := m.MkBitVectorExtract(ebits-1, 0, packed)
	if err != nil {
		return false, err
	}
	significand, err := m.MkBitVectorExtract(width-2, ebits, packed)
	if err != nil {
		return false, err
	}
	value, err := c.owner.converter.MkFP(sign, exponent, significand)
	if err != nil {
		return false, err
	}
	return true, m.SetRef(out, value)
}

func (c *fpa2BVConfiguration) ReduceQuantifier(q, body ast.ASTHandle, patterns, noPatterns ast.AppArguments, out, _ ast.ASTRef) (bool, error) {
	m := c.owner.manager
	count, err := m.GetNumDeclarations(q)
	if err != nil {
		return false, err
	}
	if int(count) > len(c.bindings) {
		return false, ast.ErrASTDomain
	}
	oldSize := len(c.bindings) - int(count)
	defer func() { c.bindings = c.bindings[:oldSize] }()
	sorts := make([]ast.ASTHandle, count)
	names := make([]usymbol.Symbol, count)
	for i := uint32(0); i < count; i++ {
		sort, e := m.GetDeclarationSort(q, i)
		if e != nil {
			return false, e
		}
		name, e := m.GetDeclarationName(q, i)
		if e != nil {
			return false, e
		}
		ebits, sbits, floating, e := m.FloatingPointSortShape(sort)
		if e != nil {
			return false, e
		}
		if floating {
			sorts[i], e = m.BitVectorSort(ebits + sbits)
			if e != nil {
				return false, e
			}
			text, e := m.Symbols.Display(name)
			if e != nil {
				return false, e
			}
			names[i], e = m.Symbols.Intern(text + ".bv")
			if e != nil {
				return false, e
			}
		} else {
			sorts[i], names[i] = sort, name
		}
	}
	newPatterns, err := fpa2BVSlice(patterns)
	if err != nil {
		return false, err
	}
	newNoPatterns, err := fpa2BVSlice(noPatterns)
	if err != nil {
		return false, err
	}
	forall, err := m.IsForall(q)
	if err != nil {
		return false, err
	}
	weight, err := m.GetQuantifierWeight(q)
	if err != nil {
		return false, err
	}
	qid, skid, err := m.GetQuantifierIDs(q)
	if err != nil {
		return false, err
	}
	value, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: forall, Sorts: sorts, Names: names, Body: body,
		Weight: weight, QID: qid, SkolemID: skid, Patterns: newPatterns, NoPatterns: newNoPatterns,
	})
	if err != nil {
		return false, err
	}
	return true, m.SetRef(out, value)
}

func (c *fpa2BVConfiguration) Reset()   { c.bindings = nil }
func (c *fpa2BVConfiguration) Cleanup() { c.bindings = nil }

var _ Configuration = (*fpa2BVConfiguration)(nil)

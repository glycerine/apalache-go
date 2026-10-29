package ast

// Source: src/ast/ast.cpp:1568-1638; src/ast/ast.h:1323-1402.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

func (m *ASTManager) mkFamilyApp(fid FamilyID, k int32, ps []ASTParameter, args []ASTHandle) (astNodeRef, error) {
	if fid < 0 || !m.pluginAt(fid).present {
		return astNodeRef{}, nil
	}
	if uint64(len(args)) > math.MaxUint32 {
		return astNodeRef{}, arena.ErrBounds
	}
	if m.pluginAt(fid).kind == astArithmeticPlugin {
		previous := m.formatExecuting
		m.formatExecuting = true
		defer func() { m.formatExecuting = previous }()
		d, e := m.arithmeticExprDecl(k, ps, args)
		if e != nil || d.slot == 0 {
			return astNodeRef{}, e
		}
		return m.mkApp(d, args)
	}
	if m.pluginAt(fid).kind == astBitVectorPlugin {
		previous := m.formatExecuting
		m.formatExecuting = true
		defer func() { m.formatExecuting = previous }()
		d, handled, e := m.bitVectorExprDecl(k, ps, args)
		if e != nil {
			return astNodeRef{}, e
		}
		if handled {
			if d.slot == 0 {
				return astNodeRef{}, nil
			}
			return m.mkApp(d, args)
		}
	}
	// The basic expr overload bypasses decl_plugin's domain buffer for these
	// branches. This matters for overflow-reservation actions on large arities.
	if fid == 0 {
		var direct astNodeRef
		switch k {
		case ASTOpTrue, ASTOpFalse, ASTOpAnd, ASTOpOr, ASTOpNot, ASTOpInterp, ASTOpIff, ASTOpImplies, ASTOpXor:
			direct = m.basic.declarations[k]
		case ASTOpIte:
			if len(args) != 3 {
				return astNodeRef{}, nil
			}
			a, e := m.requireExpr(args[1])
			if e != nil {
				return astNodeRef{}, e
			}
			b, e := m.requireExpr(args[2])
			if e != nil {
				return astNodeRef{}, e
			}
			a, e = m.sortOf(a)
			if e != nil {
				return astNodeRef{}, e
			}
			b, e = m.sortOf(b)
			if e != nil {
				return astNodeRef{}, e
			}
			joined, e := m.joinSorts(a, b)
			if e != nil {
				return astNodeRef{}, e
			}
			direct, e = m.cachedIte(joined)
			if e != nil {
				return astNodeRef{}, e
			}
		case ASTOpEq, ASTOpOEq:
			if len(args) < 2 {
				return astNodeRef{}, nil
			}
			r, e := m.requireExpr(args[0])
			if e != nil {
				return astNodeRef{}, e
			}
			r, e = m.sortOf(r)
			if e != nil {
				return astNodeRef{}, e
			}
			for _, h := range args[1:] {
				other, e := m.requireExpr(h)
				if e != nil {
					return astNodeRef{}, e
				}
				other, e = m.sortOf(other)
				if e != nil {
					return astNodeRef{}, e
				}
				r, e = m.joinSorts(r, other)
				if e != nil {
					return astNodeRef{}, e
				}
			}
			direct, e = m.cachedEquality(k, r)
			if e != nil {
				return astNodeRef{}, e
			}
		}
		if direct.slot != 0 {
			return m.mkApp(direct, args)
		}
		if k >= ASTPrUndef && k < ASTLastBasicPR {
			if k == ASTPrUndef {
				if len(args) != 0 {
					return astNodeRef{}, ErrASTDomain
				}
			} else {
				if len(args) == 0 {
					return astNodeRef{}, ErrASTDomain
				}
				for _, h := range args[:len(args)-1] {
					r, e := m.requireExpr(h)
					if e != nil {
						return astNodeRef{}, e
					}
					r, e = m.sortOf(r)
					if e != nil {
						return astNodeRef{}, e
					}
					if r != m.basic.proofSort {
						return astNodeRef{}, ErrASTDomain
					}
				}
				last, e := m.requireExpr(args[len(args)-1])
				if e != nil {
					return astNodeRef{}, e
				}
				last, e = m.sortOf(last)
				if e != nil {
					return astNodeRef{}, e
				}
				if last != m.basic.boolSort && last != m.basic.proofSort {
					return astNodeRef{}, ErrASTDomain
				}
			}
			d, e := m.proofDecl(k, ps, uint32(len(args))-1)
			if e != nil || d.slot == 0 {
				return astNodeRef{}, e
			}
			return m.mkApp(d, args)
		}
	}
	return m.mkFamilyAppDomainArguments(fid, k, ps, SliceArguments(args))
}

func (m *ASTManager) mkFamilyAppDomainArguments(fid FamilyID, k int32, ps []ASTParameter, args AppArguments) (astNodeRef, error) {
	// Source decl_plugin's expr overload collects sorts in a lexical ptr_buffer;
	// use a native typed slice for those borrowed sort handles.
	var inline [16]ASTHandle
	domain := inline[:0]
	for i := uint32(0); i < args.Count(); i++ {
		h, e := args.At(i)
		if e != nil {
			return astNodeRef{}, e
		}
		r, e := m.requireExpr(h)
		if e != nil {
			return astNodeRef{}, e
		}
		s, e := m.sortOf(r)
		if e != nil {
			return astNodeRef{}, e
		}
		domain = append(domain, m.external(s))
	}
	var decl astNodeRef
	var e error
	if fid == 0 {
		decl, e = m.basicDecl(k, ps, domain)
	} else if m.pluginAt(fid).kind == astArrayPlugin {
		decl, e = m.arrayDecl(k, ps, domain, ASTHandle{})
	} else if m.pluginAt(fid).kind == astFloatingPointPlugin && k >= 0 && (k <= astFPAMinusZero || k >= astFPAAdd && k <= astFPARoundToIntegral || k >= astFPAEq && k <= astFPAGe || k >= astFPAIsNaN && k <= astFPAIsPositive || k == astFPAFP || k == astFPAToFP || k == astFPAToFPUnsigned || k == astFPAToUBV || k == astFPAToSBV || k == astFPAToReal || k == astFPAToIEEEBV || k == astFPABVWrap || k == astFPABV2RM) {
		previous := m.formatExecuting
		m.formatExecuting = true
		defer func() { m.formatExecuting = previous }()
		if k == astFPANeg || k == astFPAAbs {
			decl, e = m.floatingPointUnaryDecl(k, domain)
		} else if k == astFPAToReal {
			decl, e = m.floatingPointToRealDecl(domain)
		} else if k == astFPAToUBV || k == astFPAToSBV {
			decl, e = m.floatingPointToBVDecl(k, ps, domain)
		} else if k == astFPAToFPUnsigned {
			decl, e = m.floatingPointToFPUnsignedDecl(ps, domain)
		} else if k == astFPAFP {
			decl, e = m.floatingPointFPDecl(domain)
		} else if k == astFPAToFP {
			decl, e = m.floatingPointToFPDecl(ps, domain)
		} else if k == astFPAToIEEEBV {
			decl, e = m.floatingPointToIEEEBVDecl(domain)
		} else if k == astFPABVWrap {
			decl, e = m.floatingPointBVWrapDecl(ps, domain)
		} else if k == astFPABV2RM {
			decl, e = m.floatingPointBV2RMDecl(ps, domain, ASTHandle{})
		} else if k == astFPAAdd || k == astFPASub || k == astFPAMul || k == astFPADiv {
			decl, e = m.floatingPointRMBinaryDecl(k, domain)
		} else if k == astFPARem || k == astFPAMin || k == astFPAMax {
			decl, e = m.floatingPointBinaryDecl(k, domain)
		} else if k == astFPASqrt || k == astFPARoundToIntegral {
			decl, e = m.floatingPointRMUnaryDecl(k, domain)
		} else if k == astFPAFMA {
			decl, e = m.floatingPointFMADecl(domain)
		} else if k >= astFPAEq && k <= astFPAGe {
			decl, e = m.floatingPointBinaryRelationDecl(k, domain)
		} else if k >= astFPAIsNaN {
			decl, e = m.floatingPointClassificationDecl(k, domain)
		} else if k <= astFPARMTowardZero {
			decl, e = m.floatingPointRoundingDecl(k, ps, domain)
		} else {
			decl, e = m.floatingPointSpecialDecl(k, ps, ASTHandle{})
		}
	} else if m.pluginAt(fid).kind == astFormatPlugin {
		decl, e = m.formatDecl(k, ps, domain)
	} else if m.pluginAt(fid).kind == astDatalogPlugin {
		decl, e = m.datalogDecl(k, ps, domain, ASTHandle{})
	} else if m.pluginAt(fid).kind == astPseudoBooleanPlugin {
		decl, e = m.pseudoBooleanDecl(k, ps, domain)
	} else if m.pluginAt(fid).kind == astSequencePlugin {
		decl, e = m.sequenceDecl(k, ps, domain, ASTHandle{})
	} else if m.pluginAt(fid).kind == astDatatypePlugin {
		decl, e = m.datatypeDecl(k, ps, domain, ASTHandle{})
	} else {
		decl, e = m.builtinDecl(fid, k, ps, domain)
	}
	if e != nil || decl.slot == 0 {
		return astNodeRef{}, e
	}
	return m.mkAppArguments(decl, args)
}
func (m *ASTManager) MkFamilyApp(fid FamilyID, k int32, ps []ASTParameter, args []ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.mkFamilyApp(fid, k, ps, args)
	return m.external(r), e
}

// The eager base plugins ignore optional range; array const declarations use it.
func (m *ASTManager) MkFamilyFuncDecl(fid FamilyID, k int32, ps []ASTParameter, domain []ASTHandle, rangeSort ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.mkFamilyFuncDecl(fid, k, ps, domain, rangeSort)
}

func (m *ASTManager) mkFamilyFuncDecl(fid FamilyID, k int32, ps []ASTParameter, domain []ASTHandle, rangeSort ASTHandle) (ASTHandle, error) {
	if fid < 0 || !m.pluginAt(fid).present {
		return ASTHandle{}, nil
	}
	if uint64(len(domain)) > math.MaxUint32 {
		return ASTHandle{}, arena.ErrBounds
	}
	var r astNodeRef
	var e error
	if m.pluginAt(fid).kind == astArithmeticPlugin {
		previous := m.formatExecuting
		m.formatExecuting = true
		defer func() { m.formatExecuting = previous }()
		r, e = m.arithmeticDecl(k, ps, domain)
	} else if fid == 0 {
		r, e = m.basicDecl(k, ps, domain)
	} else if m.pluginAt(fid).kind == astArrayPlugin {
		r, e = m.arrayDecl(k, ps, domain, rangeSort)
	} else if m.pluginAt(fid).kind == astFloatingPointPlugin && k >= 0 && (k <= astFPAMinusZero || k >= astFPAAdd && k <= astFPARoundToIntegral || k >= astFPAEq && k <= astFPAGe || k >= astFPAIsNaN && k <= astFPAIsPositive || k == astFPAFP || k == astFPAToFP || k == astFPAToFPUnsigned || k == astFPAToUBV || k == astFPAToSBV || k == astFPAToReal || k == astFPAToIEEEBV || k == astFPABVWrap || k == astFPABV2RM) {
		previous := m.formatExecuting
		m.formatExecuting = true
		defer func() { m.formatExecuting = previous }()
		if k == astFPANeg || k == astFPAAbs {
			r, e = m.floatingPointUnaryDecl(k, domain)
		} else if k == astFPAToReal {
			r, e = m.floatingPointToRealDecl(domain)
		} else if k == astFPAToUBV || k == astFPAToSBV {
			r, e = m.floatingPointToBVDecl(k, ps, domain)
		} else if k == astFPAToFPUnsigned {
			r, e = m.floatingPointToFPUnsignedDecl(ps, domain)
		} else if k == astFPAFP {
			r, e = m.floatingPointFPDecl(domain)
		} else if k == astFPAToFP {
			r, e = m.floatingPointToFPDecl(ps, domain)
		} else if k == astFPAToIEEEBV {
			r, e = m.floatingPointToIEEEBVDecl(domain)
		} else if k == astFPABVWrap {
			r, e = m.floatingPointBVWrapDecl(ps, domain)
		} else if k == astFPABV2RM {
			r, e = m.floatingPointBV2RMDecl(ps, domain, rangeSort)
		} else if k == astFPAAdd || k == astFPASub || k == astFPAMul || k == astFPADiv {
			r, e = m.floatingPointRMBinaryDecl(k, domain)
		} else if k == astFPARem || k == astFPAMin || k == astFPAMax {
			r, e = m.floatingPointBinaryDecl(k, domain)
		} else if k == astFPASqrt || k == astFPARoundToIntegral {
			r, e = m.floatingPointRMUnaryDecl(k, domain)
		} else if k == astFPAFMA {
			r, e = m.floatingPointFMADecl(domain)
		} else if k >= astFPAEq && k <= astFPAGe {
			r, e = m.floatingPointBinaryRelationDecl(k, domain)
		} else if k >= astFPAIsNaN {
			r, e = m.floatingPointClassificationDecl(k, domain)
		} else if k <= astFPARMTowardZero {
			r, e = m.floatingPointRoundingDecl(k, ps, domain)
		} else {
			r, e = m.floatingPointSpecialDecl(k, ps, rangeSort)
		}
	} else if m.pluginAt(fid).kind == astFormatPlugin {
		r, e = m.formatDecl(k, ps, domain)
	} else if m.pluginAt(fid).kind == astDatalogPlugin {
		r, e = m.datalogDecl(k, ps, domain, rangeSort)
	} else if m.pluginAt(fid).kind == astPseudoBooleanPlugin {
		r, e = m.pseudoBooleanDecl(k, ps, domain)
	} else if m.pluginAt(fid).kind == astSequencePlugin {
		r, e = m.sequenceDecl(k, ps, domain, rangeSort)
	} else if m.pluginAt(fid).kind == astDatatypePlugin {
		r, e = m.datatypeDecl(k, ps, domain, rangeSort)
	} else {
		r, e = m.builtinDecl(fid, k, ps, domain)
	}
	return m.external(r), e
}

func (m *ASTManager) MkFamilySort(fid FamilyID, k int32, ps []ASTParameter) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.mkFamilySort(fid, k, ps)
}

func (m *ASTManager) mkFamilySort(fid FamilyID, k int32, ps []ASTParameter) (ASTHandle, error) {
	if fid < 0 || !m.pluginAt(fid).present {
		return ASTHandle{}, nil
	}
	var r astNodeRef
	var e error
	if m.pluginAt(fid).kind == astBitVectorPlugin {
		r, e = m.bitVectorSort(ps)
		return m.external(r), e
	}
	if m.pluginAt(fid).kind == astDatalogPlugin {
		r, e = m.datalogSort(fid, k, ps)
		return m.external(r), e
	}
	if m.pluginAt(fid).kind == astFormatPlugin {
		if ASTSourceDebug && k != 0 {
			return ASTHandle{}, ErrASTDomain
		}
		return m.external(m.formatPlugin.sort), nil
	}

	if m.pluginAt(fid).kind == astFloatingPointPlugin {
		m.formatExecuting = true
		defer func() { m.formatExecuting = false }()
		r, e = m.floatingPointSort(k, ps)
		return m.external(r), e
	}
	if m.pluginAt(fid).kind == astSequencePlugin {
		m.formatExecuting = true
		defer func() { m.formatExecuting = false }()
		if e = m.initializeSequence(m.sequencePlugin); e != nil {
			return ASTHandle{}, e
		}
		var param *astParameterRecord
		if k != 2 {
			if len(ps) != 1 {
				return ASTHandle{}, ErrASTDomain
			}
			param, e = m.parameter(ps[0])
			if e != nil {
				return ASTHandle{}, e
			}
		}
		r, e = m.sequenceSort(m.sequencePlugin, k, param)
		return m.external(r), e
	}
	if m.pluginAt(fid).kind == astArrayPlugin {
		m.formatExecuting = true
		defer func() { m.formatExecuting = false }()
		r, e = m.arraySort(k, ps)
		return m.external(r), e
	}
	if m.pluginAt(fid).kind == astDatatypePlugin {
		r, e = m.datatypeSort(k, ps)
		return m.external(r), e
	}
	if m.pluginAt(fid).kind == astArithmeticPlugin {
		if k == 0 || k == 1 {
			return m.external(m.arithmeticPlugin.fields[k]), nil
		}
		return ASTHandle{}, nil
	}
	switch fid {
	case 0:
		if k == 0 {
			r = m.basic.boolSort
		} else {
			if ASTSourceDebug && k != 1 {
				return ASTHandle{}, ErrASTDomain
			}
			r = m.basic.proofSort
		}
	case 4:
		r, e = m.userSort(k, ps)
	default:
		return ASTHandle{}, ErrASTDomain // source UNREACHABLE followed by null
	}
	return m.external(r), e
}

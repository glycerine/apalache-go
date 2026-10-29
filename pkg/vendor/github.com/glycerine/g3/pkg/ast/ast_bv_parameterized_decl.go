package ast

// Source: src/ast/bv_decl_plugin.cpp:155-164,350-403,530-576,584-628.

import (
	"math"

	"github.com/glycerine/g3/pkg/util/z3exception"
)

func bvDeclarationError(message string) error {
	return ASTException{z3exception.NewDefaultException(message)}
}

func (m *ASTManager) bitVectorSortWidth(fid FamilyID, h ASTHandle) (int32, bool, error) {
	r, e := m.requireKind(h, ASTSort)
	if e != nil {
		return 0, false, e
	}
	info, e := m.sortInfo(m.sortAt(r.slot).info)
	if e != nil {
		return 0, false, e
	}
	if info.family != fid || info.kind != 0 {
		return 0, false, nil
	}
	p, e := m.infoParameter(info, 0)
	if e != nil {
		return 0, false, e
	}
	v, e := m.parameter(p)
	if e != nil {
		return 0, false, e
	}
	if v.Kind != ASTParameterInt {
		return 0, false, ErrASTDomain
	}
	return v.integer, true, nil
}

// Large widths bypass the sort cache in the source. In particular, a negative
// signed repeat result converts to unsigned here without a huge cache resize.
func (p *astBitVectorPluginOwner) getSort(width uint32) (astNodeRef, error) {
	if width < 1<<12 {
		return p.mkSort(width)
	}
	m := p.owner
	parameter := astParameterRecord{Kind: ASTParameterInt, integer: int32(width)}
	if e := m.beginFormatInfo(true, p.family, 0, &parameter); e != nil {
		return astNodeRef{}, e
	}
	defer m.endFormatInfo()
	m.formatLexical.info.size = VeryBigSortSize()
	return m.mkSort(p.names[0], ASTSortInfo{m.origin, ^uint32(0) - 1, m.formatLexical.epoch})
}

// Checked adapter only for signed arithmetic which is undefined on overflow in
// C++. Defined negative and zero results retain their unsigned conversion.
func bvCheckedSize(n int64) (uint32, error) {
	if n < math.MinInt32 || n > math.MaxInt32 {
		return 0, ErrASTDomain
	}
	return uint32(int32(n)), nil
}

func (m *ASTManager) bitVectorParameterizedDecl(k int32, ps []ASTParameter, domain []ASTHandle, width int32) (astNodeRef, error) {
	p := m.bitVectorPlugin
	var size uint32
	var nameIndex int
	var e error
	kept := ps
	switch k {
	case 55:
		return m.bitVectorBit2Bool(uint32(width), ps, domain)
	case 57, 58:
		return p.conversionDecl(k, uint32(width), ps, domain)
	case 37:
		nameIndex = 1
		var total int64
		for _, s := range domain {
			n, ok, e := m.bitVectorSortWidth(p.family, s)
			if e != nil {
				return astNodeRef{}, e
			}
			if !ok {
				return astNodeRef{}, bvDeclarationError("invalid concat application")
			}
			total += int64(n)
			// Source adds after every sort read, not at the end of the loop.
			if _, e = bvCheckedSize(total); e != nil {
				return astNodeRef{}, e
			}
		}
		size, e = bvCheckedSize(total)
		kept = nil // concat deliberately discards all parameters
	case 38, 39:
		nameIndex = int(k - 36)
		message := "invalid sign_extend application"
		if k == 39 {
			message = "invalid zero_extend application"
		}
		if len(domain) != 1 || len(ps) != 1 {
			return astNodeRef{}, bvDeclarationError(message)
		}
		v, err := m.parameter(ps[0])
		if err != nil {
			return astNodeRef{}, err
		}
		if v.Kind != ASTParameterInt || v.integer < 0 {
			return astNodeRef{}, bvDeclarationError(message)
		}
		size, e = bvCheckedSize(int64(width) + int64(v.integer))
	case 40:
		nameIndex = 4
		if len(domain) != 1 || len(ps) != 2 {
			return astNodeRef{}, bvDeclarationError("invalid extract application")
		}
		high, err := m.parameter(ps[0])
		if err != nil {
			return astNodeRef{}, err
		}
		if high.Kind != ASTParameterInt {
			return astNodeRef{}, bvDeclarationError("invalid extract application")
		}
		low, err := m.parameter(ps[1])
		if err != nil {
			return astNodeRef{}, err
		}
		if low.Kind != ASTParameterInt || low.integer > high.integer || high.integer >= width {
			return astNodeRef{}, bvDeclarationError("invalid extract application")
		}
		size, e = bvCheckedSize(int64(high.integer) - int64(low.integer) + 1)
	case 41:
		nameIndex = 7
		if len(domain) != 1 {
			return astNodeRef{}, bvDeclarationError("repeat expects one argument")
		}
		if len(ps) != 1 {
			return astNodeRef{}, bvDeclarationError("repeat expects one nonzero integer parameter")
		}
		v, err := m.parameter(ps[0])
		if err != nil {
			return astNodeRef{}, err
		}
		if v.Kind != ASTParameterInt || v.integer == 0 {
			return astNodeRef{}, bvDeclarationError("repeat expects one nonzero integer parameter")
		}
		size, e = bvCheckedSize(int64(width) * int64(v.integer))
	case 48, 49:
		nameIndex = int(k - 43)
		if len(domain) != 1 {
			message := "rotate left expects one argument"
			if k == 49 {
				message = "rotate right expects one argument"
			}
			return astNodeRef{}, bvDeclarationError(message)
		}
		result, err := m.requireKind(domain[0], ASTSort)
		if err != nil {
			return astNodeRef{}, err
		}
		return m.pluginDecl(p.names[nameIndex], p.family, k, ps, domain, result, ASTFuncFlags{}, false)
	default:
		return astNodeRef{}, nil
	}
	if e != nil {
		return astNodeRef{}, e
	}
	result, e := p.getSort(size)
	if e != nil {
		return astNodeRef{}, e
	}
	return m.pluginDecl(p.names[nameIndex], p.family, k, kept, domain, result, ASTFuncFlags{}, false)
}

// Return handled=false only where the source calls the base domain-buffer
// overload. Fixed cached operators never build that buffer.
func (m *ASTManager) bitVectorExprDecl(k int32, ps []ASTParameter, args []ASTHandle) (astNodeRef, bool, error) {
	return m.bitVectorExprDeclArguments(k, ps, SliceArguments(args))
}

func (m *ASTManager) bitVectorExprDeclArguments(k int32, ps []ASTParameter, args AppArguments) (astNodeRef, bool, error) {
	p := m.bitVectorPlugin
	if k == 57 {
		_, found, e := m.int2BVSize(ps)
		if e != nil {
			return astNodeRef{}, true, e
		}
		if found {
			return astNodeRef{}, false, nil
		}
	}
	switch k {
	case 1:
		return p.fields[1], true, nil
	case 2:
		return p.fields[0], true, nil
	case 59:
		return p.fields[2], true, nil
	case 60:
		return p.fields[3], true, nil
	case 0:
		decl, e := m.bitVectorNumeralDecl(ps, int(args.count))
		return decl, true, e
	case 56:
		return astNodeRef{}, false, nil
	}
	if args.count == 0 {
		return astNodeRef{}, true, bvDeclarationError("operator is applied to arguments of the wrong sort")
	}
	first, e := args.at(0)
	if e != nil {
		return astNodeRef{}, true, e
	}
	r, e := m.requireExpr(first)
	if e != nil {
		return astNodeRef{}, true, e
	}
	sort, e := m.sortOf(r)
	if e != nil {
		return astNodeRef{}, true, e
	}
	width, ok, e := m.bitVectorSortWidth(p.family, m.external(sort))
	if e != nil {
		return astNodeRef{}, true, e
	}
	if !ok {
		return astNodeRef{}, true, bvDeclarationError("operator is applied to arguments of the wrong sort")
	}
	decl, e := p.fixedDecl(k, uint32(width))
	if e != nil {
		return astNodeRef{}, true, e
	}
	if decl.slot == 0 {
		return astNodeRef{}, false, nil
	}
	record := m.declAt(decl.slot)
	flags, e := m.funcFlags(decl)
	if e != nil {
		return astNodeRef{}, true, e
	}
	count := int(args.count)
	mismatch := count != int(record.arity)
	associative := flags.LeftAssociative && flags.RightAssociative
	if mismatch && !associative {
		return astNodeRef{}, true, bvDeclarationError("declared arity mismatches supplied arity")
	}
	for i := uint32(0); i < args.count; i++ {
		h, e := args.at(i)
		if e != nil {
			return astNodeRef{}, true, e
		}
		argument, e := m.requireExpr(h)
		if e != nil {
			return astNodeRef{}, true, e
		}
		actual, e := m.sortOf(argument)
		if e != nil {
			return astNodeRef{}, true, e
		}
		index := i
		if mismatch && associative {
			index = 0
		}
		expected, e := m.readInternal(record.domain, index)
		if e != nil {
			return astNodeRef{}, true, e
		}
		if actual != expected {
			if mismatch && associative {
				return astNodeRef{}, true, bvDeclarationError("declared sorts do not match supplied sorts")
			}
			// This source branch formats both ASTs via mk_pp into an ostringstream.
			// Preserve rejection; full SMT2 diagnostic formatting remains required.
			return astNodeRef{}, true, ErrASTDomain
		}
	}
	return decl, true, nil
}

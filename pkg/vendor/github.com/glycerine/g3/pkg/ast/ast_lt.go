package ast

// Source: src/ast/ast_lt.cpp:22-155; src/ast/ast_lt.h:23-35.

import (
	"errors"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// ASTLess follows the original field comparisons and first differing AST tail
// transition. It does not replace structural order with allocation IDs.
func (m *ASTManager) ASTLess(a, b ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	x, e := m.internal(a)
	if e != nil {
		return false, e
	}
	y, e := m.internal(b)
	if e != nil {
		return false, e
	}
	return m.astLess(x, y)
}
func (m *ASTManager) ASTLexLess(a, b AppArguments) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	if a.Count() != b.Count() {
		return false, ErrASTDomain
	}
	for i := uint32(0); i < a.Count(); i++ {
		x, e := a.At(i)
		if e != nil {
			return false, e
		}
		y, e := b.At(i)
		if e != nil {
			return false, e
		}
		if x == y {
			continue
		}
		l, e := m.internal(x)
		if e != nil {
			return false, e
		}
		r, e := m.internal(y)
		if e != nil {
			return false, e
		}
		return m.astLess(l, r)
	}
	return false, nil
}
func (m *ASTManager) ASTIsSorted(args AppArguments) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	for i := uint32(1); i < args.Count(); i++ {
		a, e := args.At(i)
		if e != nil {
			return false, e
		}
		b, e := args.At(i - 1)
		if e != nil {
			return false, e
		}
		x, e := m.internal(a)
		if e != nil {
			return false, e
		}
		y, e := m.internal(b)
		if e != nil {
			return false, e
		}
		less, e := m.astLess(x, y)
		if e != nil || less {
			return false, e
		}
	}
	return true, nil
}

type astLTParameterResult struct {
	decided, less bool
	first, second astNodeRef
}

func astLTParameterCount(info *astInfoRecord) uint32 {
	if info == nil {
		return 0
	}
	return info.count
}
func (m *ASTManager) astLTParameters(a, b *astInfoRecord) (astLTParameterResult, error) {
	for i := uint32(0); i < astLTParameterCount(a); i++ {
		pa, e := m.infoParameter(a, i)
		if e != nil {
			return astLTParameterResult{}, e
		}
		pb, e := m.infoParameter(b, i)
		if e != nil {
			return astLTParameterResult{}, e
		}
		result, e := m.astLTParameterPair(pa, pb)
		if e != nil || result.decided || result.first.slot != 0 {
			return result, e
		}
	}
	return astLTParameterResult{}, nil
}

// The source copies both parameter objects before comparing their fields.
// These real parameter owners retain rational payloads through lexical cleanup.
func (m *ASTManager) astLTParameterPair(pa, pb ASTParameter) (result astLTParameterResult, err error) {
	left, e := m.copyParameter(pa)
	if e != nil {
		return result, e
	}
	defer func() { err = errors.Join(err, m.dropParameter(left)) }()
	right, e := m.copyParameter(pb)
	if e != nil {
		return result, e
	}
	defer func() { err = errors.Join(err, m.dropParameter(right)) }()
	pa, pb = left, right
	x, e := m.parameter(pa)
	if e != nil {
		return astLTParameterResult{}, e
	}
	y, e := m.parameter(pb)
	if e != nil {
		return astLTParameterResult{}, e
	}
	if x.Kind != y.Kind {
		return astLTParameterResult{decided: true, less: x.Kind < y.Kind}, nil
	}
	result = astLTParameterResult{decided: true}
	switch x.Kind {
	case ASTParameterInt:
		if x.integer == y.integer {
			return astLTParameterResult{}, nil
		}
		result.less = x.integer < y.integer
	case ASTParameterAST:
		if x.ast == y.ast {
			return astLTParameterResult{}, nil
		}
		return astLTParameterResult{first: x.ast, second: y.ast}, nil
	case ASTParameterSymbol:
		if x.symbol == y.symbol {
			return astLTParameterResult{}, nil
		}
		result.less, e = m.Symbols.Less(x.symbol, y.symbol)
	case ASTParameterRational:
		same, err := m.numerals.Equal(&x.Rational, &y.Rational)
		if err != nil {
			return astLTParameterResult{}, err
		}
		if same {
			return astLTParameterResult{}, nil
		}
		result.less, e = m.numerals.Less(&x.Rational, &y.Rational)
	case ASTParameterDouble:
		if x.double == y.double {
			return astLTParameterResult{}, nil
		}
		result.less = x.double < y.double
	case ASTParameterExternal:
		if x.external == y.external {
			return astLTParameterResult{}, nil
		}
		result.less = x.external < y.external
	default:
		return astLTParameterResult{}, ErrASTDomain
	}
	return result, e
}
func (m *ASTManager) astLTInfo(r astNodeRef) (*astInfoRecord, error) {
	if m.nodeAt(r.slot).kind == ASTSort {
		p := m.sortAt(r.slot).info
		if p == (ASTSortInfo{}) {
			return nil, nil
		}
		return m.sortInfo(p)
	}
	p := m.declAt(r.slot).info
	if p == (ASTFuncDeclInfo{}) {
		return nil, nil
	}
	return m.funcInfo(p)
}
func (m *ASTManager) astLess(a, b astNodeRef) (bool, error) {
start:
	for {
		if e := m.validNode(a); e != nil {
			return false, e
		}
		if e := m.validNode(b); e != nil {
			return false, e
		}
		if a == b {
			return false, nil
		}
		kind := m.nodeAt(a.slot).kind
		otherKind := m.nodeAt(b.slot).kind
		if kind != otherKind {
			return kind < otherKind, nil
		}
		switch kind {
		case ASTSort, ASTFuncDecl:
			var nameA, nameB usymbol.Symbol
			if kind == ASTSort {
				nameA = m.sortAt(a.slot).name
				nameB = m.sortAt(b.slot).name
			} else {
				nameA = m.declAt(a.slot).name
				nameB = m.declAt(b.slot).name
			}
			if nameA != nameB {
				return m.Symbols.Less(nameA, nameB)
			}
			if kind == ASTFuncDecl {
				arityA, arityB := m.declAt(a.slot).arity, m.declAt(b.slot).arity
				if arityA != arityB {
					return arityA < arityB, nil
				}
			}
			ia, e := m.astLTInfo(a)
			if e != nil {
				return false, e
			}
			ib, e := m.astLTInfo(b)
			if e != nil {
				return false, e
			}
			na, nb := astLTParameterCount(ia), astLTParameterCount(ib)
			if na != nb {
				return na < nb, nil
			}
			if kind == ASTSort && ASTSourceDebug && na == 0 {
				return false, ErrASTDomain
			}
			parameter, e := m.astLTParameters(ia, ib)
			if e != nil {
				return false, e
			}
			if parameter.first.slot != 0 {
				a, b = parameter.first, parameter.second
				continue start
			}
			if parameter.decided {
				return parameter.less, nil
			}
			if kind == ASTSort {
				if ASTSourceDebug {
					return false, ErrASTDomain
				}
				return false, nil
			}
			x, y := m.declAt(a.slot), m.declAt(b.slot)
			for i := uint32(0); i < x.arity; i++ {
				left, e := m.readInternal(x.domain, i)
				if e != nil {
					return false, e
				}
				right, e := m.readInternal(y.domain, i)
				if e != nil {
					return false, e
				}
				if left != right {
					a, b = left, right
					continue start
				}
			}
			a, b = x.rangeSort, y.rangeSort
		case ASTApp:
			x, y := m.appAt(a.slot), m.appAt(b.slot)
			if x.count != y.count {
				return x.count < y.count, nil
			}
			if x.depth != y.depth {
				return x.depth < y.depth, nil
			}
			if x.decl != y.decl {
				a, b = x.decl, y.decl
				continue start
			}
			for i := uint32(0); i < x.count; i++ {
				left, e := m.readInternal(x.args, i)
				if e != nil {
					return false, e
				}
				right, e := m.readInternal(y.args, i)
				if e != nil {
					return false, e
				}
				if left != right {
					a, b = left, right
					continue start
				}
			}
			if ASTSourceDebug {
				return false, ErrASTDomain
			}
			return false, nil
		case ASTVar:
			x, y := m.varAt(a.slot), m.varAt(b.slot)
			if x.index != y.index {
				return x.index < y.index, nil
			}
			a, b = x.sort, y.sort
		case ASTQuantifier:
			x, y := m.quantifierAt(a.slot), m.quantifierAt(b.slot)
			if x.forall != y.forall {
				return !x.forall && y.forall, nil
			}
			if x.count != y.count {
				return x.count < y.count, nil
			}
			if x.patternCount != y.patternCount {
				return x.patternCount < y.patternCount, nil
			}
			if x.noPatternCount != y.noPatternCount {
				return x.noPatternCount < y.noPatternCount, nil
			}
			if x.weight != y.weight {
				return uint32(x.weight) < uint32(y.weight), nil
			}
			for i := uint32(0); i < x.count; i++ {
				left, e := m.readSymbol(x.names, i)
				if e != nil {
					return false, e
				}
				right, e := m.readSymbol(y.names, i)
				if e != nil {
					return false, e
				}
				if left != right {
					return m.Symbols.Less(left, right)
				}
				ls, e := m.readInternal(x.sorts, i)
				if e != nil {
					return false, e
				}
				rs, e := m.readInternal(y.sorts, i)
				if e != nil {
					return false, e
				}
				if ls != rs {
					a, b = ls, rs
					continue start
				}
			}
			for i := uint32(0); i < x.patternCount; i++ {
				left, e := m.readInternal(x.patterns, i)
				if e != nil {
					return false, e
				}
				right, e := m.readInternal(y.patterns, i)
				if e != nil {
					return false, e
				}
				if left != right {
					a, b = left, right
					continue start
				}
			}
			for i := uint32(0); i < x.noPatternCount; i++ {
				left, e := m.readInternal(x.noPatterns, i)
				if e != nil {
					return false, e
				}
				right, e := m.readInternal(y.noPatterns, i)
				if e != nil {
					return false, e
				}
				if left != right {
					a, b = left, right
					continue start
				}
			}
			a, b = x.body, y.body
		default:
			return false, ErrASTDomain
		}
	}
}

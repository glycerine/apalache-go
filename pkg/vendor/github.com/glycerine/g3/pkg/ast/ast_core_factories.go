package ast

// Source: src/ast/ast.cpp:2064-2211.
// Source node construction. Results are non-owning source RC0 handles.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (m *ASTManager) requireKind(h ASTHandle, kind ASTKind) (astNodeRef, error) {
	r, e := m.internal(h)
	if e != nil {
		return r, e
	}
	if m.nodeAt(r.slot).kind != kind {
		return astNodeRef{}, ErrASTDomain
	}
	return r, nil
}
func (m *ASTManager) requireExpr(h ASTHandle) (astNodeRef, error) {
	r, e := m.internal(h)
	if e != nil {
		return r, e
	}
	if m.nodeAt(r.slot).kind > ASTQuantifier {
		return astNodeRef{}, ErrASTDomain
	}
	return r, nil
}
func (m *ASTManager) mkSort(name usymbol.Symbol, info ASTSortInfo) (astNodeRef, error) {
	if info != (ASTSortInfo{}) {
		if _, e := m.sortInfo(info); e != nil {
			return astNodeRef{}, e
		}
	}
	r, e := m.allocateNode(ASTSort)
	if e != nil {
		return r, e
	}
	*m.sortAt(r.slot) = astSortRecord{name: name, info: info}
	return m.registerNode(r)
}
func (m *ASTManager) MkSort(name usymbol.Symbol, info ASTSortInfo) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.mkSort(name, info)
	return m.external(r), e
}
func (m *ASTManager) mkFuncDecl(name usymbol.Symbol, domain []ASTHandle, rangeSort ASTHandle, info ASTFuncDeclInfo) (astNodeRef, error) {
	return m.mkFuncDeclInfo(name, domain, rangeSort, info, true)
}
func (m *ASTManager) mkFuncDeclInfo(name usymbol.Symbol, domain []ASTHandle, rangeSort ASTHandle, info ASTFuncDeclInfo, collapseNull bool) (astNodeRef, error) {
	if uint64(len(domain)) > math.MaxUint32 {
		return astNodeRef{}, arena.ErrBounds
	}
	rangeRef, e := m.requireKind(rangeSort, ASTSort)
	if e != nil {
		return astNodeRef{}, e
	}
	if info != (ASTFuncDeclInfo{}) {
		v, e := m.funcInfo(info)
		if e != nil {
			return astNodeRef{}, e
		}
		f := v.flags
		if ASTSourceDebug && ((len(domain) != 1 && f.Injective) || (len(domain) != 2 && (f.LeftAssociative && f.RightAssociative || f.Commutative))) {
			return astNodeRef{}, ErrASTDomain
		}
		if collapseNull && astFuncInfoNull(v) {
			info = ASTFuncDeclInfo{}
		}
	}
	for _, s := range domain {
		if _, e = m.requireKind(s, ASTSort); e != nil {
			return astNodeRef{}, e
		}
	}
	r, e := m.allocateNode(ASTFuncDecl)
	if e != nil {
		return r, e
	}
	v := m.declAt(r.slot)
	v.name = name
	v.info = info
	v.arity = uint32(len(domain))
	v.rangeSort = rangeRef
	v.domain, e = m.newASTPayload(domain)
	if e != nil {
		m.freeNodeStorage(r)
		return astNodeRef{}, e
	}
	return m.registerNode(r)
}
func (m *ASTManager) MkFuncDecl(name usymbol.Symbol, domain []ASTHandle, rangeSort ASTHandle, info ASTFuncDeclInfo) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.mkFuncDecl(name, domain, rangeSort, info)
	return m.external(r), e
}

// MkFuncDeclWithInfo translates the source pointer overload. A nonzero
// metadata token is preserved even when info.is_null() is true.
func (m *ASTManager) MkFuncDeclWithInfo(name usymbol.Symbol, domain []ASTHandle, rangeSort ASTHandle, info ASTFuncDeclInfo) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.mkFuncDeclInfo(name, domain, rangeSort, info, false)
	return m.external(r), e
}
func (m *ASTManager) MkConst(name usymbol.Symbol, sort ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	d, e := m.mkFuncDecl(name, nil, sort, ASTFuncDeclInfo{})
	if e != nil {
		return ASTHandle{}, e
	}
	r, e := m.mkApp(d, nil)
	return m.external(r), e
}
func (m *ASTManager) MkBoolSort() (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.external(m.boolSort), nil
}
func (m *ASTManager) MkVar(index uint32, sort ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	s, e := m.requireKind(sort, ASTSort)
	if e != nil {
		return ASTHandle{}, e
	}
	r, e := m.allocateNode(ASTVar)
	if e != nil {
		return ASTHandle{}, e
	}
	*m.varAt(r.slot) = astVarRecord{index: index, sort: s}
	original := r
	r, e = m.registerNode(r)
	if e == nil && r == original {
		e = m.traceVar(r)
	}
	return m.external(r), e
}
func (m *ASTManager) funcFlags(decl astNodeRef) (ASTFuncFlags, error) {
	if e := m.validNode(decl); e != nil {
		return ASTFuncFlags{}, e
	}
	if m.nodeAt(decl.slot).kind != ASTFuncDecl {
		return ASTFuncFlags{}, ErrASTDomain
	}
	p := m.declAt(decl.slot).info
	if p == (ASTFuncDeclInfo{}) {
		return ASTFuncFlags{}, nil
	}
	info, e := m.funcInfo(p)
	if e != nil {
		return ASTFuncFlags{}, e
	}
	return info.flags, nil
}
func (m *ASTManager) mkAppCore(decl astNodeRef, args []ASTHandle) (astNodeRef, error) {
	if uint64(len(args)) > math.MaxUint32 {
		return astNodeRef{}, arena.ErrBounds
	}
	return m.mkAppCoreArguments(decl, AppArguments{slice: args, count: uint32(len(args))})
}
func (m *ASTManager) mkAppCoreArguments(decl astNodeRef, args AppArguments) (result astNodeRef, err error) {
	frame, prepareError := m.prepareTraceAcquisition(args.count)
	if prepareError != nil {
		return astNodeRef{}, prepareError
	}
	defer func() { m.finishTraceAcquisition(frame) }()
	// Source allocates app storage before coercion and check_args.
	r, e := m.allocateNode(ASTApp)
	if e != nil {
		return r, e
	}
	v := m.appAt(r.slot)
	v.decl = decl
	v.count = args.count
	flags, e := m.funcFlags(decl)
	if e != nil {
		m.freeNodeStorage(r)
		return astNodeRef{}, e
	}
	d := m.declAt(decl.slot)
	needed, e := m.appCoercionNeeded(decl, args, flags.LeftAssociative && flags.RightAssociative)
	if e != nil {
		m.freeNodeStorage(r)
		return astNodeRef{}, e
	}
	if needed {
		if m.appCoercion.active {
			m.freeNodeStorage(r)
			return astNodeRef{}, ErrASTPending
		}
		defer func() { err = errors.Join(err, m.cleanupAppCoercion(false)) }()
		args, e = m.coerceAppArguments(decl, args, flags.LeftAssociative && flags.RightAssociative)
		if e != nil {
			m.freeNodeStorage(r)
			return astNodeRef{}, e
		}
	}
	for i := uint32(0); i < args.count; i++ {
		h, e := args.at(i)
		if e != nil {
			m.freeNodeStorage(r)
			return astNodeRef{}, e
		}
		child, e := m.requireExpr(h)
		if e != nil {
			m.freeNodeStorage(r)
			return astNodeRef{}, e
		}
		actual, e := m.sortOf(child)
		if e != nil {
			m.freeNodeStorage(r)
			return astNodeRef{}, e
		}
		index := uint32(i)
		if flags.LeftAssociative && flags.RightAssociative {
			index = 0
		}
		if index >= d.arity {
			m.freeNodeStorage(r)
			return astNodeRef{}, ErrASTDomain
		}
		expected, e := m.readInternal(d.domain, index)
		if e != nil {
			m.freeNodeStorage(r)
			return astNodeRef{}, e
		}
		if actual != expected {
			m.freeNodeStorage(r)
			return astNodeRef{}, ErrASTDomain
		}
	}
	v.args, e = m.newASTPayloadArguments(args)
	if e != nil {
		m.freeNodeStorage(r)
		return astNodeRef{}, e
	}
	original := r
	if frame != nil {
		frame.node = r
	}
	r, e = m.registerNode(r)
	if e == nil && r == original {
		if frame != nil {
			frame.hash = m.nodeAt(r.slot).hash
		}
		e = m.traceApp(r)
		if e != nil {
			e = m.catchTraceAllocation(frame, e)
		}
	}
	return r, e
}
func (m *ASTManager) mkApp(decl astNodeRef, args []ASTHandle) (astNodeRef, error) {
	if uint64(len(args)) > math.MaxUint32 {
		return astNodeRef{}, arena.ErrBounds
	}
	return m.mkAppArguments(decl, AppArguments{slice: args, count: uint32(len(args))})
}
func (m *ASTManager) mkAppArguments(decl astNodeRef, args AppArguments) (result astNodeRef, cause error) {
	f, e := m.funcFlags(decl)
	if e != nil {
		return astNodeRef{}, e
	}
	arity := m.declAt(decl.slot).arity
	mismatch := uint64(arity) != uint64(int(args.count))
	fid, _, e := m.declIdentity(decl)
	if e != nil {
		return astNodeRef{}, e
	}
	if mismatch && (!f.LeftAssociative && !f.RightAssociative && !f.Chainable || int(args.count) < 2 && fid == 0 && !(f.LeftAssociative && f.RightAssociative)) {
		return astNodeRef{}, ErrASTDomain
	}
	defer func() {
		if cause == nil && result.slot != 0 {
			cause = m.traceAppGround(result)
		}
	}()
	if int(args.count) == 1 && f.Chainable && arity == 2 {
		return m.trueApp, nil
	}
	if int(args.count) > 2 && !f.FlatAssociative {
		var pair [2]ASTHandle
		if f.RightAssociative {
			j := int(args.count) - 1
			pair[0], e = args.at(uint32(j - 1))
			if e != nil {
				return astNodeRef{}, e
			}
			pair[1], e = args.at(uint32(j))
			if e != nil {
				return astNodeRef{}, e
			}
			r, e := m.mkAppCore(decl, pair[:])
			if e != nil {
				return r, e
			}
			j--
			for j > 0 {
				j--
				pair[0], e = args.at(uint32(j))
				if e != nil {
					return astNodeRef{}, e
				}
				pair[1] = m.external(r)
				r, e = m.mkAppCore(decl, pair[:])
				if e != nil {
					return r, e
				}
			}
			return r, nil
		}
		if f.LeftAssociative {
			pair[0], e = args.at(0)
			if e != nil {
				return astNodeRef{}, e
			}
			pair[1], e = args.at(1)
			if e != nil {
				return astNodeRef{}, e
			}
			r, e := m.mkAppCore(decl, pair[:])
			if e != nil {
				return r, e
			}
			for i := 2; i < int(args.count); i++ {
				pair[0] = m.external(r)
				pair[1], e = args.at(uint32(i))
				if e != nil {
					return astNodeRef{}, e
				}
				r, e = m.mkAppCore(decl, pair[:])
				if e != nil {
					return r, e
				}
			}
			return r, nil
		}
		if f.Chainable {
			m.traceChainableApp()
			var inline [16]ASTHandle
			buffer := inline[:0]
			for i := 1; i < int(args.count); i++ {
				pair[0], e = args.at(uint32(i - 1))
				if e != nil {
					return astNodeRef{}, e
				}
				pair[1], e = args.at(uint32(i))
				if e != nil {
					return astNodeRef{}, e
				}
				r, e := m.mkAppCore(decl, pair[:])
				if e != nil {
					return r, e
				}
				buffer = append(buffer, m.external(r))
			}
			return m.mkFamilyApp(0, ASTOpAnd, nil, buffer)
		}
	}
	return m.mkAppCoreArguments(decl, args)
}
func (m *ASTManager) MkApp(decl ASTHandle, args []ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	d, e := m.requireKind(decl, ASTFuncDecl)
	if e != nil {
		return ASTHandle{}, e
	}
	r, e := m.mkApp(d, args)
	return m.external(r), e
}
func (m *ASTManager) MkQuantifier(spec ASTQuantifierSpec) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if len(spec.Sorts) != len(spec.Names) || (len(spec.Patterns) != 0 && len(spec.NoPatterns) != 0) || ASTSourceDebug && len(spec.Sorts) == 0 {
		return ASTHandle{}, ErrASTDomain
	}
	for _, s := range spec.Sorts {
		if _, e := m.requireKind(s, ASTSort); e != nil {
			return ASTHandle{}, e
		}
	}
	body, e := m.requireExpr(spec.Body)
	if e != nil {
		return ASTHandle{}, e
	}
	for _, hs := range [][]ASTHandle{spec.Patterns, spec.NoPatterns} {
		for _, h := range hs {
			if _, e = m.requireExpr(h); e != nil {
				return ASTHandle{}, e
			}
		}
	}
	if ASTSourceDebug {
		for i, h := range spec.Patterns {
			// Source traces each pattern immediately before asserting that one.
			// The selected SMT2 printer acquires this manager's lock itself.
			m.mu.Unlock()
			traceErr := m.traceQuantifierPattern(i, h)
			m.mu.Lock()
			if traceErr != nil {
				return ASTHandle{}, traceErr
			}
			r, e := m.requireExpr(h)
			if e != nil {
				return ASTHandle{}, e
			}
			if m.nodeAt(r.slot).kind != ASTApp {
				return ASTHandle{}, ErrASTDomain
			}
			fid, k, e := m.declIdentity(m.appAt(r.slot).decl)
			if e != nil {
				return ASTHandle{}, e
			}
			if fid != 2 || k != 0 {
				return ASTHandle{}, ErrASTDomain
			}
			a := m.appAt(r.slot)
			for i := uint32(0); i < a.count; i++ {
				child, e := m.readInternal(a.args, i)
				if e != nil {
					return ASTHandle{}, e
				}
				if m.nodeAt(child.slot).kind != ASTApp {
					return ASTHandle{}, ErrASTDomain
				}
			}
		}
	}
	if _, e = m.Symbols.FromToken(spec.QID.Token()); e != nil {
		return ASTHandle{}, e
	}
	if _, e = m.Symbols.FromToken(spec.SkolemID.Token()); e != nil {
		return ASTHandle{}, e
	}
	for _, n := range []int{len(spec.Sorts), len(spec.Patterns), len(spec.NoPatterns)} {
		if uint64(n) > math.MaxUint32 {
			return ASTHandle{}, arena.ErrBounds
		}
	}
	r, e := m.allocateNode(ASTQuantifier)
	if e != nil {
		return ASTHandle{}, e
	}
	v := m.quantifierAt(r.slot)
	v.body = body
	v.forall = spec.Forall
	v.count = uint32(len(spec.Sorts))
	v.patternCount = uint32(len(spec.Patterns))
	v.noPatternCount = uint32(len(spec.NoPatterns))
	v.weight = spec.Weight
	v.qid = spec.QID
	v.skolemID = spec.SkolemID
	v.depth = m.depth(body) + 1
	v.unusedVars = true
	v.labels = m.hasLabels(body)
	if v.sorts, e = m.newASTPayload(spec.Sorts); e == nil {
		v.names, e = m.newSymbolPayload(spec.Names)
	}
	if e == nil {
		v.patterns, e = m.newASTPayload(spec.Patterns)
	}
	if e == nil {
		v.noPatterns, e = m.newASTPayload(spec.NoPatterns)
	}
	if e != nil {
		m.freeNodeStorage(r)
		return ASTHandle{}, e
	}
	original := r
	r, e = m.registerNode(r)
	if e == nil && r == original {
		e = m.traceQuantifier(r)
	}
	return m.external(r), e
}

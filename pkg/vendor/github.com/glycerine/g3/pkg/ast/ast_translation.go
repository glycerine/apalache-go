package ast

// Source: src/ast/ast_translation.cpp; src/ast/ast_translation.h.

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type astTranslationParameter struct {
	kind     ASTParameterKind
	integer  int32
	ast      ASTHandle
	symbol   usymbol.Symbol
	rational unumeral.OwnedRational
	double   float64
	external ASTParameter
}

type astTranslationInfo struct {
	present bool
	family  FamilyID
	kind    int32
	params  []astTranslationParameter
	size    SortSize
	flags   ASTFuncFlags
	private bool
}

type astTranslationNode struct {
	kind             ASTKind
	refCount         uint32
	name             usymbol.Symbol
	info             astTranslationInfo
	children         []ASTHandle
	index            uint32
	forall           bool
	declarationCount uint32
	patternCount     uint32
	noPatternCount   uint32
	declarationNames []usymbol.Symbol
	weight           int32
	qid, skolemID    usymbol.Symbol
}

// ASTTranslation translates manager-owned AST DAGs and retains the source
// cache policy: only nodes with more than one source reference remain pinned
// between calls.
type ASTTranslation struct {
	from, to *ASTManager
	cache    map[ASTHandle]ASTHandle
	persist  bool
	closed   bool
}

func NewASTTranslation(from, to *ASTManager, copyPlugins bool) (*ASTTranslation, error) {
	return newASTTranslation(from, to, copyPlugins, true)
}

func newASTTranslation(from, to *ASTManager, copyPlugins, persist bool) (*ASTTranslation, error) {
	if from == nil || to == nil || from == to {
		return nil, ErrASTHandle
	}
	t := &ASTTranslation{from: from, to: to, cache: make(map[ASTHandle]ASTHandle), persist: persist}
	if copyPlugins {
		if err := to.CopyFamiliesPlugins(from); err != nil {
			return nil, err
		}
		if err := t.copyDatatypeDefinitions(); err != nil {
			_ = t.Close()
			return nil, err
		}
	}
	return t, nil
}

func (t *ASTTranslation) From() *ASTManager { return t.from }
func (t *ASTTranslation) To() *ASTManager   { return t.to }

func (t *ASTTranslation) ResetCache() error {
	if t == nil || t.closed {
		return ErrASTHandle
	}
	var result error
	for source, target := range t.cache {
		result = errors.Join(result, t.from.DecRef(source), t.to.DecRef(target))
		delete(t.cache, source)
	}
	return result
}

func (t *ASTTranslation) Close() error {
	if t == nil || t.closed {
		return nil
	}
	err := t.ResetCache()
	t.closed = true
	t.from, t.to, t.cache = nil, nil, nil
	return err
}

func (t *ASTTranslation) Translate(source ASTHandle) (ASTHandle, error) {
	if t == nil || t.closed || t.from == nil || t.to == nil {
		return ASTHandle{}, ErrASTHandle
	}
	if source.IsNull() {
		return ASTHandle{}, nil
	}
	if result, found := t.cache[source]; found {
		return result, nil
	}

	nodes := make(map[ASTHandle]astTranslationNode)
	seen := make(map[ASTHandle]bool)
	order := make([]ASTHandle, 0, 64)
	type visit struct {
		node     ASTHandle
		expanded bool
	}
	stack := []visit{{node: source}}
	for len(stack) != 0 {
		last := len(stack) - 1
		item := stack[last]
		stack = stack[:last]
		if _, cached := t.cache[item.node]; cached {
			continue
		}
		if item.expanded {
			order = append(order, item.node)
			continue
		}
		if seen[item.node] {
			continue
		}
		seen[item.node] = true
		node, err := t.snapshotNode(item.node)
		if err != nil {
			return ASTHandle{}, err
		}
		nodes[item.node] = node
		stack = append(stack, visit{node: item.node, expanded: true})
		for i := len(node.children) - 1; i >= 0; i-- {
			child := node.children[i]
			if _, cached := t.cache[child]; !cached && !seen[child] {
				stack = append(stack, visit{node: child})
			}
		}
	}

	results := make(map[ASTHandle]ASTHandle, len(order))
	lookup := func(h ASTHandle) (ASTHandle, error) {
		if r, found := results[h]; found {
			return r, nil
		}
		if r, found := t.cache[h]; found {
			return r, nil
		}
		return ASTHandle{}, ErrASTDomain
	}
	for _, h := range order {
		result, err := t.buildNode(nodes[h], lookup)
		if err != nil {
			return ASTHandle{}, err
		}
		results[h] = result
		if t.persist && nodes[h].refCount > 1 {
			if err = t.from.IncRef(h); err != nil {
				return ASTHandle{}, err
			}
			if err = t.to.IncRef(result); err != nil {
				_ = t.from.DecRef(h)
				return ASTHandle{}, err
			}
			t.cache[h] = result
		}
	}
	return lookup(source)
}

// TranslateAST performs one translation with a nonpersistent cache.
func TranslateAST(source ASTHandle, from, to *ASTManager) (result ASTHandle, err error) {
	t, err := newASTTranslation(from, to, true, false)
	if err != nil {
		return ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, t.Close()) }()
	return t.Translate(source)
}

func (t *ASTTranslation) snapshotNode(h ASTHandle) (astTranslationNode, error) {
	m := t.from
	if err := m.lock(); err != nil {
		return astTranslationNode{}, err
	}
	defer m.mu.Unlock()
	r, err := m.internal(h)
	if err != nil {
		return astTranslationNode{}, err
	}
	n := astTranslationNode{kind: m.nodeAt(r.slot).kind, refCount: m.nodeAt(r.slot).rc}
	appendChild := func(child astNodeRef) { n.children = append(n.children, m.external(child)) }
	switch n.kind {
	case ASTSort:
		v := m.sortAt(r.slot)
		n.name = v.name
		n.info, err = m.snapshotInfo(v.info.slot, v.info.generation)
		if err == nil {
			for _, p := range n.info.params {
				if p.kind == ASTParameterAST {
					n.children = append(n.children, p.ast)
				}
			}
		}
	case ASTFuncDecl:
		v := m.declAt(r.slot)
		n.name = v.name
		n.info, err = m.snapshotInfo(v.info.slot, v.info.generation)
		if err == nil {
			for _, p := range n.info.params {
				if p.kind == ASTParameterAST {
					n.children = append(n.children, p.ast)
				}
			}
			for i := uint32(0); i < v.arity; i++ {
				var child astNodeRef
				child, err = m.readInternal(v.domain, i)
				if err != nil {
					break
				}
				appendChild(child)
			}
			appendChild(v.rangeSort)
		}
	case ASTVar:
		v := m.varAt(r.slot)
		n.index = v.index
		appendChild(v.sort)
	case ASTApp:
		v := m.appAt(r.slot)
		appendChild(v.decl)
		for i := uint32(0); i < v.count && err == nil; i++ {
			var child astNodeRef
			child, err = m.readInternal(v.args, i)
			if err == nil {
				appendChild(child)
			}
		}
	case ASTQuantifier:
		v := m.quantifierAt(r.slot)
		n.forall, n.declarationCount, n.patternCount, n.noPatternCount = v.forall, v.count, v.patternCount, v.noPatternCount
		n.weight, n.qid, n.skolemID = v.weight, v.qid, v.skolemID
		for i := uint32(0); i < v.count && err == nil; i++ {
			var child astNodeRef
			child, err = m.readInternal(v.sorts, i)
			if err == nil {
				appendChild(child)
			}
			if err == nil {
				var name usymbol.Symbol
				name, err = m.readSymbol(v.names, i)
				n.declarationNames = append(n.declarationNames, name)
			}
		}
		appendChild(v.body)
		for i := uint32(0); i < v.patternCount && err == nil; i++ {
			var child astNodeRef
			child, err = m.readInternal(v.patterns, i)
			if err == nil {
				appendChild(child)
			}
		}
		for i := uint32(0); i < v.noPatternCount && err == nil; i++ {
			var child astNodeRef
			child, err = m.readInternal(v.noPatterns, i)
			if err == nil {
				appendChild(child)
			}
		}
	default:
		err = ErrASTDomain
	}
	return n, err
}

func (m *ASTManager) snapshotInfo(slot, generation uint32) (astTranslationInfo, error) {
	if slot == 0 {
		return astTranslationInfo{}, nil
	}
	r, err := m.info(slot, generation)
	if err != nil {
		return astTranslationInfo{}, err
	}
	result := astTranslationInfo{present: true, family: r.family, kind: r.kind, size: r.size, flags: r.flags, private: r.private}
	for i := uint32(0); i < r.count; i++ {
		p, e := m.infoParameter(r, i)
		if e != nil {
			return astTranslationInfo{}, e
		}
		v, e := m.parameter(p)
		if e != nil {
			return astTranslationInfo{}, e
		}
		copy := astTranslationParameter{kind: v.Kind, integer: v.integer, ast: m.external(v.ast), symbol: v.symbol, rational: v.Rational, double: v.double}
		if v.Kind == ASTParameterExternal {
			copy.external = p
		}
		result.params = append(result.params, copy)
	}
	return result, nil
}

func (t *ASTTranslation) translateSymbol(s usymbol.Symbol) (usymbol.Symbol, error) {
	if s.IsNull() || s.IsNumerical() {
		return s, nil
	}
	text, err := t.from.Symbols.String(s)
	if err != nil {
		return usymbol.Symbol{}, err
	}
	return t.to.Symbols.Intern(text)
}

func (t *ASTTranslation) buildParameters(info astTranslationInfo, lookup func(ASTHandle) (ASTHandle, error)) ([]ASTParameter, error) {
	result := make([]ASTParameter, 0, len(info.params))
	fail := func(err error) ([]ASTParameter, error) {
		for i := len(result) - 1; i >= 0; i-- {
			err = errors.Join(err, t.to.DelParameter(&result[i]))
		}
		return nil, err
	}
	for _, source := range info.params {
		var p ASTParameter
		var err error
		switch source.kind {
		case ASTParameterInt:
			p, err = t.to.NewParameterInt(source.integer)
		case ASTParameterAST:
			var h ASTHandle
			h, err = lookup(source.ast)
			if err == nil {
				p, err = t.to.NewParameterAST(h)
			}
		case ASTParameterSymbol:
			var s usymbol.Symbol
			s, err = t.translateSymbol(source.symbol)
			if err == nil {
				p, err = t.to.NewParameterSymbol(s)
			}
		case ASTParameterRational:
			var q unumeral.OwnedRational
			q, err = t.to.numerals.NewValue()
			if err == nil {
				err = t.to.numerals.CopyFrom(&q, t.from.numerals, &source.rational)
			}
			if err == nil {
				p, err = t.to.NewParameterRational(&q)
			}
			if q != (unumeral.OwnedRational{}) {
				err = errors.Join(err, t.to.numerals.Del(&q))
			}
		case ASTParameterDouble:
			p, err = t.to.NewParameterDouble(source.double)
		case ASTParameterExternal:
			if t.from.arithmeticPlugin != nil && info.family == t.from.arithmeticPlugin.family {
				p, err = t.from.TranslateArithmeticExternalParameter(source.external, t.to)
			} else if t.from.floatingPointPlugin != nil && info.family == t.from.floatingPointPlugin.family {
				p, err = t.from.TranslateFloatingPointExternalParameter(source.external, t.to)
			} else {
				err = ErrASTDomain
			}
		default:
			err = ErrASTDomain
		}
		if err != nil {
			return fail(err)
		}
		result = append(result, p)
	}
	return result, nil
}

// TranslateDependency linearizes the source dependency DAG, translates its
// unique expression leaves in source order, and rebuilds the target join.
func (t *ASTTranslation) TranslateDependency(source ASTDependency) (ASTDependency, error) {
	if t == nil || t.closed || t.from == nil || t.to == nil {
		return ASTDependency{}, ErrASTHandle
	}
	if source.IsNull() {
		return ASTDependency{}, nil
	}
	leaves, err := t.from.DependencyLinearize(source, nil)
	if err != nil {
		return ASTDependency{}, err
	}
	translated := make([]ASTHandle, len(leaves))
	for i, leaf := range leaves {
		translated[i], err = t.Translate(leaf)
		if err != nil {
			return ASTDependency{}, err
		}
	}
	progress, err := t.to.DependencyJoinValues(translated)
	if err != nil {
		return progress.Completed, err
	}
	return progress.Completed, nil
}

func (t *ASTTranslation) buildNode(n astTranslationNode, lookup func(ASTHandle) (ASTHandle, error)) (result ASTHandle, err error) {
	child := func(i int) (ASTHandle, error) {
		if i < 0 || i >= len(n.children) {
			return ASTHandle{}, ErrASTDomain
		}
		return lookup(n.children[i])
	}
	switch n.kind {
	case ASTVar:
		sort, e := child(0)
		if e != nil {
			return ASTHandle{}, e
		}
		return t.to.MkVar(n.index, sort)
	case ASTApp:
		decl, e := child(0)
		if e != nil {
			return ASTHandle{}, e
		}
		args := make([]ASTHandle, len(n.children)-1)
		for i := range args {
			args[i], e = child(i + 1)
			if e != nil {
				return ASTHandle{}, e
			}
		}
		return t.to.MkApp(decl, args)
	case ASTSort, ASTFuncDecl:
		name, e := t.translateSymbol(n.name)
		if e != nil {
			return ASTHandle{}, e
		}
		if n.kind == ASTSort && !n.info.present {
			return t.to.MkUninterpretedSort(name, nil)
		}
		params, e := t.buildParameters(n.info, lookup)
		if e != nil {
			return ASTHandle{}, e
		}
		defer func() {
			for i := len(params) - 1; i >= 0; i-- {
				err = errors.Join(err, t.to.DelParameter(&params[i]))
			}
		}()
		if n.kind == ASTSort {
			info, e := t.to.NewSortInfo(n.info.family, n.info.kind, params, n.info.size, n.info.private)
			if e != nil {
				return ASTHandle{}, e
			}
			defer func() { err = errors.Join(err, t.to.DelSortInfo(&info)) }()
			return t.to.MkSort(name, info)
		}
		extra := 0
		for _, p := range n.info.params {
			if p.kind == ASTParameterAST {
				extra++
			}
		}
		domainCount := len(n.children) - extra - 1
		if domainCount < 0 {
			return ASTHandle{}, ErrASTDomain
		}
		domain := make([]ASTHandle, domainCount)
		for i := range domain {
			domain[i], e = child(extra + i)
			if e != nil {
				return ASTHandle{}, e
			}
		}
		rangeSort, e := child(len(n.children) - 1)
		if e != nil {
			return ASTHandle{}, e
		}
		if !n.info.present {
			return t.to.MkFuncDecl(name, domain, rangeSort, ASTFuncDeclInfo{})
		}
		info, e := t.to.NewFuncDeclInfo(n.info.family, n.info.kind, params, n.info.flags, n.info.private)
		if e != nil {
			return ASTHandle{}, e
		}
		defer func() { err = errors.Join(err, t.to.DelFuncDeclInfo(&info)) }()
		return t.to.MkFuncDeclWithInfo(name, domain, rangeSort, info)
	case ASTQuantifier:
		if uint64(n.declarationCount)+1+uint64(n.patternCount)+uint64(n.noPatternCount) != uint64(len(n.children)) {
			return ASTHandle{}, ErrASTDomain
		}
		spec := ASTQuantifierSpec{Forall: n.forall, Weight: n.weight}
		for i := uint32(0); i < n.declarationCount; i++ {
			h, e := child(int(i))
			if e != nil {
				return ASTHandle{}, e
			}
			spec.Sorts = append(spec.Sorts, h)
			name, e := t.translateSymbol(n.declarationNames[i])
			if e != nil {
				return ASTHandle{}, e
			}
			spec.Names = append(spec.Names, name)
		}
		bodyIndex := int(n.declarationCount)
		spec.Body, err = child(bodyIndex)
		if err != nil {
			return ASTHandle{}, err
		}
		for i := uint32(0); i < n.patternCount; i++ {
			h, e := child(bodyIndex + 1 + int(i))
			if e != nil {
				return ASTHandle{}, e
			}
			spec.Patterns = append(spec.Patterns, h)
		}
		for i := uint32(0); i < n.noPatternCount; i++ {
			h, e := child(bodyIndex + 1 + int(n.patternCount+i))
			if e != nil {
				return ASTHandle{}, e
			}
			spec.NoPatterns = append(spec.NoPatterns, h)
		}
		spec.QID, err = t.translateSymbol(n.qid)
		if err == nil {
			spec.SkolemID, err = t.translateSymbol(n.skolemID)
		}
		if err != nil {
			return ASTHandle{}, err
		}
		return t.to.MkQuantifier(spec)
	default:
		return ASTHandle{}, ErrASTDomain
	}
}

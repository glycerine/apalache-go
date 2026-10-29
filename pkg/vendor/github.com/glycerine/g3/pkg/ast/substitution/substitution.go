package substitution

// Source: src/ast/substitution/substitution.cpp;
// src/ast/substitution/substitution.h.

import (
	"bytes"
	"errors"
	"math"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type substitutionState uint8

const (
	substitutionClean substitutionState = iota
	substitutionApply
	substitutionInsert
)

// VariableOffset identifies one substitution-domain binding.
type VariableOffset struct {
	Variable uint32
	Offset   uint32
}

type applyKey struct {
	term  ExprOffset
	depth uint32
}

// Substitution maps de Bruijn variable/offset pairs to expression/offset
// pairs and owns every expression stored in its range.
type Substitution struct {
	self      *Substitution
	manager   *ast.ASTManager
	bindings  varOffsetExprMap
	order     []VariableOffset
	refs      []ast.ASTRef
	scopes    []int
	cache     map[applyKey]ast.ASTHandle
	cacheRefs []ast.ASTRef
	state     substitutionState
	closed    bool
}

func New(m *ast.ASTManager) (*Substitution, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	s := &Substitution{manager: m, cache: make(map[applyKey]ast.ASTHandle)}
	s.self = s
	return s, nil
}

func (s *Substitution) open() error {
	if s == nil || s.self != s || s.manager == nil || s.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (s *Substitution) Reserve(offsets, variables uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	if len(s.order) != 0 {
		return ast.ErrASTPending
	}
	return s.bindings.reserve(offsets, variables)
}

func (s *Substitution) ReserveOffsets(offsets uint32) error {
	return s.Reserve(offsets, s.bindings.variables)
}

func (s *Substitution) ReserveVariables(variables uint32) error {
	return s.Reserve(s.bindings.offsets, variables)
}

func (s *Substitution) release(refs *[]ast.ASTRef) error {
	for len(*refs) != 0 {
		i := len(*refs) - 1
		if err := s.manager.DelRef(&(*refs)[i]); err != nil {
			return err
		}
		*refs = (*refs)[:i]
	}
	return nil
}

func (s *Substitution) ResetCache() error {
	if err := s.open(); err != nil {
		return err
	}
	clear(s.cache)
	if err := s.release(&s.cacheRefs); err != nil {
		return err
	}
	s.state = substitutionClean
	return nil
}

func (s *Substitution) Reset() error {
	if err := s.open(); err != nil {
		return err
	}
	s.bindings.reset()
	s.order, s.scopes = nil, nil
	if err := s.release(&s.refs); err != nil {
		return err
	}
	return s.ResetCache()
}

func (s *Substitution) PushScope() error {
	if err := s.open(); err != nil {
		return err
	}
	s.scopes = append(s.scopes, len(s.order))
	return nil
}

func (s *Substitution) PopScope(count uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	if count == 0 || uint64(count) > uint64(len(s.scopes)) {
		return ast.ErrASTDomain
	}
	newLevel := len(s.scopes) - int(count)
	oldSize := s.scopes[newLevel]
	for i := oldSize; i < len(s.order); i++ {
		v := s.order[i]
		if err := s.bindings.erase(v.Variable, v.Offset); err != nil {
			return err
		}
	}
	s.order = s.order[:oldSize]
	s.scopes = s.scopes[:newLevel]
	for len(s.refs) > oldSize {
		i := len(s.refs) - 1
		if err := s.manager.DelRef(&s.refs[i]); err != nil {
			return err
		}
		s.refs = s.refs[:i]
	}
	return s.ResetCache()
}

func (s *Substitution) ScopeLevel() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return uint32(len(s.scopes)), nil
}

func (s *Substitution) TopScopeHasBindings() (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	start := 0
	if len(s.scopes) != 0 {
		start = s.scopes[len(s.scopes)-1]
	}
	return start < len(s.order), nil
}

func (s *Substitution) NumBindings() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return uint32(len(s.order)), nil
}

func (s *Substitution) Insert(variable, offset uint32, target ExprOffset) error {
	if err := s.open(); err != nil {
		return err
	}
	if target.expr.IsNull() {
		return ast.ErrASTHandle
	}
	r, err := s.manager.NewRef(target.expr)
	if err != nil {
		return err
	}
	if err = s.bindings.insert(variable, offset, target); err != nil {
		return errors.Join(err, s.manager.DelRef(&r))
	}
	s.order = append(s.order, VariableOffset{variable, offset})
	s.refs = append(s.refs, r)
	s.state = substitutionInsert
	return nil
}

func (s *Substitution) InsertVariable(variable ast.ASTHandle, offset uint32, target ExprOffset) error {
	i, err := s.manager.GetVarIndex(variable)
	if err != nil {
		return err
	}
	return s.Insert(i, offset, target)
}

func (s *Substitution) Find(variable, offset uint32) (ExprOffset, bool, error) {
	if err := s.open(); err != nil {
		return ExprOffset{}, false, err
	}
	return s.bindings.find(variable, offset)
}

func (s *Substitution) Contains(variable ast.ASTHandle, offset uint32) (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	index, err := s.manager.GetVarIndex(variable)
	if err != nil {
		return false, err
	}
	_, found, err := s.Find(index, offset)
	return found, err
}

func (s *Substitution) Binding(index uint32) (VariableOffset, ExprOffset, error) {
	if err := s.open(); err != nil {
		return VariableOffset{}, ExprOffset{}, err
	}
	if uint64(index) >= uint64(len(s.order)) {
		return VariableOffset{}, ExprOffset{}, ast.ErrASTDomain
	}
	v := s.order[index]
	r, ok, err := s.bindings.find(v.Variable, v.Offset)
	if err != nil {
		return VariableOffset{}, ExprOffset{}, err
	}
	if !ok {
		return VariableOffset{}, ExprOffset{}, ast.ErrASTDomain
	}
	return v, r, nil
}

func (s *Substitution) keep(h ast.ASTHandle) (ast.ASTHandle, error) {
	r, err := s.manager.NewRef(h)
	if err == nil {
		s.cacheRefs = append(s.cacheRefs, r)
	}
	return h, err
}

func add32(a, b uint32) (uint32, error) {
	if b > math.MaxUint32-a {
		return 0, ast.ErrASTDomain
	}
	return a + b, nil
}

func (s *Substitution) shift(h ast.ASTHandle, amount, bound uint32, memo map[applyKey]ast.ASTHandle) (ast.ASTHandle, error) {
	key := applyKey{NewExprOffset(h, amount), bound}
	if v, ok := memo[key]; ok {
		return v, nil
	}
	kind, err := s.manager.GetKind(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result := h
	switch kind {
	case ast.ASTVar:
		i, err := s.manager.GetVarIndex(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if i >= bound {
			i, err = add32(i, amount)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			sort, err := s.manager.GetSort(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			result, err = s.manager.MkVar(i, sort)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			result, err = s.keep(result)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
	case ast.ASTApp:
		n, err := s.manager.GetNumArgs(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		args := make([]ast.ASTHandle, n)
		changed := false
		for i := range args {
			old, err := s.manager.GetArg(h, uint32(i))
			if err != nil {
				return ast.ASTHandle{}, err
			}
			args[i], err = s.shift(old, amount, bound, memo)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			changed = changed || args[i] != old
		}
		if changed {
			d, err := s.manager.GetDecl(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			result, err = s.manager.MkApp(d, args)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			result, err = s.keep(result)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
	case ast.ASTQuantifier:
		n, err := s.manager.GetNumDeclarations(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		inner, err := add32(bound, n)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		result, err = s.shiftQuantifier(h, amount, inner, memo)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	default:
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	memo[key] = result
	return result, nil
}

func (s *Substitution) shiftQuantifier(q ast.ASTHandle, amount, bound uint32, memo map[applyKey]ast.ASTHandle) (ast.ASTHandle, error) {
	pn, err := s.manager.GetNumPatterns(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	patterns := make([]ast.ASTHandle, pn)
	changed := false
	for i := range patterns {
		old, e := s.manager.GetPattern(q, uint32(i))
		if e != nil {
			return ast.ASTHandle{}, e
		}
		patterns[i], e = s.shift(old, amount, bound, memo)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		changed = changed || patterns[i] != old
	}
	nn, err := s.manager.GetNumNoPatterns(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	noPatterns := make([]ast.ASTHandle, nn)
	for i := range noPatterns {
		old, e := s.manager.GetNoPattern(q, uint32(i))
		if e != nil {
			return ast.ASTHandle{}, e
		}
		noPatterns[i], e = s.shift(old, amount, bound, memo)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		changed = changed || noPatterns[i] != old
	}
	body, err := s.manager.GetQuantifierBody(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	newBody, err := s.shift(body, amount, bound, memo)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !changed && newBody == body {
		return q, nil
	}
	r, err := s.manager.UpdateQuantifierAnnotations(q, patterns, noPatterns, newBody)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return s.keep(r)
}

func (s *Substitution) apply(n, replace, replacement ExprOffset, depth uint32, deltas []uint32, active map[applyKey]bool) (ast.ASTHandle, error) {
	key := applyKey{n, depth}
	if h, ok := s.cache[key]; ok {
		return h, nil
	}
	if active[key] {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	active[key] = true
	defer delete(active, key)
	current := n
	if n == replace && !replace.expr.IsNull() {
		current = replacement
	}
	kind, err := s.manager.GetKind(current.expr)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result := current.expr
	switch kind {
	case ast.ASTVar:
		idx, err := s.manager.GetVarIndex(current.expr)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if idx >= depth {
			if target, ok, e := s.bindings.find(idx-depth, current.offset); e != nil {
				return ast.ASTHandle{}, e
			} else if ok {
				if depth != 0 {
					target.expr, e = s.shift(target.expr, depth, 0, make(map[applyKey]ast.ASTHandle))
					if e != nil {
						return ast.ASTHandle{}, e
					}
				}
				result, err = s.apply(target, replace, replacement, depth, deltas, active)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				break
			}
		}
		if int(current.offset) >= len(deltas) {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		if deltas[current.offset] != 0 {
			idx, err = add32(idx, deltas[current.offset])
			if err != nil {
				return ast.ASTHandle{}, err
			}
			sort, e := s.manager.GetSort(current.expr)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			result, e = s.manager.MkVar(idx, sort)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			result, e = s.keep(result)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
	case ast.ASTApp:
		count, e := s.manager.GetNumArgs(current.expr)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		args := make([]ast.ASTHandle, count)
		changed := false
		for i := range args {
			old, e := s.manager.GetArg(current.expr, uint32(i))
			if e != nil {
				return ast.ASTHandle{}, e
			}
			args[i], e = s.apply(NewExprOffset(old, current.offset), replace, replacement, depth, deltas, active)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			changed = changed || args[i] != old
		}
		if changed {
			d, e := s.manager.GetDecl(current.expr)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			result, e = s.manager.MkApp(d, args)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			result, e = s.keep(result)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
	case ast.ASTQuantifier:
		count, e := s.manager.GetNumDeclarations(current.expr)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		inner, e := add32(depth, count)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		var shiftedReplace, shiftedReplacement ExprOffset
		if !replace.expr.IsNull() {
			shiftedReplace = replace
			shiftedReplacement = replacement
			shiftedReplace.expr, e = s.shift(replace.expr, count, 0, make(map[applyKey]ast.ASTHandle))
			if e != nil {
				return ast.ASTHandle{}, e
			}
			shiftedReplacement.expr, e = s.shift(replacement.expr, count, 0, make(map[applyKey]ast.ASTHandle))
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		result, e = s.applyQuantifier(current.expr, current.offset, shiftedReplace, shiftedReplacement, inner, deltas, active)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	default:
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	s.cache[key] = result
	return result, nil
}

func (s *Substitution) applyQuantifier(q ast.ASTHandle, offset uint32, replace, replacement ExprOffset, depth uint32, deltas []uint32, active map[applyKey]bool) (ast.ASTHandle, error) {
	pn, e := s.manager.GetNumPatterns(q)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	ps := make([]ast.ASTHandle, pn)
	changed := false
	for i := range ps {
		old, e := s.manager.GetPattern(q, uint32(i))
		if e != nil {
			return ast.ASTHandle{}, e
		}
		ps[i], e = s.apply(NewExprOffset(old, offset), replace, replacement, depth, deltas, active)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		changed = changed || ps[i] != old
	}
	nn, e := s.manager.GetNumNoPatterns(q)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	ns := make([]ast.ASTHandle, nn)
	for i := range ns {
		old, e := s.manager.GetNoPattern(q, uint32(i))
		if e != nil {
			return ast.ASTHandle{}, e
		}
		ns[i], e = s.apply(NewExprOffset(old, offset), replace, replacement, depth, deltas, active)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		changed = changed || ns[i] != old
	}
	body, e := s.manager.GetQuantifierBody(q)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	nb, e := s.apply(NewExprOffset(body, offset), replace, replacement, depth, deltas, active)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	if !changed && nb == body {
		return q, nil
	}
	r, e := s.manager.UpdateQuantifierAnnotations(q, ps, ns, nb)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return s.keep(r)
}

func (s *Substitution) Apply(offsets uint32, deltas []uint32, n ExprOffset, out ast.ASTRef) (result error) {
	return s.ApplyModulo(offsets, deltas, n, ExprOffset{}, ExprOffset{}, out)
}

func (s *Substitution) ApplyExpression(n ast.ASTHandle, out ast.ASTRef) error {
	return s.Apply(1, []uint32{0}, NewExprOffset(n, 0), out)
}

func (s *Substitution) ApplyModulo(offsets uint32, deltas []uint32, n, replace, replacement ExprOffset, out ast.ASTRef) (result error) {
	if err := s.open(); err != nil {
		return err
	}
	if uint64(offsets) > uint64(len(deltas)) {
		return ast.ErrASTDomain
	}
	deltas = deltas[:offsets]
	modulo := !replace.expr.IsNull()
	if s.state == substitutionInsert || modulo {
		if err := s.ResetCache(); err != nil {
			return err
		}
	}
	s.state = substitutionApply
	defer func() {
		if modulo {
			result = errors.Join(result, s.ResetCache())
		}
	}()
	h, err := s.apply(n, replace, replacement, 0, deltas, make(map[applyKey]bool))
	if err != nil {
		return err
	}
	// m_new_exprs retains the result of every source apply call, including an
	// unchanged input whose descendants are borrowed by the persistent cache.
	if _, err = s.keep(h); err != nil {
		return err
	}
	return s.manager.SetRef(out, h)
}

func (s *Substitution) Acyclic() (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	colors := make(map[ExprOffset]uint8)
	var visit func(ExprOffset) (bool, error)
	visit = func(p ExprOffset) (bool, error) {
		if colors[p] == 2 {
			return true, nil
		}
		if colors[p] == 1 {
			return false, nil
		}
		colors[p] = 1
		kind, e := s.manager.GetKind(p.expr)
		if e != nil {
			return false, e
		}
		children := []ExprOffset{}
		switch kind {
		case ast.ASTVar:
			i, e := s.manager.GetVarIndex(p.expr)
			if e != nil {
				return false, e
			}
			if q, ok, e := s.bindings.find(i, p.offset); e != nil {
				return false, e
			} else if ok && q != p {
				children = append(children, q)
			}
		case ast.ASTApp:
			n, e := s.manager.GetNumArgs(p.expr)
			if e != nil {
				return false, e
			}
			for i := uint32(0); i < n; i++ {
				h, e := s.manager.GetArg(p.expr, i)
				if e != nil {
					return false, e
				}
				children = append(children, NewExprOffset(h, p.offset))
			}
		default:
			return false, ast.ErrASTDomain
		}
		for _, c := range children {
			ok, e := visit(c)
			if e != nil || !ok {
				return ok, e
			}
		}
		colors[p] = 2
		return true, nil
	}
	for _, v := range s.order {
		r, ok, e := s.bindings.find(v.Variable, v.Offset)
		if e != nil {
			return false, e
		}
		if ok {
			yes, e := visit(r)
			if e != nil || !yes {
				return yes, e
			}
		}
	}
	return true, nil
}

func (s *Substitution) format(h ast.ASTHandle) (value string, result error) {
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: s.manager.Budget()})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	doc, err := s.manager.SMT2PPExprTree(h, nil)
	if err != nil {
		return "", err
	}
	if err = s.manager.PrintFormat(out, doc, nil, nil, nil); err != nil {
		return "", err
	}
	return out.String()
}

// Display returns the source normalized diagnostic form.
func (s *Substitution) Display(offsets uint32, deltas []uint32) (string, error) {
	if err := s.open(); err != nil {
		return "", err
	}
	if uint64(offsets) > uint64(len(deltas)) {
		return "", ast.ErrASTDomain
	}
	if err := s.ResetCache(); err != nil {
		return "", err
	}
	var out bytes.Buffer
	ref, err := s.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return "", err
	}
	defer s.manager.DelRef(&ref)
	for i := uint32(0); i < offsets; i++ {
		for j := uint32(0); j < s.bindings.variables; j++ {
			r, ok, e := s.bindings.find(j, i)
			if e != nil {
				return "", e
			}
			if !ok {
				continue
			}
			if e = s.Apply(offsets, deltas, r, ref); e != nil {
				return "", e
			}
			h, e := s.manager.RefGet(ref)
			if e != nil {
				return "", e
			}
			text, e := s.format(h)
			if e != nil {
				return "", e
			}
			out.WriteString("VAR ")
			out.WriteString(strconv.FormatUint(uint64(j), 10))
			out.WriteByte(':')
			out.WriteString(strconv.FormatUint(uint64(i), 10))
			out.WriteString(" -->\n")
			out.WriteString(text)
			out.WriteByte('\n')
		}
	}
	return out.String(), nil
}

// DisplayRaw returns the source diagnostic form without normalization.
func (s *Substitution) DisplayRaw() (string, error) {
	if err := s.open(); err != nil {
		return "", err
	}
	var out bytes.Buffer
	for i := uint32(0); i < s.bindings.offsets; i++ {
		for j := uint32(0); j < s.bindings.variables; j++ {
			r, ok, e := s.bindings.find(j, i)
			if e != nil {
				return "", e
			}
			if !ok {
				continue
			}
			text, e := s.format(r.expr)
			if e != nil {
				return "", e
			}
			out.WriteString("VAR ")
			out.WriteString(strconv.FormatUint(uint64(j), 10))
			out.WriteByte(':')
			out.WriteString(strconv.FormatUint(uint64(i), 10))
			out.WriteString(" --> ")
			out.WriteString(strconv.FormatUint(uint64(r.offset), 10))
			out.WriteByte('\n')
			out.WriteString(text)
			out.WriteByte('\n')
		}
	}
	return out.String(), nil
}

func (s *Substitution) Close() error {
	if s == nil || s.self != s || s.manager == nil {
		return ast.ErrASTHandle
	}
	if s.closed {
		return nil
	}
	if err := s.Reset(); err != nil {
		return err
	}
	s.closed = true
	return nil
}

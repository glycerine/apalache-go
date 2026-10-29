package rewriter

// Source: src/ast/rewriter/quant_hoist.cpp; src/ast/rewriter/quant_hoist.h.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type quantifierType uint8

const (
	quantForallPositive quantifierType = 0x10
	quantExistsPositive quantifierType = 0x20
	quantNonePositive   quantifierType = 0x40
	quantForallNegative quantifierType = 0x11
	quantExistsNegative quantifierType = 0x21
	quantNoneNegative   quantifierType = 0x41
)

type QuantifierHoister struct {
	self   *QuantifierHoister
	m      *ast.ASTManager
	bool   *boolRewriter
	closed bool
}

func NewQuantifierHoister(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef) (*QuantifierHoister, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	rw, err := newBoolRewriter(m, c, g, p)
	if err != nil {
		if rw != nil {
			return nil, errors.Join(err, rw.Close())
		}
		return nil, err
	}
	h := &QuantifierHoister{m: m, bool: rw}
	h.self = h
	return h, nil
}

func (h *QuantifierHoister) open() error {
	if h == nil || h.self != h || h.closed || h.m == nil || h.bool == nil {
		return ast.ErrASTHandle
	}
	return nil
}

type QuantifierHoistResult struct {
	self      *QuantifierHoistResult
	manager   *ast.ASTManager
	Variables []ast.ASTRef
	Formula   ast.ASTRef
	IsForall  bool
	closed    bool
}

func (r *QuantifierHoistResult) Close() error {
	if r == nil || r.self != r || r.manager == nil {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	if r.Formula != (ast.ASTRef{}) {
		if err := r.manager.DelRef(&r.Formula); err != nil {
			return err
		}
	}
	for len(r.Variables) != 0 {
		last := len(r.Variables) - 1
		if err := r.manager.DelRef(&r.Variables[last]); err != nil {
			return err
		}
		r.Variables = r.Variables[:last]
	}
	r.closed = true
	return nil
}

type quantifierHoistState struct {
	owner     *QuantifierHoister
	temporary []ast.ASTRef
	variables []ast.ASTRef
}

func (s *quantifierHoistState) scratch(h ast.ASTHandle) (ast.ASTRef, error) {
	r, err := s.owner.m.NewRef(h)
	if err == nil {
		s.temporary = append(s.temporary, r)
	}
	return r, err
}
func (s *quantifierHoistState) own(h ast.ASTHandle) (ast.ASTHandle, error) {
	_, err := s.scratch(h)
	return h, err
}
func (s *quantifierHoistState) cleanup() error {
	for len(s.temporary) != 0 {
		last := len(s.temporary) - 1
		if err := s.owner.m.DelRef(&s.temporary[last]); err != nil {
			return err
		}
		s.temporary = s.temporary[:last]
	}
	return nil
}
func (s *quantifierHoistState) cleanupVariables() error {
	for len(s.variables) != 0 {
		last := len(s.variables) - 1
		if err := s.owner.m.DelRef(&s.variables[last]); err != nil {
			return err
		}
		s.variables = s.variables[:last]
	}
	return nil
}

func negateQuantifierType(q *quantifierType) { *q ^= 1 }
func compatibleQuantifier(q quantifierType, forall bool) bool {
	switch q {
	case quantForallPositive:
		return forall
	case quantForallNegative:
		return !forall
	case quantExistsPositive:
		return !forall
	case quantExistsNegative:
		return forall
	case quantNonePositive, quantNoneNegative:
		return true
	}
	return false
}
func setQuantifierType(q *quantifierType, forall bool) error {
	switch *q {
	case quantForallPositive:
		if !forall {
			return ast.ErrASTDomain
		}
	case quantForallNegative:
		if forall {
			return ast.ErrASTDomain
		}
	case quantExistsPositive:
		if forall {
			return ast.ErrASTDomain
		}
	case quantExistsNegative:
		if !forall {
			return ast.ErrASTDomain
		}
	case quantNonePositive:
		if forall {
			*q = quantForallPositive
		} else {
			*q = quantExistsPositive
		}
	case quantNoneNegative:
		if forall {
			*q = quantExistsNegative
		} else {
			*q = quantForallNegative
		}
	default:
		return ast.ErrASTDomain
	}
	return nil
}

func (s *quantifierHoistState) isOp(h ast.ASTHandle, op int32) (bool, error) {
	k, err := s.owner.m.GetKind(h)
	if err != nil || k != ast.ASTApp {
		return false, err
	}
	f, err := s.owner.m.GetFamilyID(h)
	if err != nil || f != 0 {
		return false, err
	}
	d, err := s.owner.m.GetDeclKind(h)
	return d == op, err
}
func (s *quantifierHoistState) args(h ast.ASTHandle) ([]ast.ASTHandle, error) {
	n, err := s.owner.m.GetNumArgs(h)
	if err != nil {
		return nil, err
	}
	r := make([]ast.ASTHandle, n)
	for i := uint32(0); i < n; i++ {
		r[i], err = s.owner.m.GetArg(h, i)
		if err != nil {
			return nil, err
		}
	}
	return r, nil
}
func (s *quantifierHoistState) construct(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return s.own(h)
}
func (s *quantifierHoistState) mkAnd(args []ast.ASTHandle, rewrite bool) (ast.ASTHandle, error) {
	if !rewrite {
		return s.construct(s.owner.m.MkAndArgs(args))
	}
	out, err := s.scratch(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = s.owner.bool.mkAnd(args, out); err != nil {
		return ast.ASTHandle{}, err
	}
	return s.owner.m.RefGet(out)
}
func (s *quantifierHoistState) mkOr(args []ast.ASTHandle, rewrite bool) (ast.ASTHandle, error) {
	if !rewrite {
		return s.construct(s.owner.m.MkOrArgs(args))
	}
	out, err := s.scratch(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = s.owner.bool.mkOr(args, out); err != nil {
		return ast.ASTHandle{}, err
	}
	return s.owner.m.RefGet(out)
}

func (s *quantifierHoistState) extract(q ast.ASTHandle, useFresh bool) (ast.ASTHandle, error) {
	n, err := s.owner.m.GetNumDeclarations(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for i := uint32(0); i < n; i++ {
		sort, e := s.owner.m.GetDeclarationSort(q, i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		name, e := s.owner.m.GetDeclarationName(q, i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		var v ast.ASTHandle
		if useFresh {
			prefix, e := s.owner.m.Symbols.String(name)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			v, e = s.owner.m.MkFreshConst(prefix, sort)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		} else {
			v, e = s.owner.m.MkConst(name, sort)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		ref, e := s.owner.m.NewRef(v)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		s.variables = append(s.variables, ref)
	}
	values := make([]ast.ASTHandle, n)
	base := len(s.variables) - int(n)
	for i := range values {
		values[i], err = s.owner.m.RefGet(s.variables[base+i])
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	out, err := s.scratch(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = InstantiateQuantifier(s.owner.m, q, values, out); err != nil {
		return ast.ASTHandle{}, err
	}
	return s.owner.m.RefGet(out)
}

func (s *quantifierHoistState) pull(fml ast.ASTHandle, qt *quantifierType, useFresh, rewrite bool) (ast.ASTHandle, error) {
	has, err := s.owner.m.HasQuantifiers(fml)
	if err != nil || !has {
		return fml, err
	}
	kind, err := s.owner.m.GetKind(fml)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if kind == ast.ASTQuantifier {
		forall, e := s.owner.m.IsForall(fml)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if !compatibleQuantifier(*qt, forall) {
			return fml, nil
		}
		if e = setQuantifierType(qt, forall); e != nil {
			return ast.ASTHandle{}, e
		}
		body, e := s.extract(fml, useFresh)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return s.pull(body, qt, useFresh, rewrite)
	}
	if kind == ast.ASTVar {
		return fml, nil
	}
	if kind != ast.ASTApp {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	args, err := s.args(fml)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if yes, e := s.isOp(fml, ast.ASTOpAnd); e != nil {
		return ast.ASTHandle{}, e
	} else if yes {
		out := make([]ast.ASTHandle, len(args))
		for i := range args {
			out[i], err = s.pull(args[i], qt, useFresh, rewrite)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		return s.mkAnd(out, rewrite)
	}
	if yes, e := s.isOp(fml, ast.ASTOpOr); e != nil {
		return ast.ASTHandle{}, e
	} else if yes {
		out := make([]ast.ASTHandle, len(args))
		for i := range args {
			out[i], err = s.pull(args[i], qt, useFresh, rewrite)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		return s.mkOr(out, rewrite)
	}
	if yes, e := s.isOp(fml, ast.ASTOpNot); e != nil {
		return ast.ASTHandle{}, e
	} else if yes && len(args) == 1 {
		negateQuantifierType(qt)
		child, e := s.pull(args[0], qt, useFresh, rewrite)
		negateQuantifierType(qt)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return s.construct(s.owner.m.MkNot(child))
	}
	if yes, e := s.isOp(fml, ast.ASTOpImplies); e != nil {
		return ast.ASTHandle{}, e
	} else if yes && len(args) == 2 {
		negateQuantifierType(qt)
		left, e := s.pull(args[0], qt, useFresh, rewrite)
		negateQuantifierType(qt)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		right, e := s.pull(args[1], qt, useFresh, rewrite)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return s.construct(s.owner.m.MkImplies(left, right))
	}
	if yes, e := s.isOp(fml, ast.ASTOpIte); e != nil {
		return ast.ASTHandle{}, e
	} else if yes && len(args) == 3 {
		thenPart, e := s.pull(args[1], qt, useFresh, rewrite)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		elsePart, e := s.pull(args[2], qt, useFresh, rewrite)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		hc, e := s.owner.m.HasQuantifiers(args[0])
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if !hc {
			return s.construct(s.owner.m.MkIte(args[0], thenPart, elsePart))
		}
		positive, e := s.pull(args[0], qt, useFresh, rewrite)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		neg, e := s.construct(s.owner.m.MkNot(args[0]))
		if e != nil {
			return ast.ASTHandle{}, e
		}
		negative, e := s.pull(neg, qt, useFresh, rewrite)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		a, e := s.mkOr([]ast.ASTHandle{negative, thenPart}, false)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		b, e := s.mkOr([]ast.ASTHandle{positive, elsePart}, false)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return s.mkAnd([]ast.ASTHandle{a, b}, false)
	}
	iff, e := s.isOp(fml, ast.ASTOpIff)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	eq, e := s.isOp(fml, ast.ASTOpEq)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	boolEq := false
	if eq && len(args) == 2 {
		sort, x := s.owner.m.GetSort(args[0])
		if x != nil {
			return ast.ASTHandle{}, x
		}
		bs, x := s.owner.m.MkBoolSort()
		if x != nil {
			return ast.ASTHandle{}, x
		}
		boolEq = sort == bs
	}
	if (iff || boolEq) && len(args) == 2 {
		p, e := s.pull(args[0], qt, useFresh, rewrite)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		q, e := s.pull(args[1], qt, useFresh, rewrite)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		n1, e := s.construct(s.owner.m.MkNot(args[0]))
		if e != nil {
			return ast.ASTHandle{}, e
		}
		n2, e := s.construct(s.owner.m.MkNot(args[1]))
		if e != nil {
			return ast.ASTHandle{}, e
		}
		np, e := s.pull(n1, qt, useFresh, rewrite)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		nq, e := s.pull(n2, qt, useFresh, rewrite)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		a, e := s.mkOr([]ast.ASTHandle{np, q}, false)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		b, e := s.mkOr([]ast.ASTHandle{nq, p}, false)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return s.mkAnd([]ast.ASTHandle{a, b}, false)
	}
	return fml, nil
}

func (h *QuantifierHoister) run(fml ast.ASTHandle, qt quantifierType, useFresh, rewrite bool) (result *QuantifierHoistResult, err error) {
	if err = h.open(); err != nil {
		return nil, err
	}
	s := &quantifierHoistState{owner: h}
	defer func() {
		err = errors.Join(err, s.cleanup())
		if err != nil {
			err = errors.Join(err, s.cleanupVariables())
		}
	}()
	formula, err := s.pull(fml, &qt, useFresh, rewrite)
	if err != nil {
		return nil, err
	}
	ref, err := h.m.NewRef(formula)
	if err != nil {
		return nil, err
	}
	result = &QuantifierHoistResult{manager: h.m, Variables: s.variables, Formula: ref, IsForall: qt == quantForallPositive}
	result.self = result
	s.variables = nil
	return result, nil
}

func (h *QuantifierHoister) formatTrace(hnd ast.ASTHandle) (value string, result error) {
	format, err := h.m.SMT2PPExprTree(hnd, nil)
	if err != nil {
		return "", err
	}
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: h.m.BudgetField})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	if err = h.m.PrintFormat(out, format, nil, nil, nil); err != nil {
		return "", err
	}
	return out.String()
}
func (h *QuantifierHoister) trace(before, after ast.ASTHandle, line int) error {
	tags := diagnostic.BorrowProcessTraceTags()
	return tags.EmitConditional("qe_verbose", "qe_verbose", "src/ast/rewriter/quant_hoist.cpp", line, func() bool { return true }, func(w io.Writer) error {
		a, err := h.formatTrace(before)
		if err != nil {
			return err
		}
		b, err := h.formatTrace(after)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, a+"\n"+b+"\n")
		return err
	})
}
func (h *QuantifierHoister) Hoist(fml ast.ASTHandle, useFresh, rewrite bool) (*QuantifierHoistResult, error) {
	r, err := h.run(fml, quantNonePositive, useFresh, rewrite)
	if err != nil {
		return nil, err
	}
	after, e := h.m.RefGet(r.Formula)
	if e == nil {
		e = h.trace(fml, after, 44)
	}
	if e != nil {
		return nil, errors.Join(e, r.Close())
	}
	return r, nil
}
func (h *QuantifierHoister) PullExists(fml ast.ASTHandle, useFresh, rewrite bool) (*QuantifierHoistResult, error) {
	r, err := h.run(fml, quantExistsPositive, useFresh, rewrite)
	if err != nil {
		return nil, err
	}
	after, e := h.m.RefGet(r.Formula)
	if e == nil {
		e = h.trace(fml, after, 55)
	}
	if e != nil {
		return nil, errors.Join(e, r.Close())
	}
	return r, nil
}
func (h *QuantifierHoister) PullQuantifier(forall bool, fml ast.ASTHandle, useFresh, rewrite bool) (*QuantifierHoistResult, error) {
	q := quantExistsPositive
	if forall {
		q = quantForallPositive
	}
	r, err := h.run(fml, q, useFresh, rewrite)
	if err != nil {
		return nil, err
	}
	after, e := h.m.RefGet(r.Formula)
	if e == nil {
		e = h.trace(fml, after, 65)
	}
	if e != nil {
		return nil, errors.Join(e, r.Close())
	}
	return r, nil
}

type QuantifierIndexedResult struct {
	*QuantifierHoistResult
	Index uint32
	Sorts []ast.ASTHandle
	Names []usymbol.Symbol
}

func (h *QuantifierHoister) PullQuantifierIndexed(forall bool, fml ast.ASTHandle, useFresh, rewrite bool) (result *QuantifierIndexedResult, err error) {
	if err = h.open(); err != nil {
		return nil, err
	}
	counter, e := NewVarCounter(h.m)
	if e != nil {
		return nil, e
	}
	index, e := counter.NextVar(fml)
	if e != nil {
		return nil, e
	}
	sorts := []ast.ASTHandle{}
	names := []usymbol.Symbol{}
	current := fml
	for {
		kind, e := h.m.GetKind(current)
		if e != nil {
			return nil, e
		}
		if kind != ast.ASTQuantifier {
			break
		}
		fa, e := h.m.IsForall(current)
		if e != nil {
			return nil, e
		}
		if fa != forall {
			break
		}
		n, e := h.m.GetNumDeclarations(current)
		if e != nil {
			return nil, e
		}
		index += n
		for i := uint32(0); i < n; i++ {
			name, x := h.m.GetDeclarationName(current, i)
			if x != nil {
				return nil, x
			}
			sort, x := h.m.GetDeclarationSort(current, i)
			if x != nil {
				return nil, x
			}
			names = append(names, name)
			sorts = append(sorts, sort)
		}
		current, e = h.m.GetQuantifierBody(current)
		if e != nil {
			return nil, e
		}
	}
	has, e := h.m.HasQuantifiers(current)
	if e != nil {
		return nil, e
	}
	if !has {
		ref, e := h.m.NewRef(current)
		if e != nil {
			return nil, e
		}
		base := &QuantifierHoistResult{manager: h.m, Formula: ref, IsForall: forall}
		base.self = base
		return &QuantifierIndexedResult{QuantifierHoistResult: base, Index: index, Sorts: sorts, Names: names}, nil
	}
	base, e := h.PullQuantifier(forall, current, useFresh, rewrite)
	if e != nil {
		return nil, e
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, base.Close())
		}
	}()
	if len(base.Variables) == 0 {
		return &QuantifierIndexedResult{QuantifierHoistResult: base, Index: index, Sorts: sorts, Names: names}, nil
	}
	rep, e := NewExprSafeReplace(h.m)
	if e != nil {
		return nil, e
	}
	defer func() { err = errors.Join(err, rep.Close()) }()
	boundNames := []usymbol.Symbol{}
	boundSorts := []ast.ASTHandle{}
	for _, vr := range base.Variables {
		v, x := h.m.RefGet(vr)
		if x != nil {
			return nil, x
		}
		decl, x := h.m.GetDecl(v)
		if x != nil {
			return nil, x
		}
		name, x := h.m.GetASTName(decl)
		if x != nil {
			return nil, x
		}
		sort, x := h.m.GetSort(v)
		if x != nil {
			return nil, x
		}
		boundNames = append(boundNames, name)
		boundSorts = append(boundSorts, sort)
		variable, x := h.m.MkVar(index, sort)
		if x != nil {
			return nil, x
		}
		index++
		if x = rep.Insert(v, variable); x != nil {
			return nil, x
		}
	}
	if e = rep.ApplyRef(base.Formula); e != nil {
		return nil, e
	}
	for i, j := 0, len(boundNames)-1; i < j; i, j = i+1, j-1 {
		boundNames[i], boundNames[j] = boundNames[j], boundNames[i]
		boundSorts[i], boundSorts[j] = boundSorts[j], boundSorts[i]
	}
	names = append(boundNames, names...)
	sorts = append(boundSorts, sorts...)
	return &QuantifierIndexedResult{QuantifierHoistResult: base, Index: index, Sorts: sorts, Names: names}, nil
}

func (h *QuantifierHoister) Close() error {
	if h == nil || h.self != h {
		return ast.ErrASTHandle
	}
	if h.closed {
		return nil
	}
	if err := h.bool.Close(); err != nil {
		return err
	}
	h.closed = true
	return nil
}

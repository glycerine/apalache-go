package ast

// Source: src/ast/factor_equivs.cpp; src/ast/factor_equivs.h.

import "github.com/glycerine/g3/pkg/util/backtrack"

// ExprEquivClass is the concrete expression specialization of
// obj_equiv_class. It owns one AST reference for each inserted expression.
type ExprEquivClass struct {
	self    *ExprEquivClass
	manager *ASTManager
	uf      *backtrack.BasicUnionFind
	ids     map[ASTHandle]uint32
	objects []ASTHandle
	refs    []ASTRef
	nextRef int
	closed  bool
}

func NewExprEquivClass(m *ASTManager) (*ExprEquivClass, error) {
	if m == nil {
		return nil, ErrASTHandle
	}
	if e := m.lock(); e != nil {
		return nil, e
	}
	m.mu.Unlock()
	e := &ExprEquivClass{manager: m, uf: backtrack.NewBasicUnionFind(), ids: make(map[ASTHandle]uint32)}
	e.self = e
	return e, nil
}

func (e *ExprEquivClass) open() error {
	if e == nil || e.self != e || e.closed || e.manager == nil {
		return ErrASTHandle
	}
	_, err := e.manager.ProofsEnabled()
	return err
}

func (e *ExprEquivClass) add(h ASTHandle) (uint32, error) {
	if id, found := e.ids[h]; found {
		return id, nil
	}
	kind, err := e.manager.GetKind(h)
	if err != nil {
		return 0, err
	}
	if kind != ASTApp && kind != ASTVar && kind != ASTQuantifier {
		return 0, ErrASTDomain
	}
	ref, err := e.manager.NewRef(h)
	if err != nil {
		return 0, err
	}
	id := e.uf.MkVar()
	e.ids[h] = id
	e.objects = append(e.objects, h)
	e.refs = append(e.refs, ref)
	return id, nil
}

func (e *ExprEquivClass) Add(h ASTHandle) error {
	if err := e.open(); err != nil {
		return err
	}
	if _, found := e.ids[h]; found {
		return ErrASTDomain
	}
	_, err := e.add(h)
	return err
}

func (e *ExprEquivClass) Merge(a, b ASTHandle) error {
	if err := e.open(); err != nil {
		return err
	}
	x, err := e.add(a)
	if err != nil {
		return err
	}
	y, err := e.add(b)
	if err != nil {
		return err
	}
	e.uf.Merge(e.uf.Find(x), e.uf.Find(y))
	return nil
}

func (e *ExprEquivClass) AreEquivalent(a, b ASTHandle) (bool, error) {
	if err := e.open(); err != nil {
		return false, err
	}
	x, err := e.add(a)
	if err != nil {
		return false, err
	}
	y, err := e.add(b)
	if err != nil {
		return false, err
	}
	return e.uf.Find(x) == e.uf.Find(y), nil
}

// Classes returns borrowed handles in the source union-find class order.
func (e *ExprEquivClass) Classes() ([][]ASTHandle, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	result := make([][]ASTHandle, 0)
	for root := uint32(0); root < uint32(len(e.objects)); root++ {
		if !e.uf.IsRoot(root) {
			continue
		}
		class := make([]ASTHandle, 0)
		id := root
		for {
			class = append(class, e.objects[id])
			id = e.uf.Next(id)
			if id == root {
				break
			}
		}
		result = append(result, class)
	}
	return result, nil
}

func (e *ExprEquivClass) Reset() error {
	if err := e.open(); err != nil {
		return err
	}
	for e.nextRef < len(e.refs) {
		if err := e.manager.DelRef(&e.refs[e.nextRef]); err != nil {
			return err
		}
		e.nextRef++
	}
	e.uf.Reset()
	clear(e.ids)
	e.objects = e.objects[:0]
	e.refs = e.refs[:0]
	e.nextRef = 0
	return nil
}

func (e *ExprEquivClass) Close() error {
	if e == nil || e.self != e {
		return ErrASTHandle
	}
	if e.closed {
		return nil
	}
	if err := e.Reset(); err != nil {
		return err
	}
	e.closed = true
	return nil
}

func (m *ASTManager) factorEquality(h ASTHandle) (ASTHandle, ASTHandle, bool, error) {
	kind, err := m.GetKind(h)
	if err != nil || kind != ASTApp {
		return ASTHandle{}, ASTHandle{}, false, err
	}
	family, err := m.GetFamilyID(h)
	if err != nil || family != m.BasicFamilyID() {
		return ASTHandle{}, ASTHandle{}, false, err
	}
	op, err := m.GetDeclKind(h)
	if err != nil || op != ASTOpEq {
		return ASTHandle{}, ASTHandle{}, false, err
	}
	count, err := m.GetNumArgs(h)
	if err != nil || count != 2 {
		return ASTHandle{}, ASTHandle{}, false, err
	}
	a, err := m.GetArg(h, 0)
	if err != nil {
		return ASTHandle{}, ASTHandle{}, false, err
	}
	b, err := m.GetArg(h, 1)
	return a, b, true, err
}

func (m *ASTManager) factorArithmeticApp(h ASTHandle, family FamilyID, op int32, arity uint32) ([]ASTHandle, bool, error) {
	kind, err := m.GetKind(h)
	if err != nil || kind != ASTApp {
		return nil, false, err
	}
	fid, err := m.GetFamilyID(h)
	if err != nil || fid != family {
		return nil, false, err
	}
	k, err := m.GetDeclKind(h)
	if err != nil || k != op {
		return nil, false, err
	}
	n, err := m.GetNumArgs(h)
	if err != nil || n != arity {
		return nil, false, err
	}
	args := make([]ASTHandle, n)
	for i := range args {
		args[i], err = m.GetArg(h, uint32(i))
		if err != nil {
			return nil, false, err
		}
	}
	return args, true, nil
}

func (m *ASTManager) factorNumeralIs(h ASTHandle, want string) (bool, error) {
	value, _, matched, err := m.ArithmeticNumeralText(h)
	return matched && value == want, err
}

func (m *ASTManager) factorTimesMinusOne(h ASTHandle, family FamilyID) (ASTHandle, bool, error) {
	args, matched, err := m.factorArithmeticApp(h, family, 9, 2)
	if err != nil || !matched {
		return ASTHandle{}, false, err
	}
	minus, err := m.factorNumeralIs(args[0], "-1")
	if err != nil || !minus {
		return ASTHandle{}, false, err
	}
	return args[1], true, nil
}

// FactorEqualities removes equalities from expressions and merges their sides.
// It first applies the source flatten_and transformation.
func (m *ASTManager) FactorEqualities(expressions []ASTHandle, equiv *ExprEquivClass) ([]ASTHandle, error) {
	if equiv == nil || equiv.manager != m {
		return expressions, ErrASTHandle
	}
	v, err := m.FlattenAnd(expressions)
	if err != nil {
		return v, err
	}
	arith, err := m.ArithmeticFamilyID()
	if err != nil {
		return v, err
	}
	j := 0
	for i := range v {
		a, b, equality, err := m.factorEquality(v[i])
		if err != nil {
			return v[:j], err
		}
		if !equality {
			v[j] = v[i]
			j++
			continue
		}
		if zero, err := m.factorNumeralIs(a, "0"); err != nil {
			return v[:j], err
		} else if zero {
			a, b = b, a
		}
		if zero, err := m.factorNumeralIs(b, "0"); err != nil {
			return v[:j], err
		} else if zero {
			args, add, err := m.factorArithmeticApp(a, arith, 6, 2)
			if err != nil {
				return v[:j], err
			}
			if add {
				if x, yes, err := m.factorTimesMinusOne(args[1], arith); err != nil {
					return v[:j], err
				} else if yes {
					a, b = args[0], x
				} else if x, yes, err := m.factorTimesMinusOne(args[0], arith); err != nil {
					return v[:j], err
				} else if yes {
					a, b = args[1], x
				}
			}
		}
		if err := equiv.Merge(a, b); err != nil {
			return v[:j], err
		}
	}
	return v[:j], nil
}

func releaseFactorRefs(m *ASTManager, refs []ASTRef) {
	for i := range refs {
		_ = m.DelRef(&refs[i])
	}
}

// EquivToExpr emits one equality from a non-value representative to every
// other member of each equivalence class.
func (m *ASTManager) EquivToExpr(equiv *ExprEquivClass) ([]ASTRef, error) {
	classes, err := equiv.Classes()
	if err != nil {
		return nil, err
	}
	var out []ASTRef
	for _, class := range classes {
		var rep ASTHandle
		for _, elem := range class {
			value, err := m.IsValue(elem)
			if err != nil {
				releaseFactorRefs(m, out)
				return nil, err
			}
			if !value {
				rep = elem
				break
			}
		}
		if rep.IsNull() {
			releaseFactorRefs(m, out)
			return nil, ErrASTDomain
		}
		for _, elem := range class {
			if elem == rep {
				continue
			}
			eq, err := m.MkEq(rep, elem)
			if err != nil {
				releaseFactorRefs(m, out)
				return nil, err
			}
			ref, err := m.NewRef(eq)
			if err != nil {
				releaseFactorRefs(m, out)
				return nil, err
			}
			out = append(out, ref)
		}
	}
	return out, nil
}

// EquivToExprFull emits every unordered equality pair in every class.
func (m *ASTManager) EquivToExprFull(equiv *ExprEquivClass) ([]ASTRef, bool, error) {
	classes, err := equiv.Classes()
	if err != nil {
		return nil, false, err
	}
	var out []ASTRef
	for _, class := range classes {
		for i := 0; i < len(class); i++ {
			for j := i + 1; j < len(class); j++ {
				eq, err := m.MkEq(class[i], class[j])
				if err != nil {
					releaseFactorRefs(m, out)
					return nil, false, err
				}
				ref, err := m.NewRef(eq)
				if err != nil {
					releaseFactorRefs(m, out)
					return nil, false, err
				}
				out = append(out, ref)
			}
		}
	}
	return out, len(out) != 0, nil
}

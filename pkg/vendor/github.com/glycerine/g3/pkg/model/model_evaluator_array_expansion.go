package model

// Source: src/model/model_evaluator.cpp:207-221,422-499,565-569.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// Native rows own one AST reference per element. Embedded lexical fields retain
// unfinished numerical/AST cleanup; collection backing belongs to Go.
type arrayExpansion struct {
	active             bool
	stores             [][]ast.ASTHandle
	drain              int
	row, copyRow, args []ast.ASTHandle
	elseCase           ast.ASTRef
	parameter          ast.ASTParameter
	parameterLive      bool
}

func appendArrayRoot(m *ast.ASTManager, roots *[]ast.ASTHandle, h ast.ASTHandle) error {
	if e := m.IncRef(h); e != nil {
		return e
	}
	*roots = append(*roots, h)
	return nil
}

// A consumed DEC can report a later destructor error. Retire that root before
// returning, retaining only unconsumed references for the cleanup retry.
func releaseArrayRoots(m *ast.ASTManager, roots *[]ast.ASTHandle) error {
	values := *roots
	for i, h := range values {
		consumed, e := m.DecRefConsumed(h)
		next := i
		if consumed {
			values[i] = ast.ASTHandle{}
			next++
		}
		if e != nil {
			*roots = values[next:]
			return e
		}
		if !consumed {
			*roots = values[next:]
			return ast.ErrASTPending
		}
	}
	*roots = values[:0]
	return nil
}
func appendArrayArguments(m *ast.ASTManager, roots *[]ast.ASTHandle, args ast.AppArguments) error {
	for i := uint32(0); i < args.Count(); i++ {
		h, e := args.At(i)
		if e != nil {
			return e
		}
		if e = appendArrayRoot(m, roots, h); e != nil {
			return e
		}
	}
	return nil
}
func (f *arrayExpansion) publishRow() {
	// Transfer this row's references to the stores without a redundant copy.
	f.stores = append(f.stores, f.row)
	f.row = nil
}
func (f *arrayExpansion) appendRowCopy(m *ast.ASTManager, input []ast.ASTHandle) error {
	if e := appendArrayArguments(m, &f.copyRow, ast.SliceArguments(input)); e != nil {
		return e
	}
	f.stores = append(f.stores, f.copyRow)
	f.copyRow = nil
	return nil
}
func (f *arrayExpansion) removeRow(m *ast.ASTManager, index int) error {
	if e := releaseArrayRoots(m, &f.stores[index]); e != nil {
		return e
	}
	copy(f.stores[index:], f.stores[index+1:])
	last := len(f.stores) - 1
	f.stores[last] = nil
	f.stores = f.stores[:last]
	return nil
}
func (f *arrayExpansion) closeLexical(m *ast.ASTManager) error {
	if f.parameterLive {
		if e := m.DelParameter(&f.parameter); e != nil {
			return e
		}
		f.parameterLive = false
	}
	for _, roots := range []*[]ast.ASTHandle{&f.args, &f.copyRow, &f.row} {
		if e := releaseArrayRoots(m, roots); e != nil {
			return e
		}
		*roots = nil
	}
	return nil
}
func (f *arrayExpansion) closeElse(m *ast.ASTManager) error {
	if f.elseCase != (ast.ASTRef{}) {
		return m.DelRef(&f.elseCase)
	}
	return nil
}
func (f *arrayExpansion) closeStores(m *ast.ASTManager) error {
	for f.drain < len(f.stores) {
		if e := releaseArrayRoots(m, &f.stores[f.drain]); e != nil {
			return e
		}
		f.drain++
	}
	*f = arrayExpansion{}
	return nil
}
func (f *arrayExpansion) close(m *ast.ASTManager) error {
	if !f.active {
		return nil
	}
	if e := f.closeLexical(m); e != nil {
		return e
	}
	if e := f.closeElse(m); e != nil {
		return e
	}
	return f.closeStores(m)
}
func (c *evaluatorConfiguration) arrayOp(h ast.ASTHandle, op int32) (bool, error) {
	m := c.owner.model.manager
	k, e := m.GetKind(h)
	if e != nil || k != ast.ASTApp {
		return false, e
	}
	fid, e := m.GetFamilyID(h)
	if e != nil || fid != c.arrayFamily {
		return false, e
	}
	kind, e := m.GetDeclKind(h)
	return kind == op, e
}
func (c *evaluatorConfiguration) arrayArgsAreValues(v []ast.ASTHandle, unique *bool) (bool, error) {
	m := c.owner.model.manager
	values := true
	for j := 0; values && j+1 < len(v); j++ {
		var e error
		values, e = m.IsValue(v[j])
		if e != nil {
			return false, e
		}
		one, e := m.IsUniqueValue(v[j])
		if e != nil {
			return false, e
		}
		*unique = *unique && one
	}
	return values, nil
}
func (c *evaluatorConfiguration) extractArray(a ast.ASTHandle, f *arrayExpansion) (bool, bool, error) {
	m := c.owner.model.manager
	values, unique := true, true
	if e := c.traceArray(435, "", a); e != nil {
		return false, unique, e
	}
	for {
		stored, e := c.arrayOp(a, ast.ASTArrayOpStore)
		if e != nil {
			return false, unique, e
		}
		if !stored {
			break
		}
		args, e := m.AppArgumentView(a)
		if e != nil {
			return false, unique, e
		}
		tail, e := args.Range(1, args.Count()-1)
		if e != nil {
			return false, unique, e
		}
		if e = appendArrayArguments(m, &f.row, tail); e != nil {
			return false, unique, e
		}
		current, e := c.arrayArgsAreValues(f.row, &unique)
		if e != nil {
			return false, unique, e
		}
		values = values && current
		f.publishRow()
		a, e = args.At(0)
		if e != nil {
			return false, unique, e
		}
	}
	constant, e := c.arrayOp(a, ast.ASTArrayOpConst)
	if e != nil {
		return false, unique, e
	}
	if constant {
		value, e := m.GetArg(a, 0)
		if e != nil {
			return false, unique, e
		}
		return true, unique, m.SetRef(f.elseCase, value)
	}
	backed, e := c.arrayOp(a, ast.ASTArrayOpAsArray)
	if e != nil {
		return false, unique, e
	}
	if !backed {
		return false, unique, c.traceArray(451, "no translation: ", a)
	}
	d, e := m.AsArrayFunction(a)
	if e != nil {
		return false, unique, e
	}
	g, e := c.owner.model.FunctionInterpretation(d)
	if e != nil || g == nil {
		return false, unique, e
	}
	if e = g.open(); e != nil {
		return false, unique, e
	}
	base := len(f.stores)
	for _, entry := range g.entries {
		if e = appendArrayArguments(m, &f.row, ast.SliceArguments(entry.args)); e != nil {
			return false, unique, e
		}
		value, e := entry.Result()
		if e != nil {
			return false, unique, e
		}
		if e = appendArrayRoot(m, &f.row, value); e != nil {
			return false, unique, e
		}
		for _, h := range f.row {
			ground, e := m.IsGround(h)
			if e != nil {
				return false, unique, e
			}
			if !ground {
				return false, unique, c.traceArray(468, "could not extract array interpretation: ", a, h)
			}
		}
		f.publishRow()
	}
	fallback, e := g.Else()
	if e != nil {
		return false, unique, e
	}
	if e = m.SetRef(f.elseCase, fallback); e != nil {
		return false, unique, e
	}
	if fallback.IsNull() {
		return false, unique, c.traceArray(476, "no else case ", a)
	}
	ground, e := m.IsGround(fallback)
	if e != nil {
		return false, unique, e
	}
	if !ground {
		return false, unique, c.traceArray(482, "non-ground else case ", a, fallback)
	}
	for i := len(f.stores); values && i > base; {
		i--
		row := f.stores[i]
		last := row[len(row)-1]
		same, e := m.AreEqual(fallback, last)
		if e != nil {
			return false, unique, e
		}
		if same {
			if e = f.removeRow(m, i); e != nil {
				return false, unique, e
			}
			continue
		}
		values, e = c.arrayArgsAreValues(f.stores[i], &unique)
		if e != nil {
			return false, unique, e
		}
	}
	return true, unique, c.traceArray(497, "else case: ", fallback)
}
func (c *evaluatorConfiguration) expandValue(out ast.ASTRef) (err error) {
	m := c.owner.model.manager
	value, e := m.RefGet(out)
	if e != nil {
		return e
	}
	sort, e := m.GetSort(value)
	if e != nil {
		return e
	}
	fid, e := m.GetFamilyID(sort)
	if e != nil || fid != c.arrayFamily {
		return e
	}
	f := &c.expansion
	if f.active {
		return uparams.ErrParamsPrecondition
	}
	f.active = true
	defer func() { err = errors.Join(err, f.close(m)) }()
	f.elseCase, e = m.NewRef(ast.ASTHandle{})
	if e != nil {
		return e
	}
	ok, _, e := c.extractArray(value, f)
	if e != nil || !ok {
		return e
	}
	fallback, e := m.RefGet(f.elseCase)
	if e != nil {
		return e
	}
	f.parameter, e = m.NewParameterAST(sort)
	if e != nil {
		return e
	}
	f.parameterLive = true
	constant, e := m.MkFamilyApp(c.arrayFamily, ast.ASTArrayOpConst, []ast.ASTParameter{f.parameter}, []ast.ASTHandle{fallback})
	if e != nil {
		return e
	}
	if e = m.DelParameter(&f.parameter); e != nil {
		return e
	}
	f.parameterLive = false
	if e = m.SetRef(out, constant); e != nil {
		return e
	}
	for i := len(f.stores); i > 0; {
		i--
		current, e := m.RefGet(out)
		if e != nil {
			return e
		}
		if e = appendArrayRoot(m, &f.args, current); e != nil {
			return e
		}
		if e = appendArrayArguments(m, &f.args, ast.SliceArguments(f.stores[i])); e != nil {
			return e
		}
		args := ast.SliceArguments(f.args)
		stored, e := m.MkArrayAppArguments(c.arrayFamily, ast.ASTArrayOpStore, args)
		if e != nil {
			return e
		}
		if e = m.SetRef(out, stored); e != nil {
			return e
		}
		if e = releaseArrayRoots(m, &f.args); e != nil {
			return e
		}
	}
	return nil
}

// Preserve the base array while releasing temporary select indices. On an
// interrupted DEC, keep all unconsumed roots reachable for lexical cleanup.
func releaseArrayTail(m *ast.ASTManager, roots *[]ast.ASTHandle) error {
	tail := (*roots)[1:]
	e := releaseArrayRoots(m, &tail)
	*roots = append((*roots)[:1], tail...)
	return e
}

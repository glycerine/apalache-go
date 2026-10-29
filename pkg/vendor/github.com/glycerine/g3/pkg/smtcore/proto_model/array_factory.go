package protomodel

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type ArrayFactory struct{ structuredFactory }

func NewArrayFactory(md *ProtoModel) (*ArrayFactory, error) {
	if err := md.open(); err != nil {
		return nil, err
	}
	fid, err := md.manager.ArrayFamilyID()
	if err != nil {
		return nil, err
	}
	return &ArrayFactory{structuredFactory: newStructuredFactory(md.manager, fid, md)}, nil
}
func (f *ArrayFactory) arrayShape(s ast.ASTHandle) ([]ast.ASTHandle, ast.ASTHandle, error) {
	n, err := f.manager.ArrayArity(s)
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	domain := make([]ast.ASTHandle, n)
	for i := range domain {
		domain[i], err = f.manager.ArrayDomain(s, uint32(i))
		if err != nil {
			return nil, ast.ASTHandle{}, err
		}
	}
	rangeSort, err := f.manager.ArrayRange(s)
	return domain, rangeSort, err
}
func (f *ArrayFactory) makeInterpretation(s ast.ASTHandle) (value ast.ASTHandle, fi *model.FunctionInterpretation, err error) {
	domain, rangeSort, err := f.arrayShape(s)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	d, err := f.manager.MkFreshFuncDecl(usymbol.Symbol{}, usymbol.Symbol{}, domain, rangeSort)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	dref, err := f.manager.NewRef(d)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	defer func() { err = errors.Join(err, f.manager.DelRef(&dref)) }()
	fi, err = model.NewFunctionInterpretation(f.manager, uint32(len(domain)))
	if err != nil {
		return ast.ASTHandle{}, fi, err
	}
	if err = f.model.core.Core().RegisterFunction(d, fi); err != nil {
		_ = fi.Close()
		return ast.ASTHandle{}, nil, err
	}
	value, err = f.manager.MkAsArray(d)
	if err != nil {
		return ast.ASTHandle{}, fi, err
	}
	err = f.RegisterValue(value)
	return value, fi, err
}
func (f *ArrayFactory) someArguments(s ast.ASTHandle) ([]ast.ASTHandle, error) {
	domain, _, err := f.arrayShape(s)
	if err != nil {
		return nil, err
	}
	args := make([]ast.ASTHandle, len(domain))
	for i, d := range domain {
		args[i], err = f.model.SomeValue(d)
		if err != nil {
			return nil, err
		}
	}
	return args, nil
}
func (f *ArrayFactory) SomeValue(s ast.ASTHandle) (ast.ASTHandle, error) {
	if err := f.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if err := f.trace("array_factory", "get_some_value", 64, nil, func(out io.Writer) error {
		if err := arrayFactoryDisplayAST(f.manager, out, s); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	set := f.sets[s]
	if set != nil && len(set.values) != 0 {
		return set.values[0], nil
	}
	v, fi, err := f.makeInterpretation(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	args, err := f.someArguments(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	_, rangeSort, err := f.arrayShape(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := f.model.SomeValue(rangeSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return v, fi.InsertEntry(args, result)
}
func (f *ArrayFactory) makeTwo(s ast.ASTHandle) (ok bool, result error) {
	if err := valueFactoryAssert("src/smt/proto_model/array_factory.cpp", 79, "!m_sort2value_set.find(s, set) || set->size() == 0", func() bool {
		set := f.sets[s]
		return set == nil || len(set.values) == 0
	}); err != nil {
		return false, err
	}
	first, err := f.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	var second ast.ASTRef
	defer func() {
		if second != (ast.ASTRef{}) {
			if e := f.manager.DelRef(&second); e != nil {
				result = errors.Join(result, e)
				ok = false
				return
			}
		}
		result = errors.Join(result, f.manager.DelRef(&first))
		if result != nil {
			ok = false
		}
	}()
	second, err = f.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	_, rangeSort, err := f.arrayShape(s)
	if err != nil {
		return false, err
	}
	pairOK, err := f.model.SomeValues(rangeSort, first, second)
	if err != nil || !pairOK {
		return false, err
	}
	args, err := f.someArguments(s)
	if err != nil {
		return false, err
	}
	_, fi1, err := f.makeInterpretation(s)
	if err != nil {
		return false, err
	}
	_, fi2, err := f.makeInterpretation(s)
	if err != nil {
		return false, err
	}
	r1, err := f.manager.RefGet(first)
	if err != nil {
		return false, err
	}
	if err = fi1.InsertEntry(args, r1); err != nil {
		return false, err
	}
	r2, err := f.manager.RefGet(second)
	if err != nil {
		return false, err
	}
	if err = fi2.InsertEntry(args, r2); err != nil {
		return false, err
	}
	if err = valueFactoryAssert("src/smt/proto_model/array_factory.cpp", 96, "m_sort2value_set.find(s, set) && set->size() == 2", func() bool {
		set := f.sets[s]
		return set != nil && len(set.values) == 2
	}); err != nil {
		return false, err
	}
	return true, nil
}
func (f *ArrayFactory) SomeValues(s ast.ASTHandle, first, second ast.ASTRef) (bool, error) {
	if err := f.open(); err != nil {
		return false, err
	}
	set := f.sets[s]
	if set == nil || len(set.values) == 0 {
		ok, err := f.makeTwo(s)
		if err != nil || !ok {
			return false, err
		}
	}
	set = f.sets[s]
	if err := valueFactoryAssert("src/smt/proto_model/array_factory.cpp", 108, "set != 0", func() bool { return set != nil }); err != nil {
		return false, err
	}
	if err := valueFactoryAssert("src/smt/proto_model/array_factory.cpp", 109, "set->size() > 0", func() bool { return set != nil && len(set.values) > 0 }); err != nil {
		return false, err
	}
	if set == nil || len(set.values) == 0 {
		return false, ast.ErrASTDomain
	}
	if err := f.manager.SetRef(first, set.values[0]); err != nil {
		return false, err
	}
	if len(set.values) == 1 {
		value, err := f.FreshValue(s)
		if err != nil {
			return false, err
		}
		if err = f.manager.SetRef(second, value); err != nil {
			return false, err
		}
		return !value.IsNull(), nil
	}
	if err := valueFactoryAssert("src/smt/proto_model/array_factory.cpp", 117, "set->size() >= 2", func() bool { return len(set.values) >= 2 }); err != nil {
		return false, err
	}
	if err := f.manager.SetRef(second, set.values[1]); err != nil {
		return false, err
	}
	return true, nil
}
func (f *ArrayFactory) FreshValue(s ast.ASTHandle) (out ast.ASTHandle, result error) {
	set, err := f.valueSet(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if len(set.values) == 0 {
		return f.SomeValue(s)
	}
	domain, rangeSort, err := f.arrayShape(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if rangeValue, e := f.model.FreshValue(rangeSort); e != nil {
		return ast.ASTHandle{}, e
	} else if !rangeValue.IsNull() {
		v, fi, e := f.makeInterpretation(s)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		args, e := f.someArguments(s)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return v, fi.InsertEntry(args, rangeValue)
	}
	if err = f.trace("array_factory_bug", "get_fresh_value", 152, nil, func(out io.Writer) error {
		if _, err := io.WriteString(out, "array fresh value: using fresh index, range: "); err != nil {
			return err
		}
		if err := arrayFactoryDisplayAST(f.manager, out, rangeSort); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	first, err := f.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var second ast.ASTRef
	defer func() {
		if second != (ast.ASTRef{}) {
			if e := f.manager.DelRef(&second); e != nil {
				result = errors.Join(result, e)
				out = ast.ASTHandle{}
				return
			}
		}
		result = errors.Join(result, f.manager.DelRef(&first))
		if result != nil {
			out = ast.ASTHandle{}
		}
	}()
	second, err = f.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	pairOK, err := f.model.SomeValues(rangeSort, first, second)
	if err != nil || !pairOK {
		if err == nil {
			err = f.trace("array_factory_bug", "get_fresh_value", 203, nil, func(out io.Writer) error {
				_, err := io.WriteString(out, "failed to build fresh array value\n")
				return err
			})
		}
		return ast.ASTHandle{}, err
	}
	r1, err := f.manager.RefGet(first)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	r2, err := f.manager.RefGet(second)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = f.trace("array_factory_bug", "get_fresh_value", 163, nil, func(out io.Writer) error {
		if _, err := io.WriteString(out, "v1: "); err != nil {
			return err
		}
		if err := arrayFactoryDisplayAST(f.manager, out, r1); err != nil {
			return err
		}
		if _, err := io.WriteString(out, " v2: "); err != nil {
			return err
		}
		if err := arrayFactoryDisplayAST(f.manager, out, r2); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	a1, a2 := make([]ast.ASTHandle, len(domain)), make([]ast.ASTHandle, len(domain))
	found := false
	for i, d := range domain {
		if !found {
			a1[i], err = f.model.FreshValue(d)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			a2[i], err = f.model.FreshValue(d)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if !a1[i].IsNull() && !a2[i].IsNull() {
				found = true
				continue
			}
		}
		a1[i], err = f.model.SomeValue(d)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		a2[i] = a1[i]
	}
	if !found {
		if err = f.trace("array_factory_bug", "get_fresh_value", 203, nil, func(out io.Writer) error {
			_, err := io.WriteString(out, "failed to build fresh array value\n")
			return err
		}); err != nil {
			return ast.ASTHandle{}, err
		}
		return ast.ASTHandle{}, nil
	}
	v, fi, err := f.makeInterpretation(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fi.InsertEntry(a1, r1); err != nil {
		return ast.ASTHandle{}, err
	}
	return v, fi.InsertEntry(a2, r2)
}

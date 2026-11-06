package protomodel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type structuredFactory struct {
	manager *ast.ASTManager
	model   *ProtoModel
	family  ast.FamilyID
	sets    map[ast.ASTHandle]*factoryValueSet
	sorts   []ast.ASTRef
	values  []ast.ASTRef
}

func newStructuredFactory(m *ast.ASTManager, family ast.FamilyID, md *ProtoModel) structuredFactory {
	return structuredFactory{manager: m, family: family, model: md, sets: make(map[ast.ASTHandle]*factoryValueSet)}
}
func (f *structuredFactory) FamilyID() ast.FamilyID { return f.family }
func (f *structuredFactory) open() error {
	if f == nil || f.manager == nil || f.sets == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (f *structuredFactory) valueSet(s ast.ASTHandle) (*factoryValueSet, error) {
	if err := f.open(); err != nil {
		return nil, err
	}
	set := f.sets[s]
	if set == nil {
		// Source publishes the allocated set in m_sort2value_set before
		// m_sorts.push_back acquires the sort.  Preserve that failure prefix:
		// a refused sort reference leaves the empty set discoverable, and a
		// later lookup does not retry the source push_back.
		set = &factoryValueSet{seen: make(map[ast.ASTHandle]struct{})}
		f.sets[s] = set
		ref, err := f.manager.NewRef(s)
		if err != nil {
			return nil, err
		}
		f.sorts = append(f.sorts, ref)
	}
	if err := valueFactoryAssert("src/smt/proto_model/struct_factory.cpp", 30, "set != 0", func() bool { return set != nil }); err != nil {
		return nil, err
	}
	return set, nil
}
func (f *structuredFactory) RegisterValue(v ast.ASTHandle) error {
	if err := f.open(); err != nil {
		return err
	}
	s, err := f.manager.GetSort(v)
	if err != nil {
		return err
	}
	set, err := f.valueSet(s)
	if err != nil {
		return err
	}
	if _, found := set.seen[v]; found {
		return nil
	}
	ref, err := f.manager.NewRef(v)
	if err != nil {
		return err
	}
	set.seen[v] = struct{}{}
	set.values = append(set.values, v)
	f.values = append(f.values, ref)
	return nil
}
func (f *structuredFactory) SomeValues(s ast.ASTHandle, factory ValueFactory, first, second ast.ASTRef) (bool, error) {
	set, err := f.valueSet(s)
	if err != nil {
		return false, err
	}
	switch len(set.values) {
	case 0:
		value, err := factory.FreshValue(s)
		if err != nil {
			return false, err
		}
		if err = f.manager.SetRef(first, value); err != nil {
			return false, err
		}
		value, err = factory.FreshValue(s)
		if err != nil {
			return false, err
		}
		if err = f.manager.SetRef(second, value); err != nil {
			return false, err
		}
		a, err := f.manager.RefGet(first)
		if err != nil {
			return false, err
		}
		b, err := f.manager.RefGet(second)
		if err != nil {
			return false, err
		}
		return !a.IsNull() && !b.IsNull(), nil
	case 1:
		value, err := factory.SomeValue(s)
		if err != nil {
			return false, err
		}
		if err = f.manager.SetRef(first, value); err != nil {
			return false, err
		}
		value, err = factory.FreshValue(s)
		if err != nil {
			return false, err
		}
		if err = f.manager.SetRef(second, value); err != nil {
			return false, err
		}
		return !value.IsNull(), nil
	default:
		if err = f.manager.SetRef(first, set.values[0]); err != nil {
			return false, err
		}
		if err = f.manager.SetRef(second, set.values[1]); err != nil {
			return false, err
		}
		return true, nil
	}
}

// Source struct_factory destroys its borrowed-value sets in its body, then
// m_sorts before m_values. Each native ref_vector releases in insertion order.
// Go owning retries skip tokens that DelRef consumed before returning an error;
// the borrowed AST manager retains any interrupted numerical deletion graph.
func (f *structuredFactory) Close() (err error) {
	if f == nil || f.manager == nil {
		return nil
	}
	f.sets = nil
	for i := range f.sorts {
		if f.sorts[i] == (ast.ASTRef{}) {
			continue
		}
		err = errors.Join(err, f.manager.DelRef(&f.sorts[i]))
	}
	for i := range f.values {
		if f.values[i] == (ast.ASTRef{}) {
			continue
		}
		err = errors.Join(err, f.manager.DelRef(&f.values[i]))
	}
	if err == nil {
		f.values, f.sorts = nil, nil
		f.manager, f.model = nil, nil
	}
	return err
}

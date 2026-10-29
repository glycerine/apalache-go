package protomodel

// Source: src/smt/proto_model/value_factory.cpp:53-117 and
// concrete simple_factory<unsigned> in value_factory.h:79-221.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type userSortValueSet struct {
	values []ast.ASTHandle
	seen   map[ast.ASTHandle]struct{}
	next   uint32
}

// userSortFactory implements the retained unsigned simple_factory. A slice plus
// membership map replaces the source unordered set; selection among registered
// values uses insertion order rather than exposing C++ hash-table layout.
type userSortFactory struct {
	manager *ast.ASTManager
	family  ast.FamilyID
	sets    map[ast.ASTHandle]*userSortValueSet
	finite  map[ast.ASTHandle]bool
	sorts   []ast.ASTRef
	values  []ast.ASTRef
}

func newUserSortFactory(manager *ast.ASTManager) *userSortFactory {
	return &userSortFactory{manager: manager, family: 4, sets: make(map[ast.ASTHandle]*userSortValueSet), finite: make(map[ast.ASTHandle]bool)}
}
func NewUserSortFactory(manager *ast.ASTManager) ValueFactory { return newUserSortFactory(manager) }

func (f *userSortFactory) FamilyID() ast.FamilyID { return f.family }

func (f *userSortFactory) open() error {
	if f == nil || f.manager == nil || f.sets == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (f *userSortFactory) valueSet(sort ast.ASTHandle) (*userSortValueSet, error) {
	if err := f.open(); err != nil {
		return nil, err
	}

	set := f.sets[sort]
	if set == nil {
		// simple_factory::get_value_set publishes the allocated set/map before
		// m_sorts.push_back acquires the sort. A failed acquisition therefore
		// leaves a warm empty set and is not retried by a later lookup.
		set = &userSortValueSet{seen: make(map[ast.ASTHandle]struct{})}
		f.sets[sort] = set
		ref, err := f.manager.NewRef(sort)
		if err != nil {
			return nil, err
		}
		f.sorts = append(f.sorts, ref)
	}
	if err := simpleFactoryAssert(105, "set != 0", func() bool { return set != nil }); err != nil {
		return nil, err
	}
	if diagnostic.DebugBuild {
		var set2 *userSortValueSet
		if err := simpleFactoryAssert(108, "m_sort2value_set.find(s, set2)", func() bool { set2 = f.sets[sort]; return set2 != nil }); err != nil {
			return nil, err
		}
		if err := simpleFactoryAssert(109, "set == set2", func() bool { return set == set2 }); err != nil {
			return nil, err
		}
	}
	return set, nil
}

func (f *userSortFactory) insert(value uint32, sort ast.ASTHandle) (ast.ASTHandle, bool, error) {
	set, err := f.valueSet(sort)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	numeral, err := f.manager.MkModelValue(value, sort)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	fresh := false
	if _, found := set.seen[numeral]; !found {
		ref, err := f.manager.NewRef(numeral)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		f.values = append(f.values, ref)
		set.seen[numeral] = struct{}{}
		set.values = append(set.values, numeral)
		fresh = true
	}
	if err := simpleFactoryAssert(125, "set->m_values.contains(new_val)", func() bool { _, ok := set.seen[numeral]; return ok }); err != nil {
		return ast.ASTHandle{}, fresh, err
	}
	return numeral, fresh, nil
}

func (f *userSortFactory) someValue(sort ast.ASTHandle) (ast.ASTHandle, error) {
	if err := f.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if set := f.sets[sort]; set != nil && len(set.values) != 0 {
		return set.values[0], nil
	}
	value, _, err := f.insert(0, sort)
	return value, err
}

func (f *userSortFactory) someValues(sort ast.ASTHandle, first, second ast.ASTRef) (bool, error) {
	if err := f.open(); err != nil {
		return false, err
	}
	set, exists := f.sets[sort]
	if exists && len(set.values) >= 2 {
		if err := f.manager.SetRef(first, set.values[0]); err != nil {
			return false, err
		}
		if err := f.manager.SetRef(second, set.values[1]); err != nil {
			return false, err
		}
	} else if exists && len(set.values) == 1 {
		if err := f.manager.SetRef(first, set.values[0]); err != nil {
			return false, err
		}
		value, _, err := f.insert(0, sort)
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
		if a == b {
			value, _, err = f.insert(1, sort)
			if err != nil {
				return false, err
			}
			if err = f.manager.SetRef(second, value); err != nil {
				return false, err
			}
		}
	} else {
		value, _, err := f.insert(0, sort)
		if err != nil {
			return false, err
		}
		if err = f.manager.SetRef(first, value); err != nil {
			return false, err
		}
		value, _, err = f.insert(1, sort)
		if err != nil {
			return false, err
		}
		if err = f.manager.SetRef(second, value); err != nil {
			return false, err
		}
	}
	if exists {
		a, err := f.manager.RefGet(first)
		if err != nil {
			return false, err
		}
		b, err := f.manager.RefGet(second)
		if err != nil {
			return false, err
		}
		if err = simpleFactoryAssert(171, "v1 != v2", func() bool { return a != b }); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (f *userSortFactory) freshValue(sort ast.ASTHandle) (ast.ASTHandle, error) {
	set, err := f.valueSet(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	size, err := f.manager.GetSortSize(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var maxSize uint32
	hasMax := false
	if size.IsFinite() {
		n, e := size.Size()
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if n < uint64(^uint32(0)) {
			maxSize = uint32(n)
			hasMax = true
		}
	}
	start := set.next
	for {
		value, fresh, err := f.insert(set.next, sort)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		set.next++
		if hasMax && set.next > maxSize+start {
			return ast.ASTHandle{}, nil
		}
		if fresh {
			if err := simpleFactoryAssert(201, "result != 0", func() bool { return value != (ast.ASTHandle{}) }); err != nil {
				return ast.ASTHandle{}, err
			}
			return value, nil
		}
	}
}

func (f *userSortFactory) RegisterValue(value ast.ASTHandle) error {
	if err := f.open(); err != nil {
		return err
	}
	sort, err := f.manager.GetSort(value)
	if err != nil {
		return err
	}
	if err := valueFactoryAssert("src/smt/proto_model/value_factory.cpp", 111, "!is_finite(m_manager.get_sort(n))", func() bool { return !f.finite[sort] }); err != nil {
		return err
	}
	set, err := f.valueSet(sort)
	if err != nil {
		return err
	}
	if _, found := set.seen[value]; found {
		return nil
	}

	ref, err := f.manager.NewRef(value)
	if err != nil {
		return err
	}
	f.values = append(f.values, ref)
	set.seen[value] = struct{}{}
	set.values = append(set.values, value)
	return nil
}

// Member destruction releases sorts before the value vector. Values keep
// their own sort references alive until the later value-vector release.
// Each vector decrements its entries in forward insertion order (ref_vector.h).
func (f *userSortFactory) Close() (err error) {
	if f == nil || f.manager == nil {
		return nil
	}
	f.sets, f.finite = nil, nil
	for i := range f.sorts {
		// DelRef may consume the token before returning a numerical error.
		if f.sorts[i] == (ast.ASTRef{}) {
			continue
		}
		err = errors.Join(err, f.manager.DelRef(&f.sorts[i]))
	}
	for i := range f.values {
		// DelRef may consume the token before returning a numerical error.
		if f.values[i] == (ast.ASTRef{}) {
			continue
		}
		err = errors.Join(err, f.manager.DelRef(&f.values[i]))
	}
	if err == nil {
		f.values, f.sorts = nil, nil
		f.manager = nil
	}
	return err
}

func (f *userSortFactory) SomeValue(sort ast.ASTHandle) (ast.ASTHandle, error) {
	if err := f.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if !f.finite[sort] {
		return f.someValue(sort)
	}
	set := f.sets[sort]
	if err := valueFactoryAssert("src/smt/proto_model/value_factory.cpp", 82, "set != 0", func() bool { return set != nil }); err != nil {
		return ast.ASTHandle{}, err
	}
	if err := valueFactoryAssert("src/smt/proto_model/value_factory.cpp", 83, "!set->m_values.empty()", func() bool { return set != nil && len(set.values) > 0 }); err != nil {
		return ast.ASTHandle{}, err
	}
	if set == nil || len(set.values) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return set.values[0], nil
}
func (f *userSortFactory) SomeValues(sort ast.ASTHandle, first, second ast.ASTRef) (bool, error) {
	if err := f.open(); err != nil {
		return false, err
	}
	if !f.finite[sort] {
		return f.someValues(sort, first, second)
	}
	set := f.sets[sort]
	if set == nil || len(set.values) < 2 {
		return false, nil
	}
	if err := f.manager.SetRef(first, set.values[0]); err != nil {
		return false, err
	}
	if err := f.manager.SetRef(second, set.values[1]); err != nil {
		return false, err
	}
	return true, nil
}
func (f *userSortFactory) FreshValue(sort ast.ASTHandle) (ast.ASTHandle, error) {
	if err := f.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if f.finite[sort] {
		return ast.ASTHandle{}, nil
	}
	return f.freshValue(sort)
}
func (f *userSortFactory) Freeze(sort ast.ASTHandle) error {
	if err := f.open(); err != nil {
		return err
	}
	if f.finite[sort] {
		return nil
	}
	set := f.sets[sort]
	if set == nil || len(set.values) == 0 {
		if _, err := f.SomeValue(sort); err != nil {
			return err
		}
	}
	if err := valueFactoryAssert("src/smt/proto_model/value_factory.cpp", 65, "m_sort2value_set.find(s, set) && set != 0 && !set->m_values.empty()", func() bool { set = f.sets[sort]; return set != nil && len(set.values) > 0 }); err != nil {
		return err
	}
	f.finite[sort] = true
	return nil
}
func (f *userSortFactory) IsFinite(sort ast.ASTHandle) bool { return f.finite[sort] }
func (f *userSortFactory) KnownUniverse(sort ast.ASTHandle) []ast.ASTHandle {
	if set := f.sets[sort]; set != nil {
		return append([]ast.ASTHandle(nil), set.values...)
	}
	return nil
}
func (f *userSortFactory) Sorts() []ast.ASTHandle {
	var sorts []ast.ASTHandle
	for _, ref := range f.sorts {
		sort, err := f.manager.RefGet(ref)
		if err == nil {
			sorts = append(sorts, sort)
		}
	}
	return sorts
}

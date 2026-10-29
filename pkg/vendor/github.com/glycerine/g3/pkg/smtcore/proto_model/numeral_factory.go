package protomodel

// Source: src/smt/proto_model/numeral_factory.cpp/.h and
// concrete simple_factory<rational> in value_factory.h:79-221.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type numeralValueSet struct {
	values []ast.ASTHandle
	seen   map[ast.ASTHandle]struct{}
	next   big.Rat
}

// numeralFactory implements the retained rational simple_factory. A slice plus
// membership map replaces the source unordered set; selection among registered
// values uses insertion order rather than exposing C++ hash-table layout.
type numeralFactory struct {
	manager   *ast.ASTManager
	family    ast.FamilyID
	sets      map[ast.ASTHandle]*numeralValueSet
	bitVector bool
	sorts     []ast.ASTRef
	values    []ast.ASTRef
}

type ArithmeticFactory struct{ *numeralFactory }
type BitVectorFactory struct{ *numeralFactory }

func newNumeralFactory(manager *ast.ASTManager, family ast.FamilyID, bv bool) *numeralFactory {
	return &numeralFactory{manager: manager, family: family, bitVector: bv, sets: make(map[ast.ASTHandle]*numeralValueSet)}
}
func NewArithmeticFactory(manager *ast.ASTManager) (*ArithmeticFactory, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	family, err := manager.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	return &ArithmeticFactory{newNumeralFactory(manager, family, false)}, nil
}
func NewBitVectorFactory(manager *ast.ASTManager) (*BitVectorFactory, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	family, err := manager.BitVectorUtilityFamilyID()
	if err != nil {
		return nil, err
	}
	return &BitVectorFactory{newNumeralFactory(manager, family, true)}, nil
}

func (f *numeralFactory) FamilyID() ast.FamilyID { return f.family }

func (f *numeralFactory) open() error {
	if f == nil || f.manager == nil || f.sets == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (f *numeralFactory) valueSet(sort ast.ASTHandle) (*numeralValueSet, error) {
	if err := f.open(); err != nil {
		return nil, err
	}

	set := f.sets[sort]
	if set == nil {
		// Preserve simple_factory::get_value_set publication order: the set and
		// map entry precede the owning sort-vector push.
		set = &numeralValueSet{seen: make(map[ast.ASTHandle]struct{})}
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
		var set2 *numeralValueSet
		if err := simpleFactoryAssert(108, "m_sort2value_set.find(s, set2)", func() bool { set2 = f.sets[sort]; return set2 != nil }); err != nil {
			return nil, err
		}
		if err := simpleFactoryAssert(109, "set == set2", func() bool { return set == set2 }); err != nil {
			return nil, err
		}
	}
	return set, nil
}

func (f *numeralFactory) insert(value *big.Rat, sort ast.ASTHandle) (ast.ASTHandle, bool, error) {
	set, err := f.valueSet(sort)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	numeral, err := f.makeValue(value, sort)
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

func (f *numeralFactory) SomeValue(sort ast.ASTHandle) (ast.ASTHandle, error) {
	if err := f.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if set := f.sets[sort]; set != nil && len(set.values) != 0 {
		return set.values[0], nil
	}
	value, _, err := f.insert(big.NewRat(0, 1), sort)
	return value, err
}

func (f *numeralFactory) SomeValues(sort ast.ASTHandle, first, second ast.ASTRef) (bool, error) {
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
		value, _, err := f.insert(big.NewRat(0, 1), sort)
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
			value, _, err = f.insert(big.NewRat(1, 1), sort)
			if err != nil {
				return false, err
			}
			if err = f.manager.SetRef(second, value); err != nil {
				return false, err
			}
		}
	} else {
		value, _, err := f.insert(big.NewRat(0, 1), sort)
		if err != nil {
			return false, err
		}
		if err = f.manager.SetRef(first, value); err != nil {
			return false, err
		}
		value, _, err = f.insert(big.NewRat(1, 1), sort)
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

func (f *numeralFactory) FreshValue(sort ast.ASTHandle) (ast.ASTHandle, error) {
	set, err := f.valueSet(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	size, err := f.manager.GetSortSize(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var maxSize big.Rat
	hasMax := false
	if size.IsFinite() {
		n, e := size.Size()
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if n < uint64(^uint32(0)) {
			maxSize.SetInt(new(big.Int).SetUint64(n))
			hasMax = true
		}
	}
	start := new(big.Rat).Set(&set.next)
	limit := new(big.Rat).Add(&maxSize, start)
	for {
		value, fresh, err := f.insert(&set.next, sort)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		set.next.Add(&set.next, big.NewRat(1, 1))
		if hasMax && set.next.Cmp(limit) > 0 {
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

func (f *numeralFactory) RegisterValue(value ast.ASTHandle) error {
	if err := f.open(); err != nil {
		return err
	}
	sort, err := f.manager.GetSort(value)
	if err != nil {
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
func (f *numeralFactory) Close() (err error) {
	if f == nil || f.manager == nil {
		return nil
	}
	f.sets = nil
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

func (f *numeralFactory) makeValue(value *big.Rat, sort ast.ASTHandle) (ast.ASTHandle, error) {
	if value == nil {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if f.bitVector {
		bits, err := f.manager.BitVectorSize(sort)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return f.manager.BitVectorNumeralRat(value, bits)
	}
	family, err := f.manager.GetFamilyID(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if family != f.family {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	kind, err := f.manager.GetDeclKind(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return f.manager.ArithmeticNumeralRat(value, kind == 1)
}
func (f *ArithmeticFactory) MkNumValue(value *big.Rat, integer bool) (ast.ASTHandle, error) {
	if f == nil || f.numeralFactory == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	if err := f.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	kind := int32(0)
	if integer {
		kind = 1
	}
	sort, err := f.manager.MkFamilySort(f.family, kind, nil)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	numeral, _, err := f.insert(value, sort)
	return numeral, err
}
func (f *BitVectorFactory) MkNumValue(value *big.Rat, bits uint32) (ast.ASTHandle, error) {
	if f == nil || f.numeralFactory == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	if err := f.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := f.manager.BitVectorSort(bits)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	numeral, _, err := f.insert(value, sort)
	return numeral, err
}

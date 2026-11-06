package api

// High-level array construction over the public SMT context.

import (
	"github.com/glycerine/g3/pkg/ast"
)

// ArraySort constructs one SMT array sort with one or more index sorts. The
// range may itself be an array sort.
func (c *SMTContext) ArraySort(indexSorts []ast.ASTHandle, rangeSort ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if len(indexSorts) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	manager, err := c.command.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkArraySort(ast.SliceArguments(indexSorts), rangeSort)
}

// NestedArraySort constructs Array I0 (Array I1 (... range)) for the supplied
// outer-to-inner index sorts.
func (c *SMTContext) NestedArraySort(indexSorts []ast.ASTHandle, rangeSort ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if len(indexSorts) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	result := rangeSort
	var err error
	for index := len(indexSorts) - 1; index >= 0; index-- {
		result, err = c.ArraySort([]ast.ASTHandle{indexSorts[index]}, result)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return result, nil
}

// ConstantArray constructs a constant array of the supplied complete array
// sort. The value must have the array range sort.
func (c *SMTContext) ConstantArray(arraySort, value ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	manager, err := c.command.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkConstArray(arraySort, value)
}

// ArrayMap applies a function pointwise to one or more arrays with matching
// index sorts.
func (c *SMTContext) ArrayMap(function ast.ASTHandle, arrays ...ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if len(arrays) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	manager, err := c.command.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkArrayMap(function, ast.SliceArguments(arrays))
}

// ArrayDefault returns the default value associated with an array.
func (c *SMTContext) ArrayDefault(array ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	manager, err := c.command.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	family, err := manager.ArrayFamilyID()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkFamilyApp(family, ast.ASTArrayOpDefault, nil, []ast.ASTHandle{array})
}

// ArrayExt constructs the first-index extensionality witness for two arrays.
func (c *SMTContext) ArrayExt(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	manager, err := c.command.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	family, err := manager.ArrayFamilyID()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkFamilyApp(family, ast.ASTArrayOpExt, nil, []ast.ASTHandle{left, right})
}

// AsArray constructs an array whose entries are supplied by a function.
func (c *SMTContext) AsArray(function ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	manager, err := c.command.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkAsArray(function)
}

// ArrayArity returns the number of index sorts in an array sort.
func (c *SMTContext) ArrayArity(arraySort ast.ASTHandle) (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	manager, err := c.command.Manager()
	if err != nil {
		return 0, err
	}
	return manager.ArrayArity(arraySort)
}

// ArrayDomain returns one index sort from an array sort.
func (c *SMTContext) ArrayDomain(arraySort ast.ASTHandle, index uint32) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	manager, err := c.command.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.ArrayDomain(arraySort, index)
}

// ArrayRange returns the value sort from an array sort.
func (c *SMTContext) ArrayRange(arraySort ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	manager, err := c.command.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.ArrayRange(arraySort)
}

// NestedConstantArray constructs a right-nested constant array whose leaf is
// value. Index sorts are ordered outermost to innermost.
func (c *SMTContext) NestedConstantArray(indexSorts []ast.ASTHandle, value ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if len(indexSorts) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	manager, err := c.command.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result := value
	for index := len(indexSorts) - 1; index >= 0; index-- {
		rangeSort, x := manager.GetSort(result)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		arraySort, x := c.ArraySort([]ast.ASTHandle{indexSorts[index]}, rangeSort)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		result, x = manager.MkConstArray(arraySort, result)
		if x != nil {
			return ast.ASTHandle{}, x
		}
	}
	return result, nil
}

// ArraySelect selects from a native multi-index array.
func (c *SMTContext) ArraySelect(array ast.ASTHandle, indices ...ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if len(indices) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	manager, err := c.command.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	family, err := manager.ArrayFamilyID()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arguments := append([]ast.ASTHandle{array}, indices...)
	return manager.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, arguments)
}

// ArrayStore updates one tuple in a native multi-index array.
func (c *SMTContext) ArrayStore(array ast.ASTHandle, indices []ast.ASTHandle, value ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if len(indices) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	manager, err := c.command.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	family, err := manager.ArrayFamilyID()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arguments := make([]ast.ASTHandle, 0, len(indices)+2)
	arguments = append(arguments, array)
	arguments = append(arguments, indices...)
	arguments = append(arguments, value)
	return manager.MkFamilyApp(family, ast.ASTArrayOpStore, nil, arguments)
}

// NestedArraySelect follows a path through right-nested one-index arrays.
func (c *SMTContext) NestedArraySelect(array ast.ASTHandle, indices ...ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if len(indices) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	result := array
	var err error
	for _, index := range indices {
		result, err = c.ArraySelect(result, index)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return result, nil
}

// NestedArrayStore functionally updates a leaf along a right-nested array path.
func (c *SMTContext) NestedArrayStore(array ast.ASTHandle, indices []ast.ASTHandle, value ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if len(indices) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if len(indices) == 1 {
		return c.ArrayStore(array, indices, value)
	}
	inner, err := c.ArraySelect(array, indices[0])
	if err != nil {
		return ast.ASTHandle{}, err
	}
	updated, err := c.NestedArrayStore(inner, indices[1:], value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.ArrayStore(array, indices[:1], updated)
}

// SetSort constructs the array-backed set sort for an element sort.
func (c *SMTContext) SetSort(elementSort ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	manager, err := c.command.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{elementSort}), boolean)
}

// EmptySet constructs an empty set over an element sort.
func (c *SMTContext) EmptySet(elementSort ast.ASTHandle) (ast.ASTHandle, error) {
	setSort, err := c.SetSort(elementSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	manager, err := c.command.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkEmptySet(setSort)
}

// FullSet constructs the set containing every value of an element sort.
func (c *SMTContext) FullSet(elementSort ast.ASTHandle) (ast.ASTHandle, error) {
	setSort, err := c.SetSort(elementSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	manager, err := c.command.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkFullSet(setSort)
}

func (c *SMTContext) setApplication(kind int32, sets []ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	manager, err := c.command.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	family, err := manager.ArrayFamilyID()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkFamilyApp(family, kind, nil, sets)
}

// SetUnion constructs the union of one or more sets.
func (c *SMTContext) SetUnion(sets ...ast.ASTHandle) (ast.ASTHandle, error) {
	return c.setApplication(ast.ASTArrayOpSetUnion, sets)
}

// SetIntersection constructs the intersection of one or more sets.
func (c *SMTContext) SetIntersection(sets ...ast.ASTHandle) (ast.ASTHandle, error) {
	return c.setApplication(ast.ASTArrayOpSetIntersect, sets)
}

// SetDifference constructs the relative complement of right in left.
func (c *SMTContext) SetDifference(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	return c.setApplication(ast.ASTArrayOpSetDifference, []ast.ASTHandle{left, right})
}

// SetComplement constructs the complement of a set.
func (c *SMTContext) SetComplement(set ast.ASTHandle) (ast.ASTHandle, error) {
	return c.setApplication(ast.ASTArrayOpSetComplement, []ast.ASTHandle{set})
}

// SetSubset reports whether left is a subset of right.
func (c *SMTContext) SetSubset(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	return c.setApplication(ast.ASTArrayOpSetSubset, []ast.ASTHandle{left, right})
}

// SetMember reports whether an element belongs to a set.
func (c *SMTContext) SetMember(element, set ast.ASTHandle) (ast.ASTHandle, error) {
	return c.ArraySelect(set, element)
}

// SetAdd returns a set that additionally contains element.
func (c *SMTContext) SetAdd(set, element ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	manager, err := c.command.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	value, err := manager.MkTrue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.ArrayStore(set, []ast.ASTHandle{element}, value)
}

// SetDelete returns a set without element.
func (c *SMTContext) SetDelete(set, element ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	manager, err := c.command.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	value, err := manager.MkFalse()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.ArrayStore(set, []ast.ASTHandle{element}, value)
}

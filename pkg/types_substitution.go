package apalache

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// EqClass groups type-variable numbers that unification has made equivalent.
// Identity equality is provided by ID; DeepEqual compares its variable set.
type EqClass struct {
	id        UID
	variables map[int]struct{}
	represent int
}

func NewEqClass(variables ...int) *EqClass {
	if len(variables) == 0 {
		panic("an equivalence class must not be empty")
	}
	class := &EqClass{id: NewUID()}
	class.SetVariables(variables...)
	return class
}
func (class *EqClass) ID() UID             { return class.id }
func (class *EqClass) Representative() int { return class.represent }
func (class *EqClass) Variables() []int {
	result := make([]int, 0, len(class.variables))
	for variable := range class.variables {
		result = append(result, variable)
	}
	sort.Ints(result)
	return result
}
func (class *EqClass) SetVariables(variables ...int) {
	if len(variables) == 0 {
		panic("an equivalence class must not be empty")
	}
	class.variables = make(map[int]struct{}, len(variables))
	for _, variable := range variables {
		if variable < 0 {
			panic("an equivalence class requires non-negative type variables")
		}
		class.variables[variable] = struct{}{}
	}
	values := class.Variables()
	class.represent = values[0]
}
func (class *EqClass) Copy() *EqClass { return NewEqClass(class.Variables()...) }
func (class *EqClass) DeepEqual(other *EqClass) bool {
	if class == nil || other == nil || len(class.variables) != len(other.variables) {
		return class == nil && other == nil
	}
	for variable := range class.variables {
		if _, ok := other.variables[variable]; !ok {
			return false
		}
	}
	return true
}

type SubstitutionEntry struct {
	Class *EqClass
	Type  Type
}

// Substitution maps disjoint equivalence classes to types.
type Substitution struct {
	entries    []SubstitutionEntry
	varToEntry map[int]int
}

func NewSubstitution(entries ...SubstitutionEntry) (*Substitution, error) {
	result := &Substitution{entries: make([]SubstitutionEntry, len(entries)), varToEntry: make(map[int]int)}
	for index, entry := range entries {
		if entry.Class == nil || entry.Type == nil {
			return nil, fmt.Errorf("substitution entry %d requires a class and type", index)
		}
		classCopy := entry.Class.Copy()
		result.entries[index] = SubstitutionEntry{Class: classCopy, Type: entry.Type}
		for _, variable := range classCopy.Variables() {
			if previous, exists := result.varToEntry[variable]; exists {
				return nil, fmt.Errorf("type variable %d occurs in substitution classes %d and %d", variable, previous, index)
			}
			result.varToEntry[variable] = index
		}
	}
	return result, nil
}

func EmptySubstitution() *Substitution {
	result, _ := NewSubstitution()
	return result
}
func (substitution *Substitution) IsEmpty() bool {
	return substitution == nil || len(substitution.entries) == 0
}
func (substitution *Substitution) Entries() []SubstitutionEntry {
	result := make([]SubstitutionEntry, len(substitution.entries))
	for index, entry := range substitution.entries {
		result[index] = SubstitutionEntry{Class: entry.Class.Copy(), Type: entry.Type}
	}
	return result
}

func (substitution *Substitution) String() string {
	if substitution == nil || len(substitution.entries) == 0 {
		return "Sub{}"
	}
	entries := substitution.Entries()
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Class.Representative() < entries[j].Class.Representative()
	})
	parts := make([]string, len(entries))
	for index, entry := range entries {
		variables := entry.Class.Variables()
		names := make([]string, len(variables))
		for variableIndex, variable := range variables {
			typed, err := NewVarType(variable)
			if err != nil {
				names[variableIndex] = fmt.Sprintf("a%d", variable)
			} else {
				names[variableIndex] = typed.String()
			}
		}
		parts[index] = fmt.Sprintf("[%s] -> %s", strings.Join(names, ", "), entry.Type)
	}
	return "Sub{" + strings.Join(parts, ", ") + "}"
}

// ApplyOnce recursively walks a type, replacing each encountered variable at
// most once. A replacement itself is not revisited during the same call.
func (substitution *Substitution) ApplyOnce(typ Type) (Type, bool, error) {
	if typ == nil {
		return nil, false, fmt.Errorf("cannot substitute a nil type")
	}
	switch typed := typ.(type) {
	case VarType:
		index, ok := substitution.varToEntry[typed.no]
		if !ok {
			return typ, false, nil
		}
		result := substitution.entries[index].Type
		return result, !EqualTypes(typ, result), nil
	case primitiveType, ConstType:
		return typ, false, nil
	case SetType:
		element, changed, err := substitution.ApplyOnce(typed.element)
		if err != nil {
			return nil, false, err
		}
		result, _ := NewSetType(element)
		return result, changed, nil
	case SeqType:
		element, changed, err := substitution.ApplyOnce(typed.element)
		if err != nil {
			return nil, false, err
		}
		result, _ := NewSeqType(element)
		return result, changed, nil
	case FunType:
		argument, argumentChanged, err := substitution.ApplyOnce(typed.arg)
		if err != nil {
			return nil, false, err
		}
		resultType, resultChanged, err := substitution.ApplyOnce(typed.result)
		if err != nil {
			return nil, false, err
		}
		result, _ := NewFunType(argument, resultType)
		return result, argumentChanged || resultChanged, nil
	case TupleType:
		elements, changed, err := substitution.applySlice(typed.elements)
		if err != nil {
			return nil, false, err
		}
		result, _ := NewTupleType(elements...)
		return result, changed, nil
	case SparseTupleType:
		fields, changed, err := substitution.applyIntFields(typed.fields)
		if err != nil {
			return nil, false, err
		}
		result, _ := NewSparseTupleType(fields)
		return result, changed, nil
	case RecordType:
		fields, changed, err := substitution.applyFields(typed.fields)
		if err != nil {
			return nil, false, err
		}
		result, _ := NewRecordType(fields)
		return result, changed, nil
	case OperatorType:
		arguments, argumentsChanged, err := substitution.applySlice(typed.arguments)
		if err != nil {
			return nil, false, err
		}
		resultType, resultChanged, err := substitution.ApplyOnce(typed.result)
		if err != nil {
			return nil, false, err
		}
		result, _ := NewOperatorType(arguments, resultType)
		return result, argumentsChanged || resultChanged, nil
	case RowType:
		return substitution.applyRow(typed)
	case RecordRowType:
		rowType, changed, err := substitution.applyRow(typed.row)
		if err != nil {
			return nil, false, err
		}
		return NewRecordRowType(rowType), changed, nil
	case VariantType:
		rowType, changed, err := substitution.applyRow(typed.row)
		if err != nil {
			return nil, false, err
		}
		return NewVariantType(rowType), changed, nil
	default:
		return nil, false, fmt.Errorf("unsupported type %T", typ)
	}
}

func (substitution *Substitution) applySlice(types []Type) ([]Type, bool, error) {
	result := make([]Type, len(types))
	changed := false
	for index, typ := range types {
		var itemChanged bool
		var err error
		result[index], itemChanged, err = substitution.ApplyOnce(typ)
		if err != nil {
			return nil, false, err
		}
		changed = changed || itemChanged
	}
	return result, changed, nil
}

func (substitution *Substitution) applyFields(fields map[string]Type) (map[string]Type, bool, error) {
	result := make(map[string]Type, len(fields))
	changed := false
	for name, typ := range fields {
		newType, itemChanged, err := substitution.ApplyOnce(typ)
		if err != nil {
			return nil, false, err
		}
		result[name] = newType
		changed = changed || itemChanged
	}
	return result, changed, nil
}

func (substitution *Substitution) applyIntFields(fields map[int]Type) (map[int]Type, bool, error) {
	result := make(map[int]Type, len(fields))
	changed := false
	for index, typ := range fields {
		newType, itemChanged, err := substitution.ApplyOnce(typ)
		if err != nil {
			return nil, false, err
		}
		result[index] = newType
		changed = changed || itemChanged
	}
	return result, changed, nil
}

func (substitution *Substitution) applyRow(row RowType) (RowType, bool, error) {
	fields, fieldsChanged, err := substitution.applyFields(row.fields)
	if err != nil {
		return RowType{}, false, err
	}
	if row.other == nil {
		result, _ := NewRowType(fields, nil)
		return result, fieldsChanged, nil
	}
	tail, tailChanged, err := substitution.ApplyOnce(*row.other)
	if err != nil {
		return RowType{}, false, err
	}
	switch typedTail := tail.(type) {
	case VarType:
		result, _ := NewRowType(fields, &typedTail)
		return result, fieldsChanged || tailChanged, nil
	case RowType:
		merged := typedTail.Fields()
		for name, typ := range fields {
			merged[name] = typ
		}
		other, ok := typedTail.Other()
		if ok {
			result, _ := NewRowType(merged, &other)
			return result, fieldsChanged || tailChanged, nil
		}
		result, _ := NewRowType(merged, nil)
		return result, fieldsChanged || tailChanged, nil
	default:
		return RowType{}, false, fmt.Errorf("expected a variable or row after substituting row tail, found %T", tail)
	}
}

const recursiveSubstitutionLimit = 100000

func (substitution *Substitution) ApplyRecursive(typ Type) (Type, error) {
	current := typ
	for iteration := 0; iteration < recursiveSubstitutionLimit; iteration++ {
		next, changed, err := substitution.ApplyOnce(current)
		if err != nil {
			return nil, err
		}
		if !changed {
			return next, nil
		}
		current = next
	}
	return nil, fmt.Errorf("recursive substitution took more than %d iterations; cyclic substitution", recursiveSubstitutionLimit)
}

func EqualSubstitutions(left, right *Substitution) bool {
	if left == nil || right == nil || len(left.entries) != len(right.entries) {
		return left == nil && right == nil
	}
	used := make([]bool, len(right.entries))
	for _, leftEntry := range left.entries {
		found := false
		for index, rightEntry := range right.entries {
			if !used[index] && leftEntry.Class.DeepEqual(rightEntry.Class) && EqualTypes(leftEntry.Type, rightEntry.Type) {
				used[index], found = true, true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

type TypeVarPool struct{ next int }

func NewTypeVarPool(start int) *TypeVarPool {
	return &TypeVarPool{next: start}
}
func (pool *TypeVarPool) Size() int { return pool.next }
func (pool *TypeVarPool) Fresh() VarType {
	result, err := NewVarType(pool.next)
	if err != nil {
		panic(err)
	}
	pool.next++
	return result
}
func (pool *TypeVarPool) FreshN(count int) []VarType {
	if count < 0 {
		pool.next += count
		return []VarType{}
	}
	result := make([]VarType, count)
	for index := range result {
		variable, err := NewVarType(pool.next + index)
		if err != nil {
			panic(err)
		}
		result[index] = variable
	}
	pool.next += count
	return result
}

var modelValuePattern = regexp.MustCompile(`^([a-zA-Z0-9_]+)_OF_([A-Z_][A-Z0-9_]*)$`)

func ParseModelValue(text string) (ConstType, string, bool) {
	match := modelValuePattern.FindStringSubmatch(text)
	if match == nil {
		return ConstType{}, "", false
	}
	typ, err := NewConstType(match[2])
	if err != nil {
		return ConstType{}, "", false
	}
	return typ, match[1], true
}
func IsModelValue(text string) bool {
	_, _, ok := ParseModelValue(text)
	return ok
}
func ModelValueOrStringType(text string) Type {
	if typ, _, ok := ParseModelValue(text); ok {
		return typ
	}
	return StrT1
}
func ConstructModelValue(typeName, valueName string) string {
	return valueName + "_OF_" + typeName
}

package apalache

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Type is a Type System 1 type (Scala TlaType1). Implementations are immutable:
// constructors copy slices/maps and accessors return copies.
type Type interface {
	fmt.Stringer
	UsedNames() []int
	IsMono() bool
	tlaType1()
}

type primitiveType string

func (primitiveType) tlaType1()        {}
func (t primitiveType) String() string { return string(t) }
func (primitiveType) UsedNames() []int { return nil }
func (primitiveType) IsMono() bool     { return true }

const (
	IntT1  primitiveType = "Int"
	RealT1 primitiveType = "Real"
	BoolT1 primitiveType = "Bool"
	StrT1  primitiveType = "Str"
)

var uninterpretedTypeName = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

// ConstType is an uninterpreted type or a type-alias reference.
type ConstType struct{ name string }

func (ConstType) tlaType1()                {}
func (t ConstType) String() string         { return t.name }
func (ConstType) UsedNames() []int         { return nil }
func (ConstType) IsMono() bool             { return true }
func (t ConstType) Name() string           { return t.name }
func IsUninterpretedType(name string) bool { return uninterpretedTypeName.MatchString(name) }
func IsAliasReference(name string) bool    { return strings.HasPrefix(name, "$") }

func NewConstType(name string) (ConstType, error) {
	if !IsUninterpretedType(name) && !IsAliasReference(name) {
		return ConstType{}, fmt.Errorf("constant type accepts an uppercase identifier or $aliasReference, found %q", name)
	}
	return ConstType{name: name}, nil
}

func AliasName(reference string) (string, error) {
	if !IsAliasReference(reference) {
		return "", fmt.Errorf("expected %q to start with $", reference)
	}
	return strings.TrimPrefix(reference, "$"), nil
}

// VarType is a non-negative type-variable identifier.
type VarType struct{ no int }

func (VarType) tlaType1() {}
func (v VarType) String() string {
	if v.no < 26 {
		return string(rune('a' + v.no))
	}
	return "a" + strconv.Itoa(v.no)
}
func (v VarType) UsedNames() []int { return []int{v.no} }
func (VarType) IsMono() bool       { return false }
func (v VarType) Number() int      { return v.no }

func NewVarType(no int) (VarType, error) {
	if no < 0 {
		return VarType{}, fmt.Errorf("variable identifier must be non-negative, found %d", no)
	}
	return VarType{no: no}, nil
}

func ParseVarType(name string) (VarType, error) {
	if len(name) == 1 && name[0] >= 'a' && name[0] <= 'z' {
		return VarType{no: int(name[0] - 'a')}, nil
	}
	if len(name) < 2 || name[0] != 'a' {
		return VarType{}, fmt.Errorf("expected a lowercase letter or a[0-9]+, found %q", name)
	}
	no, err := strconv.ParseUint(name[1:], 10, 31)
	if err != nil {
		return VarType{}, fmt.Errorf("expected a lowercase letter or a[0-9]+, found %q", name)
	}
	return VarType{no: int(no)}, nil
}

func IsValidVarTypeName(name string) bool {
	_, err := ParseVarType(name)
	return err == nil
}

type FunType struct{ arg, result Type }

func (FunType) tlaType1()          {}
func (t FunType) String() string   { return fmt.Sprintf("(%s -> %s)", t.arg, t.result) }
func (t FunType) UsedNames() []int { return unionNames(t.arg, t.result) }
func (t FunType) IsMono() bool     { return t.arg.IsMono() && t.result.IsMono() }
func (t FunType) Argument() Type   { return t.arg }
func (t FunType) Result() Type     { return t.result }

func NewFunType(arg, result Type) (FunType, error) {
	if err := requireTypes(arg, result); err != nil {
		return FunType{}, err
	}
	return FunType{arg: arg, result: result}, nil
}

type SetType struct{ element Type }

func (SetType) tlaType1()          {}
func (t SetType) String() string   { return fmt.Sprintf("Set(%s)", t.element) }
func (t SetType) UsedNames() []int { return copyInts(t.element.UsedNames()) }
func (t SetType) IsMono() bool     { return t.element.IsMono() }
func (t SetType) Element() Type    { return t.element }

func NewSetType(element Type) (SetType, error) {
	if err := requireTypes(element); err != nil {
		return SetType{}, err
	}
	return SetType{element: element}, nil
}

type SeqType struct{ element Type }

func (SeqType) tlaType1()          {}
func (t SeqType) String() string   { return fmt.Sprintf("Seq(%s)", t.element) }
func (t SeqType) UsedNames() []int { return copyInts(t.element.UsedNames()) }
func (t SeqType) IsMono() bool     { return t.element.IsMono() }
func (t SeqType) Element() Type    { return t.element }

func NewSeqType(element Type) (SeqType, error) {
	if err := requireTypes(element); err != nil {
		return SeqType{}, err
	}
	return SeqType{element: element}, nil
}

type TupleType struct{ elements []Type }

func (TupleType) tlaType1() {}
func (t TupleType) String() string {
	return "<<" + joinTypes(t.elements, ", ") + ">>"
}
func (t TupleType) UsedNames() []int { return unionNames(t.elements...) }
func (t TupleType) IsMono() bool     { return allMono(t.elements) }
func (t TupleType) Elements() []Type { return append([]Type(nil), t.elements...) }

func NewTupleType(elements ...Type) (TupleType, error) {
	if err := requireTypes(elements...); err != nil {
		return TupleType{}, err
	}
	return TupleType{elements: append([]Type(nil), elements...)}, nil
}

type SparseTupleType struct{ fields map[int]Type }

func (SparseTupleType) tlaType1() {}
func (t SparseTupleType) String() string {
	keys := make([]int, 0, len(t.fields))
	for key := range t.fields {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%d: %s", key, t.fields[key]))
	}
	return "<| " + strings.Join(parts, ", ") + " |>"
}
func (t SparseTupleType) UsedNames() []int     { return unionNames(mapIntValues(t.fields)...) }
func (t SparseTupleType) IsMono() bool         { return allMono(mapIntValues(t.fields)) }
func (t SparseTupleType) Fields() map[int]Type { return cloneIntTypeMap(t.fields) }

func NewSparseTupleType(fields map[int]Type) (SparseTupleType, error) {
	if err := requireTypes(mapIntValues(fields)...); err != nil {
		return SparseTupleType{}, err
	}
	return SparseTupleType{fields: cloneIntTypeMap(fields)}, nil
}

// RecordType is the legacy, imprecise record type (Scala RecT1).
type RecordType struct{ fields map[string]Type }

func (RecordType) tlaType1() {}
func (t RecordType) String() string {
	return "[" + joinFields(t.fields, ", ") + "]"
}
func (t RecordType) UsedNames() []int        { return unionNames(mapStringValues(t.fields)...) }
func (t RecordType) IsMono() bool            { return allMono(mapStringValues(t.fields)) }
func (t RecordType) Fields() map[string]Type { return cloneStringTypeMap(t.fields) }

func NewRecordType(fields map[string]Type) (RecordType, error) {
	if err := requireTypes(mapStringValues(fields)...); err != nil {
		return RecordType{}, err
	}
	return RecordType{fields: cloneStringTypeMap(fields)}, nil
}

type OperatorType struct {
	arguments []Type
	result    Type
}

func (OperatorType) tlaType1() {}
func (t OperatorType) String() string {
	return "((" + joinTypes(t.arguments, ", ") + ") => " + t.result.String() + ")"
}
func (t OperatorType) UsedNames() []int  { return unionNames(append(t.Arguments(), t.result)...) }
func (t OperatorType) IsMono() bool      { return allMono(append(t.Arguments(), t.result)) }
func (t OperatorType) Arguments() []Type { return append([]Type(nil), t.arguments...) }
func (t OperatorType) Result() Type      { return t.result }

func NewOperatorType(arguments []Type, result Type) (OperatorType, error) {
	if err := requireTypes(append(append([]Type(nil), arguments...), result)...); err != nil {
		return OperatorType{}, err
	}
	return OperatorType{arguments: append([]Type(nil), arguments...), result: result}, nil
}

// RowType is an ADR-014 row. other is the optional open-row variable.
type RowType struct {
	fields map[string]Type
	other  *VarType
}

func (RowType) tlaType1() {}
func (t RowType) String() string {
	pairs := joinFields(t.fields, " | ")
	if t.other == nil {
		if pairs == "" {
			return "(||)"
		}
		return "(| " + pairs + " |)"
	}
	if pairs == "" {
		return "(| " + t.other.String() + " |)"
	}
	return "(| " + pairs + " | " + t.other.String() + " |)"
}
func (t RowType) UsedNames() []int {
	types := mapStringValues(t.fields)
	if t.other != nil {
		types = append(types, *t.other)
	}
	return unionNames(types...)
}
func (t RowType) IsMono() bool            { return len(t.UsedNames()) == 0 }
func (t RowType) Fields() map[string]Type { return cloneStringTypeMap(t.fields) }
func (t RowType) Other() (VarType, bool) {
	if t.other == nil {
		return VarType{}, false
	}
	return *t.other, true
}

func NewRowType(fields map[string]Type, other *VarType) (RowType, error) {
	if err := requireTypes(mapStringValues(fields)...); err != nil {
		return RowType{}, err
	}
	var otherCopy *VarType
	if other != nil {
		value := *other
		otherCopy = &value
	}
	return RowType{fields: cloneStringTypeMap(fields), other: otherCopy}, nil
}

type RecordRowType struct{ row RowType }

func (RecordRowType) tlaType1() {}
func (t RecordRowType) String() string {
	fields := joinFields(t.row.fields, ", ")
	if t.row.other == nil {
		return "{ " + fields + " }"
	}
	if fields == "" {
		return "{ " + t.row.other.String() + " }"
	}
	return "{ " + fields + ", " + t.row.other.String() + " }"
}
func (t RecordRowType) UsedNames() []int         { return t.row.UsedNames() }
func (t RecordRowType) IsMono() bool             { return t.row.IsMono() }
func (t RecordRowType) Row() RowType             { return t.row }
func NewRecordRowType(row RowType) RecordRowType { return RecordRowType{row: row} }

type VariantType struct{ row RowType }

func (VariantType) tlaType1() {}
func (t VariantType) String() string {
	keys := sortedStringKeys(t.row.fields)
	options := make([]string, 0, len(keys))
	for _, key := range keys {
		options = append(options, fmt.Sprintf("%s(%s)", key, t.row.fields[key]))
	}
	text := strings.Join(options, " | ")
	if t.row.other == nil {
		return text
	}
	if text == "" {
		return "Variant(" + t.row.other.String() + ")"
	}
	return text + " | " + t.row.other.String()
}
func (t VariantType) UsedNames() []int       { return t.row.UsedNames() }
func (t VariantType) IsMono() bool           { return t.row.IsMono() }
func (t VariantType) Row() RowType           { return t.row }
func NewVariantType(row RowType) VariantType { return VariantType{row: row} }

// TypeScheme is a Damas-Milner principal type and its quantified variables.
type TypeScheme struct {
	principal  Type
	quantified []int
}

func (s TypeScheme) Principal() Type   { return s.principal }
func (s TypeScheme) Quantified() []int { return copyInts(s.quantified) }

func NewTypeScheme(principal Type, quantified ...int) (TypeScheme, error) {
	if err := requireTypes(principal); err != nil {
		return TypeScheme{}, err
	}
	quantified = copyInts(quantified)
	for _, no := range quantified {
		if no < 0 {
			return TypeScheme{}, fmt.Errorf("quantified variable must be non-negative, found %d", no)
		}
	}
	sort.Ints(quantified)
	quantified = slices.Compact(quantified)
	return TypeScheme{principal: principal, quantified: quantified}, nil
}

// EqualTypes performs structural Type System 1 equality.
func EqualTypes(left, right Type) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	switch l := left.(type) {
	case primitiveType:
		r, ok := right.(primitiveType)
		return ok && l == r
	case ConstType:
		r, ok := right.(ConstType)
		return ok && l.name == r.name
	case VarType:
		r, ok := right.(VarType)
		return ok && l.no == r.no
	case FunType:
		r, ok := right.(FunType)
		return ok && EqualTypes(l.arg, r.arg) && EqualTypes(l.result, r.result)
	case SetType:
		r, ok := right.(SetType)
		return ok && EqualTypes(l.element, r.element)
	case SeqType:
		r, ok := right.(SeqType)
		return ok && EqualTypes(l.element, r.element)
	case TupleType:
		r, ok := right.(TupleType)
		return ok && equalTypeSlices(l.elements, r.elements)
	case SparseTupleType:
		r, ok := right.(SparseTupleType)
		return ok && equalIntTypeMaps(l.fields, r.fields)
	case RecordType:
		r, ok := right.(RecordType)
		return ok && equalStringTypeMaps(l.fields, r.fields)
	case OperatorType:
		r, ok := right.(OperatorType)
		return ok && equalTypeSlices(l.arguments, r.arguments) && EqualTypes(l.result, r.result)
	case RowType:
		r, ok := right.(RowType)
		return ok && equalStringTypeMaps(l.fields, r.fields) && equalOptionalVar(l.other, r.other)
	case RecordRowType:
		r, ok := right.(RecordRowType)
		return ok && EqualTypes(l.row, r.row)
	case VariantType:
		r, ok := right.(VariantType)
		return ok && EqualTypes(l.row, r.row)
	default:
		return false
	}
}

func requireTypes(types ...Type) error {
	for i, typ := range types {
		if typ == nil {
			return fmt.Errorf("type at index %d is nil", i)
		}
	}
	return nil
}

func unionNames(types ...Type) []int {
	set := make(map[int]struct{})
	for _, typ := range types {
		for _, no := range typ.UsedNames() {
			set[no] = struct{}{}
		}
	}
	result := make([]int, 0, len(set))
	for no := range set {
		result = append(result, no)
	}
	sort.Ints(result)
	return result
}

func allMono(types []Type) bool {
	for _, typ := range types {
		if !typ.IsMono() {
			return false
		}
	}
	return true
}

func copyInts(values []int) []int { return append([]int(nil), values...) }

func joinTypes(types []Type, separator string) string {
	parts := make([]string, len(types))
	for i, typ := range types {
		parts[i] = typ.String()
	}
	return strings.Join(parts, separator)
}

func joinFields(fields map[string]Type, separator string) string {
	keys := sortedStringKeys(fields)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+": "+fields[key].String())
	}
	return strings.Join(parts, separator)
}

func sortedStringKeys(fields map[string]Type) []string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func mapStringValues(fields map[string]Type) []Type {
	keys := sortedStringKeys(fields)
	values := make([]Type, 0, len(keys))
	for _, key := range keys {
		values = append(values, fields[key])
	}
	return values
}

func mapIntValues(fields map[int]Type) []Type {
	keys := make([]int, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	values := make([]Type, 0, len(keys))
	for _, key := range keys {
		values = append(values, fields[key])
	}
	return values
}

func cloneStringTypeMap(fields map[string]Type) map[string]Type {
	result := make(map[string]Type, len(fields))
	for key, value := range fields {
		result[key] = value
	}
	return result
}

func cloneIntTypeMap(fields map[int]Type) map[int]Type {
	result := make(map[int]Type, len(fields))
	for key, value := range fields {
		result[key] = value
	}
	return result
}

func equalTypeSlices(left, right []Type) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !EqualTypes(left[i], right[i]) {
			return false
		}
	}
	return true
}

func equalStringTypeMaps(left, right map[string]Type) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if !EqualTypes(value, right[key]) {
			return false
		}
	}
	return true
}

func equalIntTypeMaps(left, right map[int]Type) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if !EqualTypes(value, right[key]) {
			return false
		}
	}
	return true
}

func equalOptionalVar(left, right *VarType) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.no == right.no
}

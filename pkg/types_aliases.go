package apalache

import (
	"fmt"
	"sort"
	"strings"
)

const typeAliasClosureLimit = 100

// TypeAliasSubstitution replaces constant types used as alias references.
// Each Apply performs one recursive tree traversal; Closure computes the
// transitive fixed point of an alias environment.
type TypeAliasSubstitution struct{ context map[string]Type }

func NewTypeAliasSubstitution(context map[string]Type) *TypeAliasSubstitution {
	copy := make(map[string]Type, len(context))
	for name, typ := range context {
		if typ == nil {
			panic("type alias substitution requires non-nil types")
		}
		copy[name] = typ
	}
	return &TypeAliasSubstitution{context: copy}
}

func EmptyTypeAliasSubstitution() *TypeAliasSubstitution {
	return NewTypeAliasSubstitution(nil)
}

func (substitution *TypeAliasSubstitution) Context() map[string]Type {
	result := make(map[string]Type, len(substitution.context))
	for name, typ := range substitution.context {
		result[name] = typ
	}
	return result
}

func (substitution *TypeAliasSubstitution) String() string {
	names := make([]string, 0, len(substitution.context))
	for name := range substitution.context {
		names = append(names, name)
	}
	sort.Strings(names)
	pairs := make([]string, len(names))
	for index, name := range names {
		pairs[index] = fmt.Sprintf("%s -> %s", name, substitution.context[name])
	}
	return "ConstSub{" + strings.Join(pairs, ", ") + "}"
}

// Equal is the Go counterpart of Scala's context-based equals contract.
func (substitution *TypeAliasSubstitution) Equal(other *TypeAliasSubstitution) bool {
	if substitution == nil || other == nil {
		return substitution == other
	}
	if len(substitution.context) != len(other.context) {
		return false
	}
	for name, typ := range substitution.context {
		otherType, found := other.context[name]
		if !found || !EqualTypes(typ, otherType) {
			return false
		}
	}
	return true
}

func (substitution *TypeAliasSubstitution) Apply(typ Type) (Type, bool, error) {
	if typ == nil {
		return nil, false, fmt.Errorf("cannot apply aliases to a nil type")
	}
	switch typed := typ.(type) {
	case ConstType:
		if replacement, found := substitution.context[typed.Name()]; found {
			return replacement, true, nil
		}
		if IsAliasReference(typed.Name()) {
			name, _ := AliasName(typed.Name())
			return nil, false, fmt.Errorf("missing @typeAlias: %s = <type>;", name)
		}
		return typ, false, nil
	case primitiveType, VarType:
		return typ, false, nil
	case SetType:
		element, changed, err := substitution.Apply(typed.Element())
		if err != nil {
			return nil, false, err
		}
		result, _ := NewSetType(element)
		return result, changed, nil
	case SeqType:
		element, changed, err := substitution.Apply(typed.Element())
		if err != nil {
			return nil, false, err
		}
		result, _ := NewSeqType(element)
		return result, changed, nil
	case FunType:
		argument, argumentChanged, err := substitution.Apply(typed.Argument())
		if err != nil {
			return nil, false, err
		}
		resultType, resultChanged, err := substitution.Apply(typed.Result())
		if err != nil {
			return nil, false, err
		}
		result, _ := NewFunType(argument, resultType)
		return result, argumentChanged || resultChanged, nil
	case TupleType:
		elements, changed, err := substitution.applySlice(typed.Elements())
		if err != nil {
			return nil, false, err
		}
		result, _ := NewTupleType(elements...)
		return result, changed, nil
	case SparseTupleType:
		fields, changed, err := substitution.applyIntFields(typed.Fields())
		if err != nil {
			return nil, false, err
		}
		result, _ := NewSparseTupleType(fields)
		return result, changed, nil
	case RecordType:
		fields, changed, err := substitution.applyFields(typed.Fields())
		if err != nil {
			return nil, false, err
		}
		result, _ := NewRecordType(fields)
		return result, changed, nil
	case OperatorType:
		arguments, argumentsChanged, err := substitution.applySlice(typed.Arguments())
		if err != nil {
			return nil, false, err
		}
		resultType, resultChanged, err := substitution.Apply(typed.Result())
		if err != nil {
			return nil, false, err
		}
		result, _ := NewOperatorType(arguments, resultType)
		return result, argumentsChanged || resultChanged, nil
	case RowType:
		fields, changed, err := substitution.applyFields(typed.Fields())
		if err != nil {
			return nil, false, err
		}
		if other, ok := typed.Other(); ok {
			result, _ := NewRowType(fields, &other)
			return result, changed, nil
		}
		result, _ := NewRowType(fields, nil)
		return result, changed, nil
	case RecordRowType:
		row, changed, err := substitution.Apply(typed.Row())
		if err != nil {
			return nil, false, err
		}
		result, ok := row.(RowType)
		if !ok {
			return nil, false, fmt.Errorf("expected a row after alias substitution, found %T", row)
		}
		return NewRecordRowType(result), changed, nil
	case VariantType:
		row, changed, err := substitution.Apply(typed.Row())
		if err != nil {
			return nil, false, err
		}
		result, ok := row.(RowType)
		if !ok {
			return nil, false, fmt.Errorf("expected a row after alias substitution, found %T", row)
		}
		return NewVariantType(result), changed, nil
	default:
		return nil, false, fmt.Errorf("unsupported type %T", typ)
	}
}

func (substitution *TypeAliasSubstitution) applySlice(types []Type) ([]Type, bool, error) {
	result := make([]Type, len(types))
	changed := false
	for i, typ := range types {
		var itemChanged bool
		var err error
		result[i], itemChanged, err = substitution.Apply(typ)
		if err != nil {
			return nil, false, err
		}
		changed = changed || itemChanged
	}
	return result, changed, nil
}

func (substitution *TypeAliasSubstitution) applyFields(fields map[string]Type) (map[string]Type, bool, error) {
	result := make(map[string]Type, len(fields))
	changed := false
	for name, typ := range fields {
		newType, itemChanged, err := substitution.Apply(typ)
		if err != nil {
			return nil, false, err
		}
		result[name] = newType
		changed = changed || itemChanged
	}
	return result, changed, nil
}

func (substitution *TypeAliasSubstitution) applyIntFields(fields map[int]Type) (map[int]Type, bool, error) {
	result := make(map[int]Type, len(fields))
	changed := false
	for index, typ := range fields {
		newType, itemChanged, err := substitution.Apply(typ)
		if err != nil {
			return nil, false, err
		}
		result[index] = newType
		changed = changed || itemChanged
	}
	return result, changed, nil
}

func (substitution *TypeAliasSubstitution) Closure() (*TypeAliasSubstitution, error) {
	aliases := substitution.Context()
	if caller, callee, cyclic := findAliasCycle(aliases); cyclic {
		return nil, fmt.Errorf("cannot resolve a reference to type alias %s in type alias %s: cyclic dependency", callee, caller)
	}
	for iteration := 0; iteration < typeAliasClosureLimit; iteration++ {
		current := NewTypeAliasSubstitution(aliases)
		changed := false
		next := make(map[string]Type, len(aliases))
		for name, typ := range aliases {
			replaced, itemChanged, err := current.Apply(typ)
			if err != nil {
				return nil, err
			}
			next[name] = replaced
			changed = changed || itemChanged
		}
		aliases = next
		if !changed {
			return NewTypeAliasSubstitution(aliases), nil
		}
	}
	for caller, typ := range aliases {
		for _, callee := range aliasReferences(typ) {
			if _, defined := aliases[callee]; defined {
				return nil, fmt.Errorf("cannot resolve a reference to type alias %s in type alias %s: cyclic dependency", callee, caller)
			}
		}
	}
	return nil, fmt.Errorf("type alias closure did not converge")
}

func findAliasCycle(aliases map[string]Type) (caller string, callee string, cyclic bool) {
	names := make([]string, 0, len(aliases))
	for name := range aliases {
		names = append(names, name)
	}
	sort.Strings(names)
	state := make(map[string]uint8, len(aliases)) // 0 unseen, 1 active, 2 complete
	var visit func(string) (string, string, bool)
	visit = func(name string) (string, string, bool) {
		state[name] = 1
		references := aliasReferences(aliases[name])
		sort.Strings(references)
		for _, reference := range references {
			if _, defined := aliases[reference]; !defined {
				continue
			}
			switch state[reference] {
			case 1:
				return name, reference, true
			case 0:
				if cycleCaller, cycleCallee, found := visit(reference); found {
					return cycleCaller, cycleCallee, true
				}
			}
		}
		state[name] = 2
		return "", "", false
	}
	for _, name := range names {
		if state[name] == 0 {
			if cycleCaller, cycleCallee, found := visit(name); found {
				return cycleCaller, cycleCallee, true
			}
		}
	}
	return "", "", false
}

func aliasReferences(typ Type) []string {
	result := make([]string, 0)
	var visit func(Type)
	visit = func(current Type) {
		switch typed := current.(type) {
		case ConstType:
			result = append(result, typed.Name())
		case SetType:
			visit(typed.Element())
		case SeqType:
			visit(typed.Element())
		case FunType:
			visit(typed.Argument())
			visit(typed.Result())
		case TupleType:
			for _, element := range typed.Elements() {
				visit(element)
			}
		case SparseTupleType:
			for _, field := range typed.Fields() {
				visit(field)
			}
		case RecordType:
			for _, field := range typed.Fields() {
				visit(field)
			}
		case OperatorType:
			for _, argument := range typed.Arguments() {
				visit(argument)
			}
			visit(typed.Result())
		case RowType:
			for _, field := range typed.Fields() {
				visit(field)
			}
		case RecordRowType:
			visit(typed.Row())
		case VariantType:
			visit(typed.Row())
		}
	}
	visit(typ)
	return result
}

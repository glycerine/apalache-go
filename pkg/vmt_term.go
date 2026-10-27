package apalache

import (
	"fmt"
	"math/big"
)

type VMTSortKind uint8

const (
	VMTBoolSort VMTSortKind = iota
	VMTIntSort
	VMTUninterpretedSort
	VMTFunctionSort
)

// VMTSort is the first-order sort fragment accepted by the Scala ReTLA VMT
// backend. Function sorts store their argument sorts in Domain and result in
// Range.
type VMTSort struct {
	Kind   VMTSortKind
	Name   string
	Domain []VMTSort
	Range  *VMTSort
}

func BoolVMTSort() VMTSort { return VMTSort{Kind: VMTBoolSort} }
func IntVMTSort() VMTSort  { return VMTSort{Kind: VMTIntSort} }
func UninterpretedVMTSort(name string) (VMTSort, error) {
	if name == "" {
		return VMTSort{}, fmt.Errorf("VMT uninterpreted sort name cannot be empty")
	}
	return VMTSort{Kind: VMTUninterpretedSort, Name: name}, nil
}
func FunctionVMTSort(result VMTSort, arguments ...VMTSort) VMTSort {
	copyResult := result
	return VMTSort{Kind: VMTFunctionSort, Domain: append([]VMTSort(nil), arguments...), Range: &copyResult}
}
func (sort VMTSort) Equal(other VMTSort) bool {
	if sort.Kind != other.Kind || sort.Name != other.Name || len(sort.Domain) != len(other.Domain) {
		return false
	}
	if (sort.Range == nil) != (other.Range == nil) {
		return false
	}
	if sort.Range != nil && !sort.Range.Equal(*other.Range) {
		return false
	}
	for index := range sort.Domain {
		if !sort.Domain[index].Equal(other.Domain[index]) {
			return false
		}
	}
	return true
}

func VMTSortFromType(typ Type) (VMTSort, error) {
	switch typed := typ.(type) {
	case primitiveType:
		switch typed {
		case BoolT1:
			return BoolVMTSort(), nil
		case IntT1:
			return IntVMTSort(), nil
		case StrT1:
			return UninterpretedVMTSort(StrT1.String())
		}
	case ConstType:
		return UninterpretedVMTSort(typed.Name())
	case FunType:
		result, err := VMTSortFromType(typed.Result())
		if err != nil {
			return VMTSort{}, err
		}
		arguments := []Type{typed.Argument()}
		if tuple, ok := typed.Argument().(TupleType); ok {
			arguments = tuple.Elements()
		}
		domain := make([]VMTSort, len(arguments))
		for index, argument := range arguments {
			domain[index], err = VMTSortFromType(argument)
			if err != nil {
				return VMTSort{}, err
			}
		}
		return FunctionVMTSort(result, domain...), nil
	}
	return VMTSort{}, fmt.Errorf("type %s is not permitted in VMT", typ)
}

type VMTTermKind uint8

const (
	VMTVariable VMTTermKind = iota
	VMTIntegerLiteral
	VMTBooleanLiteral
	VMTUninterpretedLiteral
	VMTAnd
	VMTOr
	VMTNot
	VMTImplies
	VMTEquiv
	VMTForall
	VMTExists
	VMTEqual
	VMTApply
	VMTIfThenElse
)

type VMTBinding struct {
	Name string
	Sort VMTSort
}

// VMTFunctionDefinition is an SMT-LIB define-fun introduced while lowering a
// ReTLA function constructor or EXCEPT expression.
type VMTFunctionDefinition struct {
	Name      string
	Arguments []VMTBinding
	Body      VMTTerm
}

// VMTDeclarations contains the auxiliary declarations discovered while an
// expression is lowered. The maps are keyed by source-level names, matching
// the declaration state maintained by the Scala backend.
type VMTDeclarations struct {
	DefinedFunctions      map[string]VMTFunctionDefinition
	DefinitionOrder       []string
	UninterpretedSorts    map[string]VMTSort
	UninterpretedLiterals map[string]VMTTerm
}

func NewVMTDeclarations() VMTDeclarations {
	return VMTDeclarations{
		DefinedFunctions:      make(map[string]VMTFunctionDefinition),
		UninterpretedSorts:    make(map[string]VMTSort),
		UninterpretedLiterals: make(map[string]VMTTerm),
	}
}

func (declarations VMTDeclarations) Clone() VMTDeclarations {
	clone := NewVMTDeclarations()
	for name, definition := range declarations.DefinedFunctions {
		definition.Arguments = append([]VMTBinding(nil), definition.Arguments...)
		clone.DefinedFunctions[name] = definition
	}
	clone.DefinitionOrder = append([]string(nil), declarations.DefinitionOrder...)
	for name, sort := range declarations.UninterpretedSorts {
		clone.UninterpretedSorts[name] = sort
	}
	for name, literal := range declarations.UninterpretedLiterals {
		clone.UninterpretedLiterals[name] = literal
	}
	return clone
}

type VMTTerm struct {
	Kind     VMTTermKind
	Sort     VMTSort
	Name     string
	Integer  *big.Int
	Boolean  bool
	Args     []VMTTerm
	Bindings []VMTBinding
}

func VMTVar(name string, sort VMTSort) (VMTTerm, error) {
	if name == "" {
		return VMTTerm{}, fmt.Errorf("VMT variable name cannot be empty")
	}
	return VMTTerm{Kind: VMTVariable, Name: name, Sort: sort}, nil
}
func VMTInt(value *big.Int) (VMTTerm, error) {
	if value == nil {
		return VMTTerm{}, fmt.Errorf("VMT integer cannot be nil")
	}
	return VMTTerm{Kind: VMTIntegerLiteral, Integer: new(big.Int).Set(value), Sort: IntVMTSort()}, nil
}
func VMTBool(value bool) VMTTerm {
	return VMTTerm{Kind: VMTBooleanLiteral, Boolean: value, Sort: BoolVMTSort()}
}
func VMTLiteral(name string, sort VMTSort) (VMTTerm, error) {
	if sort.Kind != VMTUninterpretedSort {
		return VMTTerm{}, fmt.Errorf("VMT literal %s requires an uninterpreted sort", name)
	}
	if name == "" {
		return VMTTerm{}, fmt.Errorf("VMT literal name cannot be empty")
	}
	return VMTTerm{Kind: VMTUninterpretedLiteral, Name: name, Sort: sort}, nil
}

func vmtBooleanNary(kind VMTTermKind, terms ...VMTTerm) (VMTTerm, error) {
	for index, term := range terms {
		if !term.Sort.Equal(BoolVMTSort()) {
			return VMTTerm{}, fmt.Errorf("Boolean argument %d has non-Boolean sort", index+1)
		}
	}
	return VMTTerm{Kind: kind, Sort: BoolVMTSort(), Args: append([]VMTTerm(nil), terms...)}, nil
}
func VMTAndTerm(terms ...VMTTerm) (VMTTerm, error) { return vmtBooleanNary(VMTAnd, terms...) }
func VMTOrTerm(terms ...VMTTerm) (VMTTerm, error)  { return vmtBooleanNary(VMTOr, terms...) }
func VMTNotTerm(term VMTTerm) (VMTTerm, error) {
	if !term.Sort.Equal(BoolVMTSort()) {
		return VMTTerm{}, fmt.Errorf("negation requires a Boolean term")
	}
	return VMTTerm{Kind: VMTNot, Sort: BoolVMTSort(), Args: []VMTTerm{term}}, nil
}
func vmtBooleanBinary(kind VMTTermKind, left, right VMTTerm) (VMTTerm, error) {
	if !left.Sort.Equal(BoolVMTSort()) || !right.Sort.Equal(BoolVMTSort()) {
		return VMTTerm{}, fmt.Errorf("Boolean connective requires Boolean terms")
	}
	return VMTTerm{Kind: kind, Sort: BoolVMTSort(), Args: []VMTTerm{left, right}}, nil
}
func VMTImpliesTerm(left, right VMTTerm) (VMTTerm, error) {
	return vmtBooleanBinary(VMTImplies, left, right)
}
func VMTEquivTerm(left, right VMTTerm) (VMTTerm, error) {
	return vmtBooleanBinary(VMTEquiv, left, right)
}
func VMTEqualTerm(left, right VMTTerm) (VMTTerm, error) {
	if !left.Sort.Equal(right.Sort) {
		return VMTTerm{}, fmt.Errorf("equality requires terms of equal sorts")
	}
	return VMTTerm{Kind: VMTEqual, Sort: BoolVMTSort(), Args: []VMTTerm{left, right}}, nil
}
func VMTApplyTerm(function VMTTerm, arguments ...VMTTerm) (VMTTerm, error) {
	if function.Sort.Kind != VMTFunctionSort || function.Sort.Range == nil {
		return VMTTerm{}, fmt.Errorf("VMT application requires a function term")
	}
	if len(arguments) != len(function.Sort.Domain) {
		return VMTTerm{}, fmt.Errorf("VMT function expects %d arguments, found %d", len(function.Sort.Domain), len(arguments))
	}
	for index, argument := range arguments {
		if !argument.Sort.Equal(function.Sort.Domain[index]) {
			return VMTTerm{}, fmt.Errorf("VMT function argument %d has the wrong sort", index+1)
		}
	}
	args := append([]VMTTerm{function}, arguments...)
	return VMTTerm{Kind: VMTApply, Sort: *function.Sort.Range, Args: args}, nil
}
func VMTIfThenElseTerm(condition, thenTerm, elseTerm VMTTerm) (VMTTerm, error) {
	if !condition.Sort.Equal(BoolVMTSort()) {
		return VMTTerm{}, fmt.Errorf("VMT conditional requires a Boolean condition")
	}
	if !thenTerm.Sort.Equal(elseTerm.Sort) {
		return VMTTerm{}, fmt.Errorf("VMT conditional branches have different sorts")
	}
	return VMTTerm{Kind: VMTIfThenElse, Sort: thenTerm.Sort, Args: []VMTTerm{condition, thenTerm, elseTerm}}, nil
}
func vmtQuantifier(kind VMTTermKind, bindings []VMTBinding, predicate VMTTerm) (VMTTerm, error) {
	if !predicate.Sort.Equal(BoolVMTSort()) {
		return VMTTerm{}, fmt.Errorf("VMT quantifier predicate must be Boolean")
	}
	for _, binding := range bindings {
		if binding.Name == "" || binding.Sort.Kind == VMTFunctionSort {
			return VMTTerm{}, fmt.Errorf("invalid VMT quantified binding %q", binding.Name)
		}
	}
	return VMTTerm{Kind: kind, Sort: BoolVMTSort(), Bindings: append([]VMTBinding(nil), bindings...), Args: []VMTTerm{predicate}}, nil
}
func VMTForallTerm(bindings []VMTBinding, predicate VMTTerm) (VMTTerm, error) {
	return vmtQuantifier(VMTForall, bindings, predicate)
}
func VMTExistsTerm(bindings []VMTBinding, predicate VMTTerm) (VMTTerm, error) {
	return vmtQuantifier(VMTExists, bindings, predicate)
}

func VMTPrimeName(name string) string { return name + "^" }
func VMTNextName(name string) string  { return "." + name }

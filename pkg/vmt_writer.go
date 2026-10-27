package apalache

import (
	"fmt"
	"sort"
	"strings"
)

func vmtAtomicSort(sort VMTSort) (string, error) {
	switch sort.Kind {
	case VMTIntSort:
		return "Int", nil
	case VMTBoolSort:
		return "Bool", nil
	case VMTUninterpretedSort:
		if sort.Name == "" {
			return "", fmt.Errorf("empty VMT uninterpreted sort")
		}
		return sort.Name, nil
	default:
		return "", fmt.Errorf("sort of quantified variable cannot be %v", sort.Kind)
	}
}

func vmtFunctionSignature(sort VMTSort) ([]string, string, error) {
	if sort.Kind != VMTFunctionSort {
		result, err := vmtAtomicSort(sort)
		return nil, result, err
	}
	if sort.Range == nil {
		return nil, "", fmt.Errorf("VMT function sort has no range")
	}
	domain := make([]string, len(sort.Domain))
	for index, argument := range sort.Domain {
		var err error
		domain[index], err = vmtAtomicSort(argument)
		if err != nil {
			return nil, "", err
		}
	}
	result, err := vmtAtomicSort(*sort.Range)
	return domain, result, err
}

func FormatVMTTerm(term VMTTerm) (string, error) {
	format := func(value VMTTerm) (string, error) { return FormatVMTTerm(value) }
	formatArgs := func(arguments []VMTTerm) ([]string, error) {
		result := make([]string, len(arguments))
		for index, argument := range arguments {
			var err error
			result[index], err = format(argument)
			if err != nil {
				return nil, err
			}
		}
		return result, nil
	}
	switch term.Kind {
	case VMTVariable:
		return term.Name, nil
	case VMTIntegerLiteral:
		if term.Integer == nil {
			return "", fmt.Errorf("nil VMT integer literal")
		}
		return term.Integer.String(), nil
	case VMTBooleanLiteral:
		if term.Boolean {
			return "true", nil
		}
		return "false", nil
	case VMTUninterpretedLiteral:
		if term.Sort.Kind != VMTUninterpretedSort {
			return "", fmt.Errorf("VMT literal has non-uninterpreted sort")
		}
		return term.Name + "_" + term.Sort.Name, nil
	case VMTAnd, VMTOr:
		arguments, err := formatArgs(term.Args)
		if err != nil {
			return "", err
		}
		if len(arguments) == 0 {
			if term.Kind == VMTAnd {
				return "true", nil
			}
			return "false", nil
		}
		if len(arguments) == 1 {
			return arguments[0], nil
		}
		head := "and"
		if term.Kind == VMTOr {
			head = "or"
		}
		return "(" + head + " " + strings.Join(arguments, " ") + ")", nil
	case VMTNot:
		return formatVMTUnary("not", term.Args, format)
	case VMTImplies:
		return formatVMTBinary("=>", term.Args, format)
	case VMTEquiv, VMTEqual:
		return formatVMTBinary("=", term.Args, format)
	case VMTApply:
		arguments, err := formatArgs(term.Args)
		if err != nil {
			return "", err
		}
		if len(arguments) == 0 {
			return "", fmt.Errorf("empty VMT function application")
		}
		return "(" + strings.Join(arguments, " ") + ")", nil
	case VMTIfThenElse:
		arguments, err := formatArgs(term.Args)
		if err != nil {
			return "", err
		}
		if len(arguments) != 3 {
			return "", fmt.Errorf("VMT conditional has %d arguments", len(arguments))
		}
		return "(ite " + strings.Join(arguments, " ") + ")", nil
	case VMTForall, VMTExists:
		if len(term.Args) != 1 {
			return "", fmt.Errorf("VMT quantifier has %d predicates", len(term.Args))
		}
		bindings := make([]string, len(term.Bindings))
		for index, binding := range term.Bindings {
			sortText, err := vmtAtomicSort(binding.Sort)
			if err != nil {
				return "", err
			}
			bindings[index] = "(" + binding.Name + " " + sortText + ")"
		}
		predicate, err := format(term.Args[0])
		if err != nil {
			return "", err
		}
		head := "forall"
		if term.Kind == VMTExists {
			head = "exists"
		}
		return "(" + head + " (" + strings.Join(bindings, " ") + ") " + predicate + ")", nil
	default:
		return "", fmt.Errorf("unsupported VMT term kind %d", term.Kind)
	}
}

func formatVMTUnary(name string, arguments []VMTTerm, format func(VMTTerm) (string, error)) (string, error) {
	if len(arguments) != 1 {
		return "", fmt.Errorf("VMT %s has %d arguments", name, len(arguments))
	}
	argument, err := format(arguments[0])
	if err != nil {
		return "", err
	}
	return "(" + name + " " + argument + ")", nil
}

func formatVMTBinary(name string, arguments []VMTTerm, format func(VMTTerm) (string, error)) (string, error) {
	if len(arguments) != 2 {
		return "", fmt.Errorf("VMT %s has %d arguments", name, len(arguments))
	}
	left, err := format(arguments[0])
	if err != nil {
		return "", err
	}
	right, err := format(arguments[1])
	if err != nil {
		return "", err
	}
	return "(" + name + " " + left + " " + right + ")", nil
}

func FormatVMTVariableDeclaration(name string, sort VMTSort) (string, error) {
	domain, result, err := vmtFunctionSignature(sort)
	if err != nil {
		return "", err
	}
	declaration := func(variable string) string {
		return "(declare-fun " + variable + " (" + strings.Join(domain, " ") + ") " + result + ")"
	}
	return declaration(name) + "\n" + declaration(VMTPrimeName(name)), nil
}

func FormatVMTSortDeclaration(sort VMTSort) (string, error) {
	if sort.Kind != VMTUninterpretedSort || sort.Name == "" {
		return "", fmt.Errorf("VMT sort declaration requires a named uninterpreted sort")
	}
	return "(declare-sort " + sort.Name + " 0)", nil
}

func FormatVMTLiteralDeclaration(literal VMTTerm) (string, error) {
	if literal.Kind != VMTUninterpretedLiteral || literal.Sort.Kind != VMTUninterpretedSort {
		return "", fmt.Errorf("VMT constant declaration requires an uninterpreted literal")
	}
	term, err := FormatVMTTerm(literal)
	if err != nil {
		return "", err
	}
	base := "(declare-fun " + term + " () " + literal.Sort.Name + ")"
	global := "(define-fun " + VMTNextName(term) + " () " + literal.Sort.Name + " (! " + term + " :global true))"
	return base + "\n" + global, nil
}

func FormatVMTDistinct(literals []VMTTerm) (string, error) {
	terms := make([]string, len(literals))
	for index, literal := range literals {
		var err error
		terms[index], err = FormatVMTTerm(literal)
		if err != nil {
			return "", err
		}
	}
	sort.Strings(terms)
	return "(define-fun .axiom () Bool (! (distinct " + strings.Join(terms, " ") + ") :axiom true))", nil
}

type VMTAnnotationKind uint8

const (
	VMTNext VMTAnnotationKind = iota
	VMTInit
	VMTTransition
	VMTInvariant
)

type VMTAnnotatedExpression struct {
	Kind    VMTAnnotationKind
	Name    string
	Index   int
	Current VMTTerm
	Next    VMTTerm
	Term    VMTTerm
}

func FormatVMTAnnotated(expression VMTAnnotatedExpression) (string, error) {
	switch expression.Kind {
	case VMTNext:
		if expression.Current.Kind != VMTVariable || expression.Next.Kind != VMTVariable ||
			!expression.Current.Sort.Equal(expression.Next.Sort) {
			return "", fmt.Errorf("VMT next binding requires equal-sort variables")
		}
		domain, result, err := vmtFunctionSignature(expression.Current.Sort)
		if err != nil {
			return "", err
		}
		parameters := make([]string, len(domain))
		arguments := make([]string, len(domain))
		for index, sortText := range domain {
			arguments[index] = fmt.Sprintf("_p%d@", index)
			parameters[index] = "(" + arguments[index] + " " + sortText + ")"
		}
		current := expression.Current.Name
		if len(arguments) > 0 {
			current = "(" + current + " " + strings.Join(arguments, " ") + ")"
		}
		return "(define-fun " + expression.Name + " (" + strings.Join(parameters, " ") + ") " + result +
			" (! " + current + " :next " + expression.Next.Name + "))", nil
	case VMTInit, VMTTransition, VMTInvariant:
		if !expression.Term.Sort.Equal(BoolVMTSort()) {
			return "", fmt.Errorf("VMT annotated predicate must be Boolean")
		}
		term, err := FormatVMTTerm(expression.Term)
		if err != nil {
			return "", err
		}
		annotation := ":init true"
		if expression.Kind == VMTTransition {
			annotation = ":action " + expression.Name
		} else if expression.Kind == VMTInvariant {
			annotation = fmt.Sprintf(":invar-property %d", expression.Index)
		}
		return "(define-fun " + expression.Name + " () Bool (! " + term + " " + annotation + "))", nil
	default:
		return "", fmt.Errorf("unsupported VMT annotation kind %d", expression.Kind)
	}
}

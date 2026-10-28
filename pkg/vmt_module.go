package apalache

import (
	"fmt"
	"sort"
	"strings"
)

type namedVMTExpression struct {
	name string
	body Expr
}

func vmtOperatorsWithPrefix(module *Module, prefixes ...string) []namedVMTExpression {
	result := make([]namedVMTExpression, 0)
	for _, declaration := range module.OperDeclarations() {
		if len(declaration.FormalParams()) != 0 {
			continue
		}
		for _, prefix := range prefixes {
			if strings.HasPrefix(declaration.Name(), prefix) {
				result = append(result, namedVMTExpression{name: declaration.Name(), body: declaration.Body()})
				break
			}
		}
	}
	sort.Slice(result, func(left, right int) bool { return result[left].name < result[right].name })
	return result
}

func vmtConstantSets(module *Module) (map[string]VMTSort, error) {
	sets := make(map[string]VMTSort)
	for _, declaration := range module.ConstDeclarations() {
		typ, typed := declaration.Tag().Type()
		if !typed {
			continue
		}
		set, ok := typ.(SetType)
		if !ok {
			continue
		}
		switch element := set.Element().(type) {
		case ConstType:
			sort, err := UninterpretedVMTSort(element.Name())
			if err != nil {
				return nil, err
			}
			sets[declaration.Name()] = sort
		case primitiveType:
			if element == StrT1 {
				sort, _ := UninterpretedVMTSort(StrT1.String())
				sets[declaration.Name()] = sort
			}
		}
	}
	return sets, nil
}

func vmtDeprime(expression Expr) Expr {
	return RewriteExpressions(IdleTracker{}, func(expression Expr) (Expr, bool) {
		operation, ok := expression.(*OperExpr)
		if !ok || operation.Operator().Name() != "PRIME" {
			return nil, false
		}
		return operation.Arguments()[0], true
	})(expression)
}

func addVMTSorts(target map[string]VMTSort, value VMTSort) {
	if value.Kind == VMTUninterpretedSort {
		target[value.Name] = value
	}
	for _, domain := range value.Domain {
		addVMTSorts(target, domain)
	}
	if value.Range != nil {
		addVMTSorts(target, *value.Range)
	}
}

// FormatVMTModule assembles a transition-disassembled ReTLA module into a
// deterministic, standalone VMT-LIB document. Constant initialization is
// intentionally omitted, matching the Scala experimental transpiler.
func FormatVMTModule(module *Module) (string, error) {
	return FormatVMTModuleWithInvariants(module, nil)
}

// FormatVMTModuleWithInvariants supplies the configured invariant names used
// by the Go safety pipeline when a separate VC-generation pass has not renamed
// them to VCInv$*. Existing normalized verification conditions take priority.
func FormatVMTModuleWithInvariants(module *Module, configuredInvariants []string) (string, error) {
	if module == nil {
		return "", fmt.Errorf("cannot format a nil VMT module")
	}
	constantSets, err := vmtConstantSets(module)
	if err != nil {
		return "", err
	}
	rewriter, err := NewVMTRewriter(constantSets)
	if err != nil {
		return "", err
	}

	formatPredicates := func(expressions []namedVMTExpression, kind VMTAnnotationKind, indexOffset int) ([]string, error) {
		formatted := make([]string, len(expressions))
		for index, expression := range expressions {
			body := expression.body
			if languageErr := WatchExpression(ReTLALanguagePredicate{}, body); languageErr != nil {
				return nil, fmt.Errorf("checking reTLA language of %s: %w", expression.name, languageErr)
			}
			if kind == VMTInit {
				body = vmtDeprime(body)
			}
			term, rewriteErr := rewriter.Rewrite(body)
			if rewriteErr != nil {
				return nil, fmt.Errorf("lowering %s: %w", expression.name, rewriteErr)
			}
			formatted[index], rewriteErr = FormatVMTAnnotated(VMTAnnotatedExpression{
				Kind: kind, Name: expression.name, Index: indexOffset + index, Term: term,
			})
			if rewriteErr != nil {
				return nil, fmt.Errorf("formatting %s: %w", expression.name, rewriteErr)
			}
		}
		return formatted, nil
	}

	inits, err := formatPredicates(vmtOperatorsWithPrefix(module, NormalizedInitPrefix), VMTInit, 0)
	if err != nil {
		return "", err
	}
	transitions, err := formatPredicates(vmtOperatorsWithPrefix(module, NormalizedNextPrefix), VMTTransition, 0)
	if err != nil {
		return "", err
	}
	stateInvariants := vmtOperatorsWithPrefix(module, NormalizedVCInvariantPrefix)
	actionInvariants := vmtOperatorsWithPrefix(module, NormalizedVCActionPrefix)
	invariantExpressions := append(stateInvariants, actionInvariants...)
	if len(invariantExpressions) == 0 {
		operators := make(map[string]*OperDecl)
		for _, declaration := range module.OperDeclarations() {
			operators[declaration.Name()] = declaration
		}
		for index, name := range configuredInvariants {
			declaration, found := operators[name]
			if !found {
				return "", fmt.Errorf("configured VMT invariant %s was not found", name)
			}
			invariantExpressions = append(invariantExpressions, namedVMTExpression{
				name: fmt.Sprintf("%s%d", NormalizedVCInvariantPrefix, index), body: declaration.Body(),
			})
		}
	}
	invariants, err := formatPredicates(invariantExpressions, VMTInvariant, 0)
	if err != nil {
		return "", err
	}

	variables := append([]*VarDecl(nil), module.VarDeclarations()...)
	sort.Slice(variables, func(left, right int) bool { return variables[left].Name() < variables[right].Name() })
	variableDeclarations := make([]string, len(variables))
	nextBindings := make([]string, len(variables))
	variableSorts := make([]VMTSort, len(variables))
	for index, declaration := range variables {
		typ, typed := declaration.Tag().Type()
		if !typed {
			return "", fmt.Errorf("VMT variable %s is untyped", declaration.Name())
		}
		variableSorts[index], err = VMTSortFromType(typ)
		if err != nil {
			return "", fmt.Errorf("VMT variable %s: %w", declaration.Name(), err)
		}
		variableDeclarations[index], err = FormatVMTVariableDeclaration(declaration.Name(), variableSorts[index])
		if err != nil {
			return "", err
		}
		current, _ := VMTVar(declaration.Name(), variableSorts[index])
		next, _ := VMTVar(VMTPrimeName(declaration.Name()), variableSorts[index])
		nextBindings[index], err = FormatVMTAnnotated(VMTAnnotatedExpression{
			Kind: VMTNext, Name: VMTNextName(declaration.Name()), Current: current, Next: next,
		})
		if err != nil {
			return "", err
		}
	}

	declarations := rewriter.Declarations()
	allSorts := make(map[string]VMTSort)
	for _, value := range constantSets {
		addVMTSorts(allSorts, value)
	}
	for _, value := range declarations.UninterpretedSorts {
		addVMTSorts(allSorts, value)
	}
	for _, value := range variableSorts {
		addVMTSorts(allSorts, value)
	}
	sortNames := make([]string, 0, len(allSorts))
	for name := range allSorts {
		sortNames = append(sortNames, name)
	}
	sort.Strings(sortNames)
	sortDeclarations := make([]string, len(sortNames))
	for index, name := range sortNames {
		sortDeclarations[index], err = FormatVMTSortDeclaration(allSorts[name])
		if err != nil {
			return "", err
		}
	}

	literalNames := make([]string, 0, len(declarations.UninterpretedLiterals))
	for name := range declarations.UninterpretedLiterals {
		literalNames = append(literalNames, name)
	}
	sort.Strings(literalNames)
	literalDeclarations := make([]string, len(literalNames))
	literalsBySort := make(map[string][]VMTTerm)
	for index, name := range literalNames {
		literal := declarations.UninterpretedLiterals[name]
		literalDeclarations[index], err = FormatVMTLiteralDeclaration(literal)
		if err != nil {
			return "", err
		}
		literalsBySort[literal.Sort.Name] = append(literalsBySort[literal.Sort.Name], literal)
	}
	distinctSorts := make([]string, 0)
	for _, name := range sortNames {
		if len(literalsBySort[name]) > 1 {
			distinctSorts = append(distinctSorts, name)
		}
	}
	distinctAssertions := make([]string, len(distinctSorts))
	for index, name := range distinctSorts {
		axiomName := ".axiom"
		if len(distinctSorts) > 1 {
			axiomName = fmt.Sprintf(".axiom.%d", index)
		}
		assertion, formatErr := FormatVMTDistinctNamed(axiomName, literalsBySort[name])
		if formatErr != nil {
			return "", formatErr
		}
		distinctAssertions[index] = assertion
	}

	definitionNames := append([]string(nil), declarations.DefinitionOrder...)
	if len(definitionNames) != len(declarations.DefinedFunctions) {
		// Be robust to declarations assembled by callers rather than VMTRewriter.
		seen := make(map[string]struct{}, len(definitionNames))
		for _, name := range definitionNames {
			seen[name] = struct{}{}
		}
		remaining := make([]string, 0)
		for name := range declarations.DefinedFunctions {
			if _, found := seen[name]; !found {
				remaining = append(remaining, name)
			}
		}
		sort.Strings(remaining)
		definitionNames = append(definitionNames, remaining...)
	}
	functionDefinitions := make([]string, len(definitionNames))
	for index, name := range definitionNames {
		functionDefinitions[index], err = FormatVMTFunctionDefinition(declarations.DefinedFunctions[name])
		if err != nil {
			return "", err
		}
	}

	var output strings.Builder
	writeSection := func(title string, lines []string) {
		output.WriteString(";" + title + "\n")
		for _, line := range lines {
			output.WriteString(line)
			output.WriteByte('\n')
		}
		output.WriteByte('\n')
	}
	writeSection("Sorts", sortDeclarations)
	writeSection("Constants", literalDeclarations)
	for _, assertion := range distinctAssertions {
		output.WriteString(assertion + "\n")
	}
	output.WriteByte('\n')
	writeSection("Variables", variableDeclarations)
	writeSection("Function definitions", functionDefinitions)
	writeSection("Variable bindings", nextBindings)
	writeSection("Initial states", inits)
	writeSection("Transitions", transitions)
	writeSection("Invariants", invariants)
	return output.String(), nil
}

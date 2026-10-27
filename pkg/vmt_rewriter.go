package apalache

import "fmt"

// VMTRewriter lowers the expression fragment accepted by the Scala ReTLA VMT
// backend into its first-order term representation.
type VMTRewriter struct {
	constantSets map[string]VMTSort
	names        *UniqueNameGenerator
	declarations VMTDeclarations
}

func NewVMTRewriter(constantSets map[string]VMTSort) (*VMTRewriter, error) {
	sets := make(map[string]VMTSort, len(constantSets))
	for name, sort := range constantSets {
		if name == "" || sort.Kind != VMTUninterpretedSort {
			return nil, fmt.Errorf("restricted constant set %q must have an uninterpreted element sort", name)
		}
		sets[name] = sort
	}
	return &VMTRewriter{constantSets: sets, names: NewUniqueNameGenerator(), declarations: NewVMTDeclarations()}, nil
}

func (rewriter *VMTRewriter) Declarations() VMTDeclarations {
	return rewriter.declarations.Clone()
}

func (rewriter *VMTRewriter) rememberSort(sort VMTSort) {
	if sort.Kind == VMTUninterpretedSort {
		rewriter.declarations.UninterpretedSorts[sort.Name] = sort
	}
}

func (rewriter *VMTRewriter) rememberTerm(term VMTTerm) {
	if term.Kind == VMTUninterpretedLiteral {
		rewriter.rememberSort(term.Sort)
		// The same source-level literal index may occur in multiple model-value
		// sorts (for example a_OF_A and a_OF_B). Its emitted SMT symbol includes
		// the sort, so retain both declarations under that full identity.
		rewriter.declarations.UninterpretedLiterals[term.Name+"_"+term.Sort.Name] = term
	} else if term.Kind == VMTVariable {
		rewriter.rememberSort(term.Sort)
	}
}

func typeOfVMTExpression(expression Expr) (Type, error) {
	typ, ok := expression.Tag().Type()
	if !ok {
		return nil, fmt.Errorf("VMT lowering requires a type tag on %T", expression)
	}
	return typ, nil
}

func (rewriter *VMTRewriter) variable(name string, expression Expr) (VMTTerm, error) {
	typ, err := typeOfVMTExpression(expression)
	if err != nil {
		return VMTTerm{}, err
	}
	sort, err := VMTSortFromType(typ)
	if err != nil {
		return VMTTerm{}, err
	}
	term, err := VMTVar(name, sort)
	if err == nil {
		rewriter.rememberTerm(term)
	}
	return term, err
}

func (rewriter *VMTRewriter) restrictedSetSort(expression Expr) (VMTSort, error) {
	switch typed := expression.(type) {
	case *ValueExpr:
		if set, ok := typed.Value().(PredefinedSet); ok {
			switch set {
			case IntSet, NatSet:
				return IntVMTSort(), nil
			case BoolSet:
				return BoolVMTSort(), nil
			}
		}
	case *NameExpr:
		if sort, ok := rewriter.constantSets[typed.Name()]; ok {
			return sort, nil
		}
	}
	return VMTSort{}, fmt.Errorf("%T is not a restricted ReTLA set", expression)
}

func (rewriter *VMTRewriter) rewriteAll(expressions []Expr) ([]VMTTerm, error) {
	terms := make([]VMTTerm, len(expressions))
	for index, expression := range expressions {
		var err error
		terms[index], err = rewriter.Rewrite(expression)
		if err != nil {
			return nil, err
		}
	}
	return terms, nil
}

func (rewriter *VMTRewriter) define(arguments []VMTBinding, body VMTTerm) (VMTTerm, error) {
	name := rewriter.names.NewName()
	definition := VMTFunctionDefinition{Name: name, Arguments: append([]VMTBinding(nil), arguments...), Body: body}
	rewriter.declarations.DefinedFunctions[name] = definition
	domain := make([]VMTSort, len(arguments))
	for index, argument := range arguments {
		domain[index] = argument.Sort
	}
	return VMTVar(name, FunctionVMTSort(body.Sort, domain...))
}

func (rewriter *VMTRewriter) rewriteEquality(leftExpression, rightExpression Expr) (VMTTerm, error) {
	left, err := rewriter.Rewrite(leftExpression)
	if err != nil {
		return VMTTerm{}, err
	}
	right, err := rewriter.Rewrite(rightExpression)
	if err != nil {
		return VMTTerm{}, err
	}
	if !left.Sort.Equal(right.Sort) {
		return VMTTerm{}, fmt.Errorf("VMT equality requires terms of equal sorts")
	}
	if left.Sort.Kind != VMTFunctionSort {
		return VMTEqualTerm(left, right)
	}
	bindings := make([]VMTBinding, len(left.Sort.Domain))
	arguments := make([]VMTTerm, len(bindings))
	for index, sort := range left.Sort.Domain {
		bindings[index] = VMTBinding{Name: rewriter.names.NewName(), Sort: sort}
		arguments[index], err = VMTVar(bindings[index].Name, sort)
		if err != nil {
			return VMTTerm{}, err
		}
	}
	leftApp, err := VMTApplyTerm(left, arguments...)
	if err != nil {
		return VMTTerm{}, err
	}
	rightApp, err := VMTApplyTerm(right, arguments...)
	if err != nil {
		return VMTTerm{}, err
	}
	equality, err := VMTEqualTerm(leftApp, rightApp)
	if err != nil {
		return VMTTerm{}, err
	}
	return VMTForallTerm(bindings, equality)
}

func (rewriter *VMTRewriter) Rewrite(expression Expr) (VMTTerm, error) {
	if expression == nil {
		return VMTTerm{}, fmt.Errorf("cannot lower a nil expression to VMT")
	}
	switch typed := expression.(type) {
	case *ValueExpr:
		switch value := typed.Value().(type) {
		case IntValue:
			return VMTInt(value.BigInt())
		case BoolValue:
			return VMTBool(value.Bool()), nil
		case StringValue:
			text := value.Value()
			sort, name := VMTSort{}, text
			if modelType, modelName, ok := ParseModelValue(text); ok {
				sort, _ = VMTSortFromType(modelType)
				name = modelName
			} else {
				sort, _ = VMTSortFromType(StrT1)
			}
			term, err := VMTLiteral(name, sort)
			if err == nil {
				rewriter.rememberTerm(term)
			}
			return term, err
		default:
			return VMTTerm{}, fmt.Errorf("literal %T is not supported in VMT", value)
		}
	case *NameExpr:
		return rewriter.variable(typed.Name(), typed)
	case *OperExpr:
		return rewriter.rewriteOperator(typed)
	default:
		return VMTTerm{}, fmt.Errorf("expression %T is not supported in VMT", expression)
	}
}

func (rewriter *VMTRewriter) rewriteOperator(expression *OperExpr) (VMTTerm, error) {
	arguments := expression.Arguments()
	operator := expression.Operator().Name()
	switch operator {
	case "PRIME":
		if len(arguments) == 1 {
			if name, ok := arguments[0].(*NameExpr); ok {
				return rewriter.variable(VMTPrimeName(name.Name()), name)
			}
		}
		return VMTTerm{}, fmt.Errorf("VMT PRIME requires a name")
	case "AND", "OR":
		terms, err := rewriter.rewriteAll(arguments)
		if err != nil {
			return VMTTerm{}, err
		}
		if operator == "AND" {
			return VMTAndTerm(terms...)
		}
		return VMTOrTerm(terms...)
	case "NOT":
		term, err := rewriter.Rewrite(arguments[0])
		if err != nil {
			return VMTTerm{}, err
		}
		return VMTNotTerm(term)
	case "IMPLIES", "EQUIV":
		terms, err := rewriter.rewriteAll(arguments)
		if err != nil {
			return VMTTerm{}, err
		}
		if operator == "IMPLIES" {
			return VMTImpliesTerm(terms[0], terms[1])
		}
		return VMTEquivTerm(terms[0], terms[1])
	case "EXISTS3", "FORALL3":
		name, ok := arguments[0].(*NameExpr)
		if !ok {
			return VMTTerm{}, fmt.Errorf("VMT quantified binder must be a name")
		}
		sort, err := rewriter.restrictedSetSort(arguments[1])
		if err != nil {
			return VMTTerm{}, err
		}
		predicate, err := rewriter.Rewrite(arguments[2])
		if err != nil {
			return VMTTerm{}, err
		}
		bindings := []VMTBinding{{Name: name.Name(), Sort: sort}}
		if operator == "EXISTS3" {
			return VMTExistsTerm(bindings, predicate)
		}
		return VMTForallTerm(bindings, predicate)
	case "EQ", "Apalache!:=":
		return rewriter.rewriteEquality(arguments[0], arguments[1])
	case "NE":
		equality, err := rewriter.rewriteEquality(arguments[0], arguments[1])
		if err != nil {
			return VMTTerm{}, err
		}
		return VMTNotTerm(equality)
	case "FUN_CTOR":
		bindings := make([]VMTBinding, (len(arguments)-1)/2)
		for index := range bindings {
			name, ok := arguments[1+2*index].(*NameExpr)
			if !ok {
				return VMTTerm{}, fmt.Errorf("VMT function binder must be a name")
			}
			sort, err := rewriter.restrictedSetSort(arguments[2+2*index])
			if err != nil {
				return VMTTerm{}, err
			}
			bindings[index] = VMTBinding{Name: name.Name(), Sort: sort}
		}
		body, err := rewriter.Rewrite(arguments[0])
		if err != nil {
			return VMTTerm{}, err
		}
		return rewriter.define(bindings, body)
	case "FUN_APP":
		function, err := rewriter.Rewrite(arguments[0])
		if err != nil {
			return VMTTerm{}, err
		}
		applicationArguments := []Expr{arguments[1]}
		if tuple, ok := arguments[1].(*OperExpr); ok && tuple.Operator().Name() == "TUPLE" {
			applicationArguments = tuple.Arguments()
		}
		terms, err := rewriter.rewriteAll(applicationArguments)
		if err != nil {
			return VMTTerm{}, err
		}
		return VMTApplyTerm(function, terms...)
	case "EXCEPT":
		return rewriter.rewriteExcept(arguments)
	case "IF_THEN_ELSE":
		terms, err := rewriter.rewriteAll(arguments)
		if err != nil {
			return VMTTerm{}, err
		}
		return VMTIfThenElseTerm(terms[0], terms[1], terms[2])
	default:
		return VMTTerm{}, fmt.Errorf("operator %s is not supported in VMT", operator)
	}
}

func (rewriter *VMTRewriter) rewriteExcept(arguments []Expr) (VMTTerm, error) {
	if len(arguments) != 3 {
		return VMTTerm{}, fmt.Errorf("VMT EXCEPT requires one update")
	}
	outer, ok := arguments[1].(*OperExpr)
	if !ok || outer.Operator().Name() != "TUPLE" || len(outer.Arguments()) != 1 {
		return VMTTerm{}, fmt.Errorf("invalid VMT EXCEPT path")
	}
	path := outer.Arguments()[0]
	pathExpressions := []Expr{path}
	if tuple, ok := path.(*OperExpr); ok && tuple.Operator().Name() == "TUPLE" {
		pathExpressions = tuple.Arguments()
	}
	pathTerms, err := rewriter.rewriteAll(pathExpressions)
	if err != nil {
		return VMTTerm{}, err
	}
	newValue, err := rewriter.Rewrite(arguments[2])
	if err != nil {
		return VMTTerm{}, err
	}
	function, err := rewriter.Rewrite(arguments[0])
	if err != nil {
		return VMTTerm{}, err
	}
	if function.Kind != VMTVariable || function.Sort.Kind != VMTFunctionSort {
		return VMTTerm{}, fmt.Errorf("VMT EXCEPT base must be a function variable")
	}
	if len(pathTerms) != len(function.Sort.Domain) {
		return VMTTerm{}, fmt.Errorf("VMT EXCEPT path has %d arguments for arity %d", len(pathTerms), len(function.Sort.Domain))
	}
	bindings := make([]VMTBinding, len(function.Sort.Domain))
	bindingTerms := make([]VMTTerm, len(bindings))
	matches := make([]VMTTerm, len(bindings))
	for index, sort := range function.Sort.Domain {
		bindings[index] = VMTBinding{Name: rewriter.names.NewName(), Sort: sort}
		bindingTerms[index], _ = VMTVar(bindings[index].Name, sort)
		matches[index], err = VMTEqualTerm(bindingTerms[index], pathTerms[index])
		if err != nil {
			return VMTTerm{}, err
		}
	}
	condition, err := VMTAndTerm(matches...)
	if err != nil {
		return VMTTerm{}, err
	}
	base, err := VMTApplyTerm(function, bindingTerms...)
	if err != nil {
		return VMTTerm{}, err
	}
	body, err := VMTIfThenElseTerm(condition, newValue, base)
	if err != nil {
		return VMTTerm{}, err
	}
	return rewriter.define(bindings, body)
}

package apalache

import (
	"fmt"
	"strings"
)

func FormatUTFExpression(expression Expr, removeSpaces ...bool) (string, error) {
	text, err := formatUTF(expression)
	if err == nil && len(removeSpaces) > 0 && removeSpaces[0] {
		text = strings.ReplaceAll(text, " ", "")
	}
	return text, err
}

func FormatSimpleExpression(expression Expr) (string, error) {
	text, err := FormatUTFExpression(expression)
	if err != nil {
		return "", err
	}
	return strings.NewReplacer(
		"≠", "/=", "∧", "/\\", "∨", "\\/", "¬", "~", "∈", "\\in", "∉", "\\notin",
		"⇒", "=>", "⇔", "<=>", "≤", "<=", "≥", ">=", "∀", "\\A", "∃", "\\E",
		"⋅", ".", "□", "[]", "◇", "<>", "→", "->", "∘", "\\o", "⇸", "-+->", "↝", "~>",
		"↦", "|->", "⟨", "<<", "⟩", ">>", "∩", "\\cap", "∪", "\\cup", "⊂", "\\subset",
		"⊆", "\\subseteq", "⊃", "\\supset", "⊇", "\\supseteq", "∖", "\\", "×", "x",
		"÷", "\\div", "∷", "::",
	).Replace(text), nil
}

func utfArgs(arguments []Expr, separator string, printer func(Expr) (string, error)) (string, error) {
	parts := make([]string, len(arguments))
	for index, argument := range arguments {
		var err error
		parts[index], err = printer(argument)
		if err != nil {
			return "", err
		}
	}
	return strings.Join(parts, separator), nil
}

func utfOpArg(expression Expr) (string, error) {
	text, err := formatUTF(expression)
	if err != nil {
		return "", err
	}
	switch typed := expression.(type) {
	case *LetInExpr:
		return "(" + text + ")", nil
	case *OperExpr:
		name := typed.Operator().Name()
		if name != "SET_ENUM" && name != "SET_IN" && name != "FUN_APP" && len(typed.Arguments()) > 1 {
			return "(" + text + ")", nil
		}
	}
	return text, nil
}

func formatUTF(expression Expr) (string, error) {
	switch typed := expression.(type) {
	case *nullExpr:
		return "<[NULL]>", nil
	case *NameExpr:
		return typed.Name(), nil
	case *ValueExpr:
		return formatUTFValue(typed.Value())
	case *LetInExpr:
		declarations := make([]string, len(typed.Declarations()))
		for index, declaration := range typed.Declarations() {
			var err error
			declarations[index], err = FormatUTFDeclaration(declaration)
			if err != nil {
				return "", err
			}
		}
		body, err := formatUTF(typed.Body())
		return "LET " + strings.Join(declarations, " ") + " IN " + body, err
	case *OperExpr:
		return formatUTFOperator(typed)
	default:
		return "", fmt.Errorf("unsupported expression %T", expression)
	}
}

func formatUTFValue(value Value) (string, error) {
	switch typed := value.(type) {
	case IntValue:
		return typed.String(), nil
	case DecimalValue:
		return finiteDecimalString(typed.Rat())
	case StringValue:
		return fmt.Sprintf("%q", typed.Value()), nil
	case BoolValue:
		if typed.Bool() {
			return "TRUE", nil
		}
		return "FALSE", nil
	case PredefinedSet:
		return typed.Name(), nil
	case RealValue:
		if typed.IsInfinity() {
			return "Infinity", nil
		}
		return "Real", nil
	default:
		return "", fmt.Errorf("unsupported value %T", value)
	}
}

func formatUTFOperator(expression *OperExpr) (string, error) {
	name, arguments := expression.Operator().Name(), expression.Arguments()
	plain := func(index int) (string, error) { return formatUTF(arguments[index]) }
	op := func(index int) (string, error) { return utfOpArg(arguments[index]) }
	joinOp := func(separator string) (string, error) { return utfArgs(arguments, separator, utfOpArg) }
	binary := func(symbol string) (string, error) {
		left, err := op(0)
		if err != nil {
			return "", err
		}
		right, err := op(1)
		return left + " " + symbol + " " + right, err
	}
	if symbol, ok := map[string]string{
		"EQ": "=", "NE": "≠", "IMPLIES": "⇒", "EQUIV": "⇔", "PLUS": "+", "MINUS": "-",
		"MULT": "*", "DIV": "÷", "MOD": "%", "REAL_DIV": "/", "POW": "^", "INT_RANGE": "..",
		"LT": "<", "GT": ">", "LE": "≤", "GE": "≥", "SET_IN": "∈", "SET_NOT_IN": "∉",
		"SET_INTERSECT": "∩", "SET_UNION2": "∪", "SET_MINUS": "∖", "SET_SUBSET_EQ": "⊆",
		"COMPOSE": "⋅", "LEADS_TO": "↝", "GUARANTEES": "⇸", "Sequences!Concat": "∘",
	}[name]; ok {
		return binary(symbol)
	}
	switch name {
	case "OPER_APP":
		callee, err := op(0)
		if err != nil {
			return "", err
		}
		args, err := utfArgs(arguments[1:], ", ", formatUTF)
		return callee + "(" + args + ")", err
	case "CHOOSE2", "FORALL2", "EXISTS2":
		symbol := map[string]string{"CHOOSE2": "CHOOSE ", "FORALL2": "∀", "EXISTS2": "∃"}[name]
		variable, err := op(0)
		if err != nil {
			return "", err
		}
		body, err := op(1)
		return symbol + variable + ": " + body, err
	case "CHOOSE3", "FORALL3", "EXISTS3":
		symbol := map[string]string{"CHOOSE3": "CHOOSE ", "FORALL3": "∀", "EXISTS3": "∃"}[name]
		variable, err := op(0)
		if err != nil {
			return "", err
		}
		set, err := op(1)
		if err != nil {
			return "", err
		}
		body, err := op(2)
		separator := ": "
		if name == "CHOOSE3" {
			separator = " : "
		}
		return symbol + variable + " ∈ " + set + separator + body, err
	case "AND", "OR":
		symbol := " ∧ "
		if name == "OR" {
			symbol = " ∨ "
		}
		return joinOp(symbol)
	case "NOT", "UNARY_MINUS", "PRIME", "ENABLED", "UNCHANGED", "DOMAIN", "SET_POWERSET", "SET_UNARY_UNION", "GLOBALLY", "EVENTUALLY":
		prefix := map[string]string{"NOT": "¬", "UNARY_MINUS": "-", "ENABLED": "ENABLED ", "UNCHANGED": "UNCHANGED ", "DOMAIN": "DOMAIN ", "SET_POWERSET": "SUBSET ", "SET_UNARY_UNION": "UNION ", "GLOBALLY": "□", "EVENTUALLY": "◇"}[name]
		argument, err := op(0)
		if name == "PRIME" {
			return argument + "'", err
		}
		return prefix + argument, err
	case "STUTTER", "NO_STUTTER":
		action, err := op(0)
		if err != nil {
			return "", err
		}
		subscript, err := op(1)
		if name == "STUTTER" {
			return "[" + action + "]_" + subscript, err
		}
		return "<" + action + ">_" + subscript, err
	case "IF_THEN_ELSE":
		condition, err := op(0)
		if err != nil {
			return "", err
		}
		thenText, err := op(1)
		if err != nil {
			return "", err
		}
		elseText, err := op(2)
		return "IF " + condition + " THEN " + thenText + " ELSE " + elseText, err
	case "CASE", "CASE_OTHER":
		start := 0
		other := ""
		if name == "CASE_OTHER" {
			var err error
			other, err = op(0)
			if err != nil {
				return "", err
			}
			start = 1
		}
		pairs := make([]string, 0, (len(arguments)-start)/2)
		for index := start; index < len(arguments); index += 2 {
			condition, err := op(index)
			if err != nil {
				return "", err
			}
			body, err := op(index + 1)
			if err != nil {
				return "", err
			}
			pairs = append(pairs, condition+" → "+body)
		}
		text := "CASE " + strings.Join(pairs, " □ ")
		if name == "CASE_OTHER" {
			text += " □ OTHER → " + other
		}
		return text, nil
	case "TEMPORAL_FORALL", "TEMPORAL_EXISTS":
		symbol := "∀"
		if name == "TEMPORAL_EXISTS" {
			symbol = "∃"
		}
		variable, err := op(0)
		if err != nil {
			return "", err
		}
		body, err := op(1)
		return "[" + symbol + "]" + variable + " . " + body, err
	case "WEAK_FAIRNESS", "STRONG_FAIRNESS":
		prefix := "WF_"
		if name == "STRONG_FAIRNESS" {
			prefix = "SF_"
		}
		subscript, err := plain(0)
		if err != nil {
			return "", err
		}
		action, err := plain(1)
		return prefix + subscript + "(" + action + ")", err
	case "SET_ENUM":
		items, err := utfArgs(arguments, ", ", formatUTF)
		return "{" + items + "}", err
	case "SET_FILTER":
		variable, err := op(0)
		if err != nil {
			return "", err
		}
		set, err := op(1)
		if err != nil {
			return "", err
		}
		predicate, err := op(2)
		return "{" + variable + " ∈ " + set + ": " + predicate + "}", err
	case "SET_MAP", "FUN_CTOR":
		body, err := plain(0)
		if err != nil {
			return "", err
		}
		bindings := make([]string, 0, (len(arguments)-1)/2)
		for index := 1; index < len(arguments); index += 2 {
			variable, err := op(index)
			if err != nil {
				return "", err
			}
			set, err := op(index + 1)
			if err != nil {
				return "", err
			}
			bindings = append(bindings, variable+" ∈ "+set)
		}
		if name == "SET_MAP" {
			return "{" + body + " : " + strings.Join(bindings, ", ") + "}", nil
		}
		return "[" + strings.Join(bindings, ", ") + " ↦ " + body + "]", nil
	case "TUPLE":
		items, err := utfArgs(arguments, ", ", formatUTF)
		return "⟨" + items + "⟩", err
	case "FUN_APP":
		function, err := op(0)
		if err != nil {
			return "", err
		}
		argument, err := plain(1)
		return function + "[" + argument + "]", err
	case "RECORD", "RECORD_SET":
		separator := " ↦ "
		if name == "RECORD_SET" {
			separator = ": "
		}
		pairs := make([]string, 0, len(arguments)/2)
		for index := 0; index < len(arguments); index += 2 {
			key, err := op(index)
			if err != nil {
				return "", err
			}
			value, err := op(index + 1)
			if err != nil {
				return "", err
			}
			pairs = append(pairs, key+separator+value)
		}
		return "[" + strings.Join(pairs, ", ") + "]", nil
	case "FUN_SET":
		domain, err := op(0)
		if err != nil {
			return "", err
		}
		codomain, err := op(1)
		return "[" + domain + " → " + codomain + "]", err
	case "EXCEPT":
		function, err := plain(0)
		if err != nil {
			return "", err
		}
		updates := make([]string, 0, (len(arguments)-1)/2)
		for index := 1; index < len(arguments); index += 2 {
			argument, err := op(index)
			if err != nil {
				return "", err
			}
			replacement, err := op(index + 1)
			if err != nil {
				return "", err
			}
			updates = append(updates, "!["+argument+"] = "+replacement)
		}
		return "[" + function + " EXCEPT " + strings.Join(updates, ", ") + "]", nil
	case "SET_TIMES":
		return joinOp(" × ")
	case "LABEL":
		body, err := plain(0)
		if err != nil {
			return "", err
		}
		label, err := plain(1)
		if err != nil {
			return "", err
		}
		label = strings.Trim(label, "\"")
		if len(arguments) > 2 {
			extra, err := utfArgs(arguments[2:], ", ", formatUTF)
			if err != nil {
				return "", err
			}
			label += "(" + extra + ")"
		}
		return label + "∷ " + body, nil
	case "Sequences!Append", "Sequences!Head", "Sequences!Tail", "Sequences!Len", "Sequences!Seq", "FiniteSets!Cardinality", "FiniteSets!IsFiniteSet":
		short := name[strings.LastIndex(name, "!")+1:]
		items, err := utfArgs(arguments, ", ", formatUTF)
		return short + "(" + items + ")", err
	}
	items, err := utfArgs(arguments, ", ", formatUTF)
	if err != nil {
		return "", err
	}
	if len(arguments) == 0 {
		return name, nil
	}
	return name + "(" + items + ")", nil
}

func FormatUTFDeclaration(declaration Decl) (string, error) {
	switch typed := declaration.(type) {
	case *ConstDecl:
		return "CONSTANT " + typed.Name(), nil
	case *VarDecl:
		return "VARIABLE " + typed.Name(), nil
	case *AssumeDecl:
		body, err := formatUTF(typed.Body())
		if err != nil {
			return "", err
		}
		if name, defined := typed.DefinedName(); defined {
			return "ASSUME " + name + " ≜ " + body, nil
		}
		return "ASSUME " + body, nil
	case *OperDecl:
		parameters := typed.FormalParams()
		formatted := make([]string, len(parameters))
		for index, parameter := range parameters {
			formatted[index] = parameter.Name + formatTLAArity(parameter.Arity)
		}
		name := typed.Name()
		if len(formatted) > 0 {
			name += "(" + strings.Join(formatted, ", ") + ")"
		}
		body, err := formatUTF(typed.Body())
		return name + " ≜ " + body, err
	default:
		return "", fmt.Errorf("unexpected declaration %T", declaration)
	}
}

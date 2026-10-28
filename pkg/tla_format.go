package apalache

import (
	"fmt"
	"regexp"
	"strings"
)

type TLAFormatOptions struct {
	ExtendedModules []string
	Annotations     *AnnotationStore
}

var invalidTLAIdentifierPart = regexp.MustCompile(`::|[^a-zA-Z0-9_]`)
var validTLAIdentifierPrefix = regexp.MustCompile(`^_*[a-zA-Z]`)

func SanitizeTLAIdentifier(name string) string {
	if !validTLAIdentifierPrefix.MatchString(name) {
		name = "id" + name
	}
	return invalidTLAIdentifierPart.ReplaceAllString(name, "_")
}
func ParseableTLAName(name string) string {
	return SanitizeTLAIdentifier(strings.NewReplacer("!", "_i_", "$", "_si_").Replace(name))
}

func FormatTLAModule(module *Module, options TLAFormatOptions) (string, error) {
	return formatTLAModuleWith(module, options, func(declaration Decl) (string, error) {
		return FormatTLADeclaration(declaration, options.Annotations)
	})
}

func formatTLAModuleWith(
	module *Module,
	options TLAFormatOptions,
	formatDeclaration func(Decl) (string, error),
) (string, error) {
	if module == nil {
		return "", fmt.Errorf("cannot format nil module")
	}
	var output strings.Builder
	middle := " MODULE " + SanitizeTLAIdentifier(module.Name()) + " "
	dashes := (80 - len(middle)) / 2
	if dashes < 5 {
		dashes = 5
	}
	output.WriteString(strings.Repeat("-", dashes) + middle + strings.Repeat("-", dashes) + "\n")
	if len(options.ExtendedModules) > 0 {
		names := make([]string, len(options.ExtendedModules))
		for i, name := range options.ExtendedModules {
			names[i] = SanitizeTLAIdentifier(name)
		}
		output.WriteString("\nEXTENDS " + strings.Join(names, ", ") + "\n")
	}
	for _, declaration := range module.Declarations() {
		formatted, err := formatDeclaration(declaration)
		if err != nil {
			return "", err
		}
		output.WriteString("\n" + formatted + "\n")
	}
	output.WriteString("\n" + strings.Repeat("=", 80) + "\n")
	return output.String(), nil
}

func FormatTLADeclaration(declaration Decl, annotations *AnnotationStore) (string, error) {
	if declaration == nil {
		return "", fmt.Errorf("cannot format nil declaration")
	}
	prefix := ""
	if annotations != nil {
		items := annotations.Get(declaration.ID())
		if len(items) > 0 {
			lines := make([]string, len(items))
			for i, item := range items {
				lines[i] = "  " + item.PrettyString()
			}
			prefix = "(*\n" + strings.Join(lines, "\n") + "\n*)\n"
		}
	}
	switch typed := declaration.(type) {
	case *ConstDecl:
		return prefix + "CONSTANT " + ParseableTLAName(typed.Name()), nil
	case *VarDecl:
		return prefix + "VARIABLE " + ParseableTLAName(typed.Name()), nil
	case *AssumeDecl:
		body, err := FormatTLAExpression(typed.Body())
		if err != nil {
			return "", err
		}
		if name, named := typed.DefinedName(); named {
			return prefix + "ASSUME " + ParseableTLAName(name) + " == (" + body + ")", nil
		}
		return prefix + "ASSUME (" + body + ")", nil
	case *TheoremDecl:
		body, err := FormatTLAExpression(typed.Body())
		if err != nil {
			return "", err
		}
		return prefix + "THEOREM " + ParseableTLAName(typed.Name()) + " == " + body, nil
	case *OperDecl:
		if recursiveFunction, ok := typed.Body().(*OperExpr); ok && recursiveFunction.Operator().Name() == "FUN_REC_CTOR" {
			arguments := recursiveFunction.Arguments()
			bindings, err := formatTLABindings(arguments[1:])
			if err != nil {
				return "", err
			}
			body, err := FormatTLAExpression(replaceRecursiveFunctionRefs(arguments[0], typed.Name()))
			if err != nil {
				return "", err
			}
			return prefix + ParseableTLAName(typed.Name()) + "[" + bindings + "] == " + body, nil
		}
		body, err := FormatTLAExpression(typed.Body())
		if err != nil {
			return "", err
		}
		parameters := formatTLAParameters(typed.FormalParams())
		recursive := ""
		if typed.IsRecursive() {
			recursive = "RECURSIVE " + ParseableTLAName(typed.Name()) + formatTLAArity(len(typed.FormalParams())) + "\n"
		}
		return recursive + prefix + ParseableTLAName(typed.Name()) + parameters + " == " + body, nil
	default:
		return "", fmt.Errorf("unsupported declaration %T", declaration)
	}
}

func formatTLAParameters(parameters []OperatorParam) string {
	if len(parameters) == 0 {
		return ""
	}
	formatted := make([]string, len(parameters))
	for i, param := range parameters {
		name := ParseableTLAName(param.Name)
		if param.Arity > 0 {
			name += formatTLAArity(param.Arity)
		}
		formatted[i] = name
	}
	return "(" + strings.Join(formatted, ", ") + ")"
}
func formatTLAArity(arity int) string {
	if arity <= 0 {
		return ""
	}
	return "(" + strings.TrimSuffix(strings.Repeat("_, ", arity), ", ") + ")"
}

func FormatTLAExpression(expression Expr) (string, error) {
	return formatTLAExpressionAt(Precedence{}, expression)
}

func formatTLAExpressionAt(parent Precedence, expression Expr) (string, error) {
	switch typed := expression.(type) {
	case *nullExpr:
		return `"NOP"`, nil
	case *NameExpr:
		return ParseableTLAName(typed.Name()), nil
	case *ValueExpr:
		return formatTLAValue(typed.Value(), parent)
	case *LetInExpr:
		parts := make([]string, len(typed.Declarations()))
		for i, declaration := range typed.Declarations() {
			formatted, err := FormatTLADeclaration(declaration, nil)
			if err != nil {
				return "", err
			}
			parts[i] = "LET " + formatted + " IN"
		}
		body, err := formatTLAExpressionAt(Precedence{}, typed.Body())
		if err != nil {
			return "", err
		}
		return wrapTLA(parent, Precedence{}, strings.Join(parts, " ")+" "+body), nil
	case *OperExpr:
		return formatTLAOperator(parent, typed)
	default:
		return "", fmt.Errorf("unsupported expression %T", expression)
	}
}
func formatTLAValue(value Value, parent Precedence) (string, error) {
	switch typed := value.(type) {
	case IntValue:
		text := typed.String()
		if typed.BigInt().Sign() < 0 {
			return wrapTLA(parent, Op("UNARY_MINUS").Precedence(), text), nil
		}
		return text, nil
	case DecimalValue:
		return finiteDecimalString(typed.Rat())
	case BoolValue:
		if typed.Bool() {
			return "TRUE", nil
		}
		return "FALSE", nil
	case StringValue:
		return fmt.Sprintf("%q", typed.Value()), nil
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

var tlaUnary = map[string]string{"NOT": "~", "UNARY_MINUS": "-", "SET_UNARY_UNION": "UNION ", "SET_POWERSET": "SUBSET ", "ENABLED": "ENABLED ", "UNCHANGED": "UNCHANGED ", "DOMAIN": "DOMAIN ", "GLOBALLY": "[]", "EVENTUALLY": "<>"}
var tlaBinary = map[string]string{"EQ": "=", "NE": "/=", "IMPLIES": "=>", "EQUIV": "<=>", "PLUS": "+", "MINUS": "-", "MULT": "*", "DIV": "\\div", "MOD": "%", "REAL_DIV": "/", "POW": "^", "INT_RANGE": "..", "LT": "<", "GT": ">", "LE": "<=", "GE": ">=", "SET_IN": "\\in", "SET_NOT_IN": "\\notin", "SET_INTERSECT": "\\intersect", "SET_UNION2": "\\union", "SET_MINUS": "\\", "SET_SUBSET_EQ": "\\subseteq", "COMPOSE": "\\cdot", "LEADS_TO": "~>", "GUARANTEES": "-+->", "Sequences!Concat": "\\o", "Apalache!:=": ":="}

func formatTLAOperator(parent Precedence, expression *OperExpr) (string, error) {
	operator := expression.Operator()
	name := operator.Name()
	arguments := expression.Arguments()
	format := func(index int, precedence Precedence) (string, error) {
		return formatTLAExpressionAt(precedence, arguments[index])
	}
	if name == "PRIME" {
		argument, err := format(0, operator.Precedence())
		if err != nil {
			return "", err
		}
		return wrapTLA(parent, operator.Precedence(), argument+"'"), nil
	}
	if sign, ok := tlaUnary[name]; ok {
		argument, err := format(0, operator.Precedence())
		if err != nil {
			return "", err
		}
		return wrapTLA(parent, operator.Precedence(), sign+argument), nil
	}
	if sign, ok := tlaBinary[name]; ok {
		left, err := format(0, operator.Precedence())
		if err != nil {
			return "", err
		}
		right, err := format(1, operator.Precedence())
		if err != nil {
			return "", err
		}
		return wrapTLA(parent, operator.Precedence(), left+" "+sign+" "+right), nil
	}
	switch name {
	case "AND", "OR":
		if len(arguments) == 0 {
			if name == "AND" {
				return "TRUE", nil
			}
			return "FALSE", nil
		}
		items, err := formatTLAArgs(arguments, operator.Precedence())
		if err != nil {
			return "", err
		}
		sign := " /\\ "
		if name == "OR" {
			sign = " \\/ "
		}
		return wrapTLA(parent, operator.Precedence(), strings.Join(items, sign)), nil
	case "SET_ENUM":
		items, err := formatTLAArgs(arguments, operator.Precedence())
		if err != nil {
			return "", err
		}
		if len(items) == 0 {
			return "{}", nil
		}
		if len(items) == 1 {
			return "{" + items[0] + "}", nil
		}
		return "{ " + strings.Join(items, ", ") + " }", nil
	case "TUPLE":
		items, err := formatTLAArgs(arguments, operator.Precedence())
		if err != nil {
			return "", err
		}
		return "<<" + strings.Join(items, ", ") + ">>", nil
	case "OPER_APP":
		if len(arguments) == 0 {
			return "", fmt.Errorf("operator application has no operator")
		}
		callee, err := formatTLAExpressionAt(operator.Precedence(), arguments[0])
		if err != nil {
			return "", err
		}
		return formatTLAApplication(parent, operator.Precedence(), callee, arguments[1:])
	case "EXISTS3", "FORALL3", "CHOOSE3":
		return formatTLABinder(parent, expression, true)
	case "EXISTS2", "FORALL2", "CHOOSE2":
		return formatTLABinder(parent, expression, false)
	case "SET_FILTER":
		variable, err := format(0, Precedence{})
		if err != nil {
			return "", err
		}
		set, err := format(1, Precedence{})
		if err != nil {
			return "", err
		}
		predicate, err := format(2, Precedence{})
		if err != nil {
			return "", err
		}
		return "{" + variable + " \\in " + set + ": " + predicate + "}", nil
	case "SET_MAP":
		return formatTLAMapSet(parent, expression)
	case "FUN_CTOR":
		return formatTLAFunCtor(parent, expression)
	case "FUN_REC_CTOR":
		return "", fmt.Errorf("recursive function constructor must be the body of an operator declaration")
	case "FUN_REC_REF":
		return "REC_FUN_UNDEFINED", nil
	case "FUN_APP":
		function, err := format(0, operator.Precedence())
		if err != nil {
			return "", err
		}
		argument, err := format(1, operator.Precedence())
		if err != nil {
			return "", err
		}
		return function + "[" + argument + "]", nil
	case "RECORD":
		return formatTLAKeyValues(arguments, "[", " |-> ", "]")
	case "RECORD_SET":
		return formatTLAKeyValues(arguments, "[", ": ", "]")
	case "FUN_SET":
		domain, err := format(0, operator.Precedence())
		if err != nil {
			return "", err
		}
		codomain, err := format(1, operator.Precedence())
		if err != nil {
			return "", err
		}
		return "[" + domain + " -> " + codomain + "]", nil
	case "EXCEPT":
		return formatTLAExcept(expression)
	case "IF_THEN_ELSE":
		condition, err := format(0, Precedence{})
		if err != nil {
			return "", err
		}
		thenText, err := format(1, Precedence{})
		if err != nil {
			return "", err
		}
		elseText, err := format(2, Precedence{})
		if err != nil {
			return "", err
		}
		return wrapTLA(parent, operator.Precedence(), "IF "+condition+" THEN "+thenText+" ELSE "+elseText), nil
	case "CASE", "CASE_OTHER":
		return formatTLACase(parent, expression)
	case "STUTTER", "NO_STUTTER":
		action, err := format(0, operator.Precedence())
		if err != nil {
			return "", err
		}
		subscript, err := formatTLASubscript(arguments[1])
		if err != nil {
			return "", err
		}
		if name == "STUTTER" {
			return "[" + action + "]_" + subscript, nil
		}
		return "<<" + action + ">>_" + subscript, nil
	case "WEAK_FAIRNESS", "STRONG_FAIRNESS":
		subscript, err := formatTLASubscript(arguments[0])
		if err != nil {
			return "", err
		}
		action, err := format(1, Precedence{})
		if err != nil {
			return "", err
		}
		prefix := "WF"
		if name == "STRONG_FAIRNESS" {
			prefix = "SF"
		}
		return prefix + "_" + subscript + "(" + action + ")", nil
	case "SET_TIMES":
		items, err := formatTLAArgs(arguments, operator.Precedence())
		if err != nil {
			return "", err
		}
		return wrapTLA(parent, operator.Precedence(), strings.Join(items, " \\X ")), nil
	case "TEMPORAL_EXISTS", "TEMPORAL_FORALL":
		variable, err := format(0, Precedence{})
		if err != nil {
			return "", err
		}
		body, err := format(1, operator.Precedence())
		if err != nil {
			return "", err
		}
		sign := "\\EE"
		if name == "TEMPORAL_FORALL" {
			sign = "\\AA"
		}
		return wrapTLA(parent, operator.Precedence(), sign+" "+variable+": "+body), nil
	case "LABEL":
		if len(arguments) < 2 {
			return "", fmt.Errorf("malformed label")
		}
		body, err := format(0, Precedence{Low: 160, High: 160})
		if err != nil {
			return "", err
		}
		labelValue, ok := arguments[1].(*ValueExpr)
		if !ok {
			return "", fmt.Errorf("malformed label name: expected string value")
		}
		labelString, ok := labelValue.Value().(StringValue)
		if !ok {
			return "", fmt.Errorf("malformed label name: expected string value")
		}
		label := labelString.Value()
		extra := make([]string, 0, len(arguments)-2)
		for _, arg := range arguments[2:] {
			argValue, ok := arg.(*ValueExpr)
			if !ok {
				return "", fmt.Errorf("malformed label argument: expected string value")
			}
			argString, ok := argValue.Value().(StringValue)
			if !ok {
				return "", fmt.Errorf("malformed label argument: expected string value")
			}
			extra = append(extra, argString.Value())
		}
		if len(extra) > 0 {
			label += "(" + strings.Join(extra, ", ") + ")"
		}
		// Scala declares LABEL at precedence (160, 160), but its concrete
		// syntax is an undelimited construct that must be grouped whenever it
		// is nested. Keep that printer rule separate from operator metadata.
		return wrapTLA(parent, Precedence{}, label+" :: "+body), nil
	case "Variants!Variant":
		items, err := formatTLAArgs(arguments, operator.Precedence())
		if err != nil {
			return "", err
		}
		return "Variants!Variant(" + strings.Join(items, ", ") + ")", nil
	}
	unqualified := name
	if bang := strings.LastIndex(unqualified, "!"); bang >= 0 {
		unqualified = unqualified[bang+1:]
	}
	unqualified = ParseableTLAName(unqualified)
	return formatTLAApplication(parent, operator.Precedence(), unqualified, arguments)
}

func formatTLAApplication(parent, precedence Precedence, callee string, arguments []Expr) (string, error) {
	declarations, arguments := extractTLAArgumentLets(arguments)
	items, err := formatTLAArgs(arguments, precedence)
	if err != nil {
		return "", err
	}
	application := callee
	if len(items) > 0 {
		application += "(" + strings.Join(items, ", ") + ")"
	}
	if len(declarations) == 0 {
		if len(items) == 0 {
			return application, nil
		}
		return wrapTLA(parent, precedence, application), nil
	}
	parts := make([]string, len(declarations))
	for index, declaration := range declarations {
		formatted, err := FormatTLADeclaration(declaration, nil)
		if err != nil {
			return "", err
		}
		parts[index] = "LET " + formatted + " IN"
	}
	return wrapTLA(parent, precedence, "("+strings.Join(parts, " ")+" "+application+")"), nil
}

func extractTLAArgumentLets(arguments []Expr) ([]*OperDecl, []Expr) {
	var declarations []*OperDecl
	result := append([]Expr(nil), arguments...)
	for index, argument := range result {
		local, ok := argument.(*LetInExpr)
		if !ok {
			continue
		}
		localDeclarations := local.Declarations()
		if len(localDeclarations) == 1 && localDeclarations[0].Name() == "LAMBDA" {
			continue
		}
		declarations = append(declarations, localDeclarations...)
		result[index] = local.Body()
	}
	return declarations, result
}

func formatTLAArgs(arguments []Expr, precedence Precedence) ([]string, error) {
	items := make([]string, len(arguments))
	for i, argument := range arguments {
		text, err := formatTLAExpressionAt(precedence, argument)
		if err != nil {
			return nil, err
		}
		items[i] = text
	}
	return items, nil
}
func wrapTLA(parent, expression Precedence, text string) string {
	if parent != (Precedence{}) && expression.Low <= parent.High {
		return "(" + text + ")"
	}
	return text
}
func formatTLABinder(parent Precedence, expression *OperExpr, bounded bool) (string, error) {
	arguments := expression.Arguments()
	operator := expression.Operator()
	variable, err := formatTLAExpressionAt(Precedence{}, arguments[0])
	if err != nil {
		return "", err
	}
	sign := map[string]string{"EXISTS3": "\\E", "FORALL3": "\\A", "CHOOSE3": "CHOOSE", "EXISTS2": "\\E", "FORALL2": "\\A", "CHOOSE2": "CHOOSE"}[operator.Name()]
	if bounded {
		set, err := formatTLAExpressionAt(operator.Precedence(), arguments[1])
		if err != nil {
			return "", err
		}
		body, err := formatTLAExpressionAt(operator.Precedence(), arguments[2])
		if err != nil {
			return "", err
		}
		return wrapTLA(parent, operator.Precedence(), sign+" "+variable+" \\in "+set+": "+body), nil
	}
	body, err := formatTLAExpressionAt(operator.Precedence(), arguments[1])
	if err != nil {
		return "", err
	}
	return wrapTLA(parent, operator.Precedence(), sign+" "+variable+" : "+body), nil
}
func formatTLAMapSet(parent Precedence, expression *OperExpr) (string, error) {
	arguments := expression.Arguments()
	body, err := formatTLAExpressionAt(Precedence{}, arguments[0])
	if err != nil {
		return "", err
	}
	bindings, err := formatTLABindings(arguments[1:])
	if err != nil {
		return "", err
	}
	return "{ (" + body + "): " + bindings + " }", nil
}
func formatTLAFunCtor(parent Precedence, expression *OperExpr) (string, error) {
	arguments := expression.Arguments()
	body, err := formatTLAExpressionAt(Precedence{}, arguments[0])
	if err != nil {
		return "", err
	}
	bindings, err := formatTLABindings(arguments[1:])
	if err != nil {
		return "", err
	}
	return "[" + bindings + " |-> " + body + "]", nil
}
func formatTLABindings(arguments []Expr) (string, error) {
	if len(arguments)%2 != 0 {
		return "", fmt.Errorf("expected name/set pairs")
	}
	items := make([]string, len(arguments)/2)
	for i := 0; i < len(arguments); i += 2 {
		name, err := formatTLAExpressionAt(Precedence{}, arguments[i])
		if err != nil {
			return "", err
		}
		set, err := formatTLAExpressionAt(Precedence{}, arguments[i+1])
		if err != nil {
			return "", err
		}
		items[i/2] = name + " \\in " + set
	}
	return strings.Join(items, ", "), nil
}
func formatTLAKeyValues(arguments []Expr, left, middle, right string) (string, error) {
	if len(arguments)%2 != 0 {
		return "", fmt.Errorf("expected key/value pairs")
	}
	items := make([]string, len(arguments)/2)
	for i := 0; i < len(arguments); i += 2 {
		keyValue, ok := arguments[i].(*ValueExpr)
		if !ok {
			return "", fmt.Errorf("key is not a value")
		}
		key, ok := keyValue.Value().(StringValue)
		if !ok {
			return "", fmt.Errorf("key is not a string")
		}
		value, err := formatTLAExpressionAt(Precedence{}, arguments[i+1])
		if err != nil {
			return "", err
		}
		items[i/2] = key.Value() + middle + value
	}
	return left + strings.Join(items, ", ") + right, nil
}
func formatTLAExcept(expression *OperExpr) (string, error) {
	arguments := expression.Arguments()
	function, err := formatTLAExpressionAt(expression.Operator().Precedence(), arguments[0])
	if err != nil {
		return "", err
	}
	updates := make([]string, 0, (len(arguments)-1)/2)
	for i := 1; i < len(arguments); i += 2 {
		path, ok := arguments[i].(*OperExpr)
		if !ok || path.Operator().Name() != "TUPLE" {
			return "", fmt.Errorf("malformed EXCEPT path: expected tuple")
		}
		indices, err := formatTLAArgs(path.Arguments(), Precedence{})
		if err != nil {
			return "", err
		}
		value, err := formatTLAExpressionAt(Precedence{}, arguments[i+1])
		if err != nil {
			return "", err
		}
		updates = append(updates, "!["+strings.Join(indices, ", ")+"] = "+value)
	}
	return "[" + function + " EXCEPT " + strings.Join(updates, ", ") + "]", nil
}

func replaceRecursiveFunctionRefs(expression Expr, name string) Expr {
	switch typed := expression.(type) {
	case *OperExpr:
		if typed.Operator().Name() == "FUN_REC_REF" {
			return NewNameExpr(name, typed.Tag())
		}
		arguments := typed.Arguments()
		for i, argument := range arguments {
			arguments[i] = replaceRecursiveFunctionRefs(argument, name)
		}
		return MustOperExpr(typed.Operator(), typed.Tag(), arguments...)
	case *LetInExpr:
		declarations := typed.Declarations()
		for i, declaration := range declarations {
			copy := declaration.copyWithTag(declaration.Tag())
			copy.SetBody(replaceRecursiveFunctionRefs(declaration.Body(), name))
			declarations[i] = copy
		}
		return NewLetInExpr(replaceRecursiveFunctionRefs(typed.Body(), name), typed.Tag(), declarations...)
	default:
		return expression
	}
}
func formatTLACase(parent Precedence, expression *OperExpr) (string, error) {
	arguments := expression.Arguments()
	start := 0
	other := ""
	if expression.Operator().Name() == "CASE_OTHER" {
		value, err := formatTLAExpressionAt(Precedence{}, arguments[0])
		if err != nil {
			return "", err
		}
		other = " [] OTHER -> " + value
		start = 1
	}
	if (len(arguments)-start)%2 != 0 {
		return "", fmt.Errorf("malformed CASE")
	}
	items := make([]string, 0, (len(arguments)-start)/2)
	for i := start; i < len(arguments); i += 2 {
		guard, err := formatTLAExpressionAt(Precedence{}, arguments[i])
		if err != nil {
			return "", err
		}
		value, err := formatTLAExpressionAt(Precedence{}, arguments[i+1])
		if err != nil {
			return "", err
		}
		items = append(items, guard+" -> "+value)
	}
	return wrapTLA(parent, expression.Operator().Precedence(), "CASE "+strings.Join(items, " [] ")+other), nil
}
func formatTLASubscript(expression Expr) (string, error) {
	text, err := formatTLAExpressionAt(Precedence{}, expression)
	if err != nil {
		return "", err
	}
	switch expression.(type) {
	case *NameExpr:
		return text, nil
	case *ValueExpr:
		if _, ok := expression.(*ValueExpr).Value().(BoolValue); ok {
			return text, nil
		}
	}
	return "(" + text + ")", nil
}

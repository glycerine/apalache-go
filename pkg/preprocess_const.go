package apalache

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

// SimplifyConstants recursively applies the Scala ConstSimplifier rules.
func SimplifyConstants(root Expr, tracker TransformationTracker) (Expr, error) {
	if root == nil {
		return nil, fmt.Errorf("cannot simplify a nil expression")
	}
	if err := WatchExpression(FlatLanguagePredicate{}, root); err != nil {
		return nil, err
	}
	if tracker == nil {
		tracker = IdleTracker{}
	}
	var simplify func(Expr) (Expr, error)
	simplify = func(expression Expr) (Expr, error) {
		var rebuilt Expr = expression
		switch typed := expression.(type) {
		case *OperExpr:
			arguments := typed.Arguments()
			for index, argument := range arguments {
				var err error
				arguments[index], err = simplify(argument)
				if err != nil {
					return nil, err
				}
			}
			rebuilt = MustOperExpr(typed.Operator(), typed.Tag(), arguments...)
		case *LetInExpr:
			declarations := typed.Declarations()
			for index, declaration := range declarations {
				body, err := simplify(declaration.Body())
				if err != nil {
					return nil, err
				}
				copy := declaration.copyWithTag(declaration.Tag())
				copy.SetBody(body)
				declarations[index] = copy
			}
			body, err := simplify(typed.Body())
			if err != nil {
				return nil, err
			}
			rebuilt = NewLetInExpr(body, typed.Tag(), declarations...)
		}
		result, err := simplifyConstantShallow(rebuilt)
		if err != nil {
			return nil, err
		}
		return tracker.TrackExpression(func(Expr) Expr { return result })(expression), nil
	}
	return simplify(root)
}

func simplifyConstantShallow(expression Expr) (Expr, error) {
	op, ok := expression.(*OperExpr)
	if !ok {
		return expression, nil
	}
	args := op.Arguments()
	name := op.Operator().Name()
	boolEx := func(value bool) Expr { return NewValueExpr(BoolValue(value), Typed(BoolT1)) }
	intEx := func(value *big.Int) Expr { return NewValueExpr(NewIntValue(value), Typed(IntT1)) }
	zero, one := big.NewInt(0), big.NewInt(1)
	intAt := func(index int) (*big.Int, bool) {
		if index < 0 || index >= len(args) {
			return nil, false
		}
		value, ok := args[index].(*ValueExpr)
		if !ok {
			return nil, false
		}
		integer, ok := value.Value().(IntValue)
		if !ok {
			return nil, false
		}
		return integer.BigInt(), true
	}
	boolAt := func(index int) (bool, bool) {
		if index < 0 || index >= len(args) {
			return false, false
		}
		value, ok := args[index].(*ValueExpr)
		if !ok {
			return false, false
		}
		boolean, ok := value.Value().(BoolValue)
		return bool(boolean), ok
	}
	knownSetEmpty := func(expr Expr) (bool, bool) {
		set, ok := expr.(*OperExpr)
		if !ok {
			return false, false
		}
		switch set.Operator().Name() {
		case "SET_ENUM":
			return len(set.Arguments()) == 0, true
		case "INT_RANGE":
			bounds := set.Arguments()
			lower, lowerOK := constantInteger(bounds[0])
			upper, upperOK := constantInteger(bounds[1])
			if lowerOK && upperOK {
				return lower.Cmp(upper) > 0, true
			}
		}
		return false, false
	}
	isEmpty := func(expr Expr) bool {
		empty, known := knownSetEmpty(expr)
		return known && empty
	}
	empty := func(tag TypeTag) Expr { return MustOperExpr(Op("SET_ENUM"), tag) }
	expressionText := func(expression Expr) string {
		formatted, err := FormatTLAExpression(expression)
		if err != nil {
			return fmt.Sprint(expression)
		}
		return formatted
	}

	if name == "NOT" {
		if value, literal := boolAt(0); literal {
			return boolEx(!value), nil
		}
		if inner, nested := args[0].(*OperExpr); nested && inner.Operator().Name() == "NOT" {
			return inner.Arguments()[0], nil
		}
		if inner, nested := args[0].(*OperExpr); nested && inner.Operator().Name() == "NE" {
			return simplifyConstantShallow(MustOperExpr(Op("EQ"), Typed(BoolT1), inner.Arguments()...))
		}
		if inner, nested := args[0].(*OperExpr); nested && inner.Operator().Name() == "SET_NOT_IN" {
			return simplifyConstantShallow(MustOperExpr(Op("SET_IN"), Typed(BoolT1), inner.Arguments()...))
		}
	}
	if name == "NE" || name == "SET_NOT_IN" {
		base := "EQ"
		if name == "SET_NOT_IN" {
			base = "SET_IN"
		}
		inner, err := simplifyConstantShallow(MustOperExpr(Op(base), Typed(BoolT1), args...))
		if err != nil {
			return nil, err
		}
		return simplifyConstantShallow(MustOperExpr(Op("NOT"), Typed(BoolT1), inner))
	}

	left, leftInt := intAt(0)
	var right *big.Int
	rightInt := false
	if len(args) > 1 {
		right, rightInt = intAt(1)
	}
	switch name {
	case "PLUS":
		if leftInt && rightInt {
			return intEx(new(big.Int).Add(left, right)), nil
		}
		if leftInt && left.Sign() == 0 {
			return args[1], nil
		}
		if rightInt && right.Sign() == 0 {
			return args[0], nil
		}
	case "MINUS":
		if leftInt && rightInt {
			return intEx(new(big.Int).Sub(left, right)), nil
		}
		if leftInt && left.Sign() == 0 {
			return MustOperExpr(Op("UNARY_MINUS"), Typed(IntT1), args[1]), nil
		}
		if rightInt && right.Sign() == 0 {
			return args[0], nil
		}
		if EqualExpr(args[0], args[1]) {
			return intEx(zero), nil
		}
	case "MULT":
		if leftInt && rightInt {
			return intEx(new(big.Int).Mul(left, right)), nil
		}
		if leftInt && left.Sign() == 0 || rightInt && right.Sign() == 0 {
			return intEx(zero), nil
		}
		if leftInt && left.Cmp(one) == 0 {
			return args[1], nil
		}
		if rightInt && right.Cmp(one) == 0 {
			return args[0], nil
		}
	case "DIV":
		if rightInt && right.Sign() == 0 {
			return nil, NewTlaInputError("Division by zero at " + expressionText(op))
		}
		if leftInt && rightInt {
			return intEx(new(big.Int).Quo(left, right)), nil
		}
		if leftInt && left.Sign() == 0 {
			return intEx(zero), nil
		}
		if rightInt && right.Cmp(one) == 0 {
			return args[0], nil
		}
		if EqualExpr(args[0], args[1]) {
			return intEx(one), nil
		}
	case "MOD":
		if rightInt && right.Sign() == 0 {
			return nil, NewTlaInputError("Mod by zero at " + expressionText(op))
		}
		if leftInt && rightInt {
			return intEx(new(big.Int).Rem(left, right)), nil
		}
		if rightInt && right.Cmp(one) == 0 || EqualExpr(args[0], args[1]) {
			return intEx(zero), nil
		}
	case "POW":
		if leftInt && rightInt {
			if left.Sign() == 0 && right.Sign() == 0 {
				return nil, NewTlaInputError("0 ^ 0 is undefined")
			}
			if right.Sign() < 0 {
				return nil, NewTlaInputError("Negative power at " + expressionText(op))
			}
			if !right.IsInt64() || right.Int64() > math.MaxInt32 {
				return nil, NewTlaInputError(fmt.Sprintf("The power at %s exceedes the limit of %d", expressionText(op), math.MaxInt32))
			}
			baseFloat, _ := new(big.Float).SetInt(left).Float64()
			estimatedPower := math.Pow(baseFloat, float64(right.Int64()))
			// Zero and negative finite results are ordinary integer powers. The
			// floating estimate is used only as the Scala-compatible upper-size
			// guard; its sign and underflow threshold must not reject them.
			if math.IsInf(estimatedPower, 0) || math.IsNaN(estimatedPower) || math.Abs(estimatedPower) > math.MaxFloat64 {
				limit := strings.Replace(strconv.FormatFloat(math.MaxFloat64, 'E', -1, 64), "E+", "E", 1)
				return nil, NewTlaInputError(fmt.Sprintf("The result of %s exceedes the limit of %s", expressionText(op), limit))
			}
			return intEx(new(big.Int).Exp(left, right, nil)), nil
		}
		if rightInt && right.Sign() == 0 {
			return intEx(one), nil
		}
		if rightInt && right.Cmp(one) == 0 {
			return args[0], nil
		}
		if leftInt && left.Sign() == 0 {
			return intEx(zero), nil
		}
		if leftInt && left.Cmp(one) == 0 {
			return intEx(one), nil
		}
	case "UNARY_MINUS":
		if leftInt {
			return intEx(new(big.Int).Neg(left)), nil
		}
	case "LT", "LE", "GT", "GE":
		if leftInt && rightInt {
			comparison := left.Cmp(right)
			return boolEx(name == "LT" && comparison < 0 || name == "LE" && comparison <= 0 || name == "GT" && comparison > 0 || name == "GE" && comparison >= 0), nil
		}
	}

	if name == "EQ" {
		if EqualExpr(args[0], args[1]) {
			return boolEx(true), nil
		}
		if leftInt && rightInt {
			return boolEx(left.Cmp(right) == 0), nil
		}
		leftValue, lok := args[0].(*ValueExpr)
		rightValue, rok := args[1].(*ValueExpr)
		if lok && rok {
			if _, stringLeft := leftValue.Value().(StringValue); stringLeft {
				if _, stringRight := rightValue.Value().(StringValue); stringRight {
					return boolEx(EqualValues(leftValue.Value(), rightValue.Value())), nil
				}
			}
		}
	}
	if name == "AND" || name == "OR" {
		identity := name == "AND"
		filtered := make([]Expr, 0, len(args))
		for _, argument := range args {
			value, literal := boolLiteral(argument)
			if literal && value == identity {
				continue
			}
			if literal && value != identity {
				return boolEx(!identity), nil
			}
			filtered = append(filtered, argument)
		}
		if len(filtered) == 0 {
			return boolEx(identity), nil
		}
		if len(filtered) == 1 {
			return filtered[0], nil
		}
		return MustOperExpr(op.Operator(), Typed(BoolT1), filtered...), nil
	}
	if name == "IMPLIES" {
		leftBool, lb := boolAt(0)
		rightBool, rb := boolAt(1)
		if lb && rb {
			return boolEx(!leftBool || rightBool), nil
		}
		if lb && !leftBool || rb && rightBool {
			return boolEx(true), nil
		}
		if lb && leftBool {
			return args[1], nil
		}
		if rb && !rightBool {
			return simplifyConstantShallow(MustOperExpr(Op("NOT"), Typed(BoolT1), args[0]))
		}
	}
	if name == "IF_THEN_ELSE" {
		predicate, literal := boolAt(0)
		if literal {
			if predicate {
				return args[1], nil
			}
			return args[2], nil
		}
		thenValue, thenBool := boolLiteral(args[1])
		elseValue, elseBool := boolLiteral(args[2])
		if thenBool && elseBool && thenValue && !elseValue {
			return args[0], nil
		}
		if thenBool && elseBool && !thenValue && elseValue {
			return simplifyConstantShallow(MustOperExpr(Op("NOT"), Typed(BoolT1), args[0]))
		}
		if thenBool && !thenValue {
			negated, err := simplifyConstantShallow(MustOperExpr(Op("NOT"), Typed(BoolT1), args[0]))
			if err != nil {
				return nil, err
			}
			return simplifyConstantShallow(MustOperExpr(Op("AND"), Typed(BoolT1), negated, args[2]))
		}
		if thenBool && thenValue {
			return simplifyConstantShallow(MustOperExpr(Op("OR"), Typed(BoolT1), args[0], args[2]))
		}
		if EqualExpr(args[1], args[2]) {
			return args[1], nil
		}
	}
	if name == "EXISTS3" || name == "FORALL3" {
		setEmpty, emptinessKnown := knownSetEmpty(args[1])
		if emptinessKnown && setEmpty {
			return boolEx(name == "FORALL3"), nil
		}
		body, literal := boolLiteral(args[2])
		if literal {
			if emptinessKnown {
				return boolEx(body), nil
			}
			if name == "EXISTS3" && !body {
				return boolEx(false), nil
			}
			if name == "FORALL3" && body {
				return boolEx(true), nil
			}
			equality := MustOperExpr(Op("EQ"), Typed(BoolT1), args[1], empty(args[1].Tag()))
			if name == "EXISTS3" {
				return simplifyConstantShallow(MustOperExpr(Op("NOT"), Typed(BoolT1), equality))
			}
			return simplifyConstantShallow(equality)
		}
	}
	if name == "SET_IN" && isEmpty(args[1]) {
		return boolEx(false), nil
	}
	if name == "SET_SUBSET_EQ" && (isEmpty(args[0]) || EqualExpr(args[0], args[1])) {
		return boolEx(true), nil
	}
	if name == "SET_UNION2" {
		if isEmpty(args[0]) {
			return args[1], nil
		}
		if isEmpty(args[1]) || EqualExpr(args[0], args[1]) {
			return args[0], nil
		}
	}
	if name == "SET_INTERSECT" {
		if isEmpty(args[0]) {
			return args[0], nil
		}
		if isEmpty(args[1]) {
			return args[1], nil
		}
		if EqualExpr(args[0], args[1]) {
			return args[0], nil
		}
	}
	if name == "SET_MINUS" {
		if isEmpty(args[0]) {
			return args[0], nil
		}
		if isEmpty(args[1]) {
			return args[0], nil
		}
		if EqualExpr(args[0], args[1]) {
			return empty(op.Tag()), nil
		}
	}
	if name == "SET_UNION2" || name == "SET_INTERSECT" || name == "SET_MINUS" {
		if result, computable := simplifyLiteralSetBinary(name, args[0], args[1], op.Tag()); computable {
			return result, nil
		}
	}
	if name == "FiniteSets!Cardinality" {
		if literals, ok := literalSet(args[0]); ok {
			return intEx(big.NewInt(int64(len(literals)))), nil
		}
	}
	return expression, nil
}

func boolLiteral(expression Expr) (bool, bool) {
	value, ok := expression.(*ValueExpr)
	if !ok {
		return false, false
	}
	boolean, ok := value.Value().(BoolValue)
	return bool(boolean), ok
}

type constantLiteral struct {
	kind    int
	boolean bool
	text    string
	integer *big.Int
}

func (literal constantLiteral) key() string {
	if literal.kind == 0 {
		return fmt.Sprintf("b:%t", literal.boolean)
	}
	if literal.kind == 1 {
		return "s:" + literal.text
	}
	return "i:" + literal.integer.String()
}
func literalSet(expression Expr) ([]constantLiteral, bool) {
	set, ok := expression.(*OperExpr)
	if !ok || set.Operator().Name() != "SET_ENUM" {
		return nil, false
	}
	unique := make(map[string]constantLiteral)
	for _, element := range set.Arguments() {
		value, ok := element.(*ValueExpr)
		if !ok {
			return nil, false
		}
		var literal constantLiteral
		switch typed := value.Value().(type) {
		case BoolValue:
			literal = constantLiteral{kind: 0, boolean: bool(typed)}
		case StringValue:
			literal = constantLiteral{kind: 1, text: typed.Value()}
		case IntValue:
			literal = constantLiteral{kind: 2, integer: typed.BigInt()}
		default:
			return nil, false
		}
		unique[literal.key()] = literal
	}
	result := make([]constantLiteral, 0, len(unique))
	for _, literal := range unique {
		result = append(result, literal)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].kind != result[j].kind {
			return result[i].kind < result[j].kind
		}
		if result[i].kind == 0 {
			return !result[i].boolean && result[j].boolean
		}
		if result[i].kind == 1 {
			return result[i].text < result[j].text
		}
		return result[i].integer.Cmp(result[j].integer) < 0
	})
	return result, true
}
func simplifyLiteralSetBinary(operator string, left, right Expr, tag TypeTag) (Expr, bool) {
	lhs, lok := literalSet(left)
	rhs, rok := literalSet(right)
	if !lok || !rok {
		return nil, false
	}
	lm, rm := make(map[string]constantLiteral), make(map[string]constantLiteral)
	for _, x := range lhs {
		lm[x.key()] = x
	}
	for _, x := range rhs {
		rm[x.key()] = x
	}
	result := make(map[string]constantLiteral)
	switch operator {
	case "SET_UNION2":
		for k, v := range lm {
			result[k] = v
		}
		for k, v := range rm {
			result[k] = v
		}
	case "SET_INTERSECT":
		for k, v := range lm {
			if _, ok := rm[k]; ok {
				result[k] = v
			}
		}
	case "SET_MINUS":
		for k, v := range lm {
			if _, ok := rm[k]; !ok {
				result[k] = v
			}
		}
	}
	literals := make([]constantLiteral, 0, len(result))
	for _, x := range result {
		literals = append(literals, x)
	}
	sort.Slice(literals, func(i, j int) bool {
		if literals[i].kind != literals[j].kind {
			return literals[i].kind < literals[j].kind
		}
		if literals[i].kind == 0 {
			return !literals[i].boolean && literals[j].boolean
		}
		if literals[i].kind == 1 {
			return literals[i].text < literals[j].text
		}
		return literals[i].integer.Cmp(literals[j].integer) < 0
	})
	args := make([]Expr, len(literals))
	for i, x := range literals {
		if x.kind == 0 {
			args[i] = NewValueExpr(BoolValue(x.boolean), Typed(BoolT1))
		} else if x.kind == 1 {
			args[i] = NewValueExpr(StringValue(x.text), Typed(StrT1))
		} else {
			args[i] = NewValueExpr(NewIntValue(x.integer), Typed(IntT1))
		}
	}
	return MustOperExpr(Op("SET_ENUM"), tag, args...), true
}

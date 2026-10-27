package apalache

import "fmt"

const variantTagField = "__tag"

func isVariantCellType(cellType CellType) bool {
	basic, ok := cellType.(BasicCellType)
	if !ok {
		return false
	}
	_, ok = basic.Type.(VariantType)
	return ok
}

func variantTypeOptions(cellType CellType) (map[string]Type, bool) {
	basic, ok := cellType.(BasicCellType)
	if !ok {
		return nil, false
	}
	variant, ok := basic.Type.(VariantType)
	if !ok {
		return nil, false
	}
	if _, open := variant.Row().Other(); open {
		return nil, false
	}
	return variant.Row().Fields(), true
}

func variantFieldKeys(options map[string]Type) []string {
	fields := make(map[string]Type, len(options)+1)
	for name, typ := range options {
		fields[name] = typ
	}
	fields[variantTagField] = StrT1
	return sortedStringKeys(fields)
}

func variantField(arena *PureArena, variant ArenaCell, name string) (ArenaCell, error) {
	options, ok := variantTypeOptions(variant.Type)
	if !ok {
		return ArenaCell{}, fmt.Errorf("variant access requires a closed variant type, found %s", variant.Type)
	}
	if name != variantTagField {
		if _, found := options[name]; !found {
			return ArenaCell{}, fmt.Errorf("variant option %q is absent from type %s", name, variant.Type)
		}
	}
	keys := variantFieldKeys(options)
	fields := arena.GetHas(variant)
	if len(fields) != len(keys) {
		return ArenaCell{}, fmt.Errorf("variant cell %s contains %d of %d fields", variant, len(fields), len(keys))
	}
	for index, key := range keys {
		if key == name {
			return fields[index].Element(), nil
		}
	}
	return ArenaCell{}, fmt.Errorf("variant field %q is absent from %s", name, variant)
}

func variantTag(arena *PureArena, variant ArenaCell) (ArenaCell, error) {
	return variantField(arena, variant, variantTagField)
}

func variantLiteralArgument(expression Expr) (string, error) {
	valueExpression, ok := expression.(*ValueExpr)
	if !ok {
		return "", fmt.Errorf("variant tag must be a string literal")
	}
	value, ok := valueExpression.Value().(StringValue)
	if !ok {
		return "", fmt.Errorf("variant tag must be a string literal")
	}
	return string(value), nil
}

func (rewriter *StratifiedRewriter) materializeVariantTag(scope RewriterScope, name string) (RewriterScope, ArenaCell, error) {
	return rewriter.Rewrite(NewValueExpr(StringValue(name), Typed(StrT1)), scope)
}

func (rewriter *StratifiedRewriter) rewriteVariant(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 2 {
		return scope, ArenaCell{}, fmt.Errorf("variant constructor requires a tag and value")
	}
	tagName, err := variantLiteralArgument(arguments[0])
	if err != nil {
		return scope, ArenaCell{}, err
	}
	typ, ok := expression.Tag().Type()
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("variant constructor must be typed")
	}
	variantType, ok := typ.(VariantType)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("variant constructor has non-variant type %s", typ)
	}
	if _, open := variantType.Row().Other(); open {
		return scope, ArenaCell{}, fmt.Errorf("variant constructor requires a closed variant type, found %s", typ)
	}
	options := variantType.Row().Fields()
	optionType, found := options[tagName]
	if !found {
		return scope, ArenaCell{}, fmt.Errorf("variant option %q is absent from type %s", tagName, typ)
	}
	scope, value, err := rewriter.Rewrite(arguments[1], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if !EqualTypes(value.Type.TLAType(), optionType) {
		return scope, ArenaCell{}, fmt.Errorf("variant option %q has value type %s, expected %s", tagName, value.Type, optionType)
	}

	// Materializing every option tag lets lazy equality use the same cached
	// string cells as the Scala encoding without mutating its arena argument.
	tags := make(map[string]ArenaCell, len(options))
	for _, name := range sortedStringKeys(options) {
		scope, tags[name], err = rewriter.materializeVariantTag(scope, name)
		if err != nil {
			return scope, ArenaCell{}, err
		}
	}
	values := make(map[string]ArenaCell, len(options))
	for _, name := range sortedStringKeys(options) {
		if name == tagName {
			values[name] = value
			continue
		}
		scope, values[name], err = rewriter.defaults.GetOrCreate(scope, options[name])
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("default variant option %q: %w", name, err)
		}
	}
	keys := variantFieldKeys(options)
	pointers := make([]ElementPointer, len(keys))
	for index, name := range keys {
		field := values[name]
		if name == variantTagField {
			field = tags[tagName]
		}
		pointers[index] = FixedElementPointer{Cell: field}
	}
	cellType, err := CellTypeFromType(variantType)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	newArena, variant := scope.Arena.AppendCellType(cellType)
	scope.Arena = newArena.AppendHas(variant, pointers...)
	return scope, variant, nil
}

func (rewriter *StratifiedRewriter) rewriteVariantTag(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 1 {
		return scope, ArenaCell{}, fmt.Errorf("variant tag requires one operand")
	}
	scope, variant, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	tag, err := variantTag(scope.Arena, variant)
	return scope, tag, err
}

func (rewriter *StratifiedRewriter) rewriteVariantGetUnsafe(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 2 {
		return scope, ArenaCell{}, fmt.Errorf("unsafe variant access requires a tag and variant")
	}
	name, err := variantLiteralArgument(arguments[0])
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope, variant, err := rewriter.Rewrite(arguments[1], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	value, err := variantField(scope.Arena, variant, name)
	return scope, value, err
}

func (rewriter *StratifiedRewriter) rewriteVariantGetOrElse(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 3 {
		return scope, ArenaCell{}, fmt.Errorf("variant access with default requires a tag, variant, and default")
	}
	name, err := variantLiteralArgument(arguments[0])
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope, variant, err := rewriter.Rewrite(arguments[1], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope, fallback, err := rewriter.Rewrite(arguments[2], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	value, err := variantField(scope.Arena, variant, name)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if !EqualCellTypes(value.Type, fallback.Type) {
		return scope, ArenaCell{}, fmt.Errorf("variant value and default have different types %s and %s", value.Type, fallback.Type)
	}
	tag, err := variantTag(scope.Arena, variant)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	var goal ArenaCell
	scope, goal, err = rewriter.materializeVariantTag(scope, name)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	equal, err := rewriter.equality.Equal(scope.Arena, tag, goal)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	var predicate ArenaCell
	scope, predicate = rewriter.bindBoolean(scope, equal)
	return rewriter.chooseConditional(scope, predicate, value, fallback)
}

func (rewriter *StratifiedRewriter) rewriteVariantFilter(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 2 {
		return scope, ArenaCell{}, fmt.Errorf("variant filter requires a tag and set")
	}
	name, err := variantLiteralArgument(arguments[0])
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope, set, err := rewriter.Rewrite(arguments[1], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	setType, ok := set.Type.TLAType().(SetType)
	if !ok || !isFiniteSetCellType(set.Type) {
		return scope, ArenaCell{}, fmt.Errorf("variant filter requires a finite set of variants, found %s", set.Type)
	}
	variantType, ok := setType.Element().(VariantType)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("variant filter requires a set of variants, found %s", set.Type)
	}
	options := variantType.Row().Fields()
	valueType, found := options[name]
	if !found {
		return scope, ArenaCell{}, fmt.Errorf("variant option %q is absent from type %s", name, variantType)
	}
	var goal ArenaCell
	scope, goal, err = rewriter.materializeVariantTag(scope, name)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	pointers := make([]ElementPointer, 0, len(scope.Arena.GetHas(set)))
	for _, sourcePointer := range scope.Arena.GetHas(set) {
		variant := sourcePointer.Element()
		value, fieldErr := variantField(scope.Arena, variant, name)
		if fieldErr != nil {
			return scope, ArenaCell{}, fieldErr
		}
		tag, fieldErr := variantTag(scope.Arena, variant)
		if fieldErr != nil {
			return scope, ArenaCell{}, fieldErr
		}
		equal, equalErr := rewriter.equality.Equal(scope.Arena, tag, goal)
		if equalErr != nil {
			return scope, ArenaCell{}, equalErr
		}
		pointers = append(pointers, SMTElementPointer{Cell: value,
			Expression: booleanAnd(sourcePointer.ToSMT(), equal)})
	}
	if len(pointers) > 0 {
		pointers, err = MergeElementPointersByCell(pointers)
		if err != nil {
			return scope, ArenaCell{}, err
		}
	}
	resultType := BasicCellType{Type: mustSet(valueType)}
	newArena, result := scope.Arena.AppendCellType(resultType)
	scope.Arena = newArena.AppendHas(result, pointers...)
	rewriter.addMembershipConstraints(scope.Arena, result)
	return scope, result, nil
}

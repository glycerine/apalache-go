package apalache

import (
	"fmt"
	"strings"

	"github.com/glycerine/tlago"
)

var tlagoApalacheBuiltinCalls = map[string]string{
	"Guess":                        "Apalache!Guess",
	"Gen":                          "Apalache!Gen",
	"Expand":                       "Apalache!Expand",
	"MkSeq":                        "Apalache!MkSeq",
	"SetAsFun":                     "Apalache!SetAsFun",
	"Repeat":                       "Apalache!Repeat",
	"ApaFoldSet":                   "Apalache!ApaFoldSet",
	"ApaFoldSeqLeft":               "Apalache!ApaFoldSeqLeft",
	"Distinct":                     "Apalache!Distinct",
	"__ApalacheSeqCapacity":        "ApalacheInternal!__ApalacheSeqCapacity",
	"__NotSupportedByModelChecker": "ApalacheInternal!__NotSupportedByModelChecker",
	"Variant":                      "Variants!Variant",
	"VariantFilter":                "Variants!VariantFilter",
	"VariantTag":                   "Variants!VariantTag",
	"VariantGetOrElse":             "Variants!VariantGetOrElse",
	"VariantGetUnsafe":             "Variants!VariantGetUnsafe",
}

var tlagoNativeVariantDeclarations = map[string]struct{}{
	"Variant": {}, "VariantFilter": {}, "VariantTag": {}, "VariantGetOrElse": {}, "VariantGetUnsafe": {},
}

var tlagoNativeTLCDeclarations = map[string]struct{}{
	":>": {}, "@@": {}, "Print": {}, "PrintT": {}, "Assert": {}, "JavaTime": {},
	"__TLCEmptySet": {}, "TLCGet": {}, "TLCSet": {}, "RandomElement": {}, "ToString": {}, "TLCEval": {},
}

var tlagoInternalBuiltinCalls = map[string]struct{}{
	"__ApalacheSeqCapacity": {}, "__NotSupportedByModelChecker": {},
}

var tlagoOpaqueBuiltinModules = map[string]struct{}{
	"Naturals": {}, "Integers": {}, "Reals": {}, "Sequences": {}, "FiniteSets": {},
	"TLC": {}, "TLCExt": {}, "Apalache": {}, "__apalache_internal": {},
}

var tlagoBuiltinOperatorArguments = map[string]map[int]string{
	"=":           {2: "EQ"},
	"/=":          {2: "NE"},
	"#":           {2: "NE"},
	"+":           {2: "PLUS"},
	"-":           {1: "UNARY_MINUS", 2: "MINUS"},
	"*":           {2: "MULT"},
	"\\div":       {2: "DIV"},
	"%":           {2: "MOD"},
	"^":           {2: "POW"},
	"..":          {2: "INT_RANGE"},
	"<":           {2: "LT"},
	">":           {2: "GT"},
	"<=":          {2: "LE"},
	"=<":          {2: "LE"},
	"\\leq":       {2: "LE"},
	">=":          {2: "GE"},
	"\\geq":       {2: "GE"},
	"~":           {1: "NOT"},
	"\\lnot":      {1: "NOT"},
	"\\in":        {2: "SET_IN"},
	"\\notin":     {2: "SET_NOT_IN"},
	"\\union":     {2: "SET_UNION2"},
	"\\cup":       {2: "SET_UNION2"},
	"\\intersect": {2: "SET_INTERSECT"},
	"\\cap":       {2: "SET_INTERSECT"},
	"\\subseteq":  {2: "SET_SUBSET_EQ"},
	"\\":          {2: "SET_MINUS"},
	"\\X":         {2: "SET_TIMES"},
	"\\times":     {2: "SET_TIMES"},
	"SUBSET":      {1: "SET_POWERSET"},
	"UNION":       {1: "SET_UNARY_UNION"},
	"DOMAIN":      {1: "DOMAIN"},
}

type tlagoBinderGroup struct {
	names   []string
	isTuple bool
}

type tlagoBinderLayout struct {
	position tlago.Position
	operator string
	groups   []tlagoBinderGroup
}

// normalizeTlagoRootLocalInstances retains definitions imported by a LOCAL
// INSTANCE. The exported IR is already a flattened root module, so retaining
// these root-visible definitions cannot expose them to another module.
func normalizeTlagoRootLocalInstances(spec *tlago.Spec) {
	if spec == nil || spec.Root == nil {
		return
	}
	for index := range spec.Root.Instances {
		spec.Root.Instances[index].Local = false
	}
}

// normalizeTlagoInstanceExtendedDefinitions supplies definitions inherited by
// a root INSTANCE target. tlago v0.1.9 exports only the target's direct
// definitions, although those definitions may refer to operators from EXTENDS.
func normalizeTlagoInstanceExtendedDefinitions(spec *tlago.Spec) {
	if spec == nil || spec.Root == nil {
		return
	}
	for _, instance := range spec.Root.Instances {
		target := spec.Modules[instance.Module]
		if target == nil {
			continue
		}
		existing := make(map[string]struct{}, len(target.Definitions))
		for _, definition := range target.Definitions {
			existing[definition.Name] = struct{}{}
		}
		var inherited []tlago.Definition
		seen := make(map[string]bool)
		var visit func(string)
		visit = func(name string) {
			if name == "" || seen[name] {
				return
			}
			seen[name] = true
			if _, opaque := tlagoOpaqueBuiltinModules[name]; opaque {
				return
			}
			module := spec.Modules[name]
			if module == nil {
				return
			}
			for _, parent := range module.Extends {
				visit(parent)
			}
			for _, definition := range module.Definitions {
				if definition.Local || definition.TheoremLike {
					continue
				}
				if _, found := existing[definition.Name]; found {
					continue
				}
				existing[definition.Name] = struct{}{}
				inherited = append(inherited, definition)
			}
		}
		for _, parent := range target.Extends {
			visit(parent)
		}
		if len(inherited) != 0 {
			target.Definitions = append(inherited, target.Definitions...)
		}
	}
}

// normalizeTlagoLambdas works around tlago v0.1.9's ApalacheIR exporter
// treating a domainless LAMBDA like a function constructor. An operator-valued
// lambda is equivalently represented as a local operator definition whose
// name is the value of the surrounding LET expression.
func normalizeTlagoLambdas(spec *tlago.Spec) {
	if spec == nil || spec.Root == nil {
		return
	}
	next := 0
	for index := range spec.Root.Definitions {
		spec.Root.Definitions[index].Expr = normalizeTlagoLambdaExpr(spec.Root.Definitions[index].Expr, &next)
	}
	for index := range spec.Root.Assumptions {
		spec.Root.Assumptions[index].Expr = normalizeTlagoLambdaExpr(spec.Root.Assumptions[index].Expr, &next)
	}
	for index := range spec.Root.Theorems {
		spec.Root.Theorems[index].Expr = normalizeTlagoLambdaExpr(spec.Root.Theorems[index].Expr, &next)
	}
}

func normalizeTlagoLambdaExpr(expression tlago.Expr, next *int) tlago.Expr {
	if expression == nil {
		return nil
	}
	transformAll := func(expressions []tlago.Expr) {
		for index := range expressions {
			expressions[index] = normalizeTlagoLambdaExpr(expressions[index], next)
		}
	}
	switch typed := expression.(type) {
	case *tlago.IdentExpr, *tlago.LiteralExpr:
	case *tlago.UnaryExpr:
		typed.Expr = normalizeTlagoLambdaExpr(typed.Expr, next)
	case *tlago.BinaryExpr:
		typed.Left = normalizeTlagoLambdaExpr(typed.Left, next)
		typed.Right = normalizeTlagoLambdaExpr(typed.Right, next)
		if typed.Op == "@@" {
			return lowerTlagoFunctionOverride(typed, next)
		}
		if typed.Op == ":>" {
			return lowerTlagoSingletonFunction(typed, next)
		}
		if typed.Op == "\\oplus" {
			return lowerTlagoBagUnion(typed, next)
		}
		if typed.Op == "\\ominus" {
			return lowerTlagoBagDifference(typed, next)
		}
		if typed.Op == "\\sqsubseteq" {
			return lowerTlagoSubBag(typed, next)
		}
	case *tlago.CallExpr:
		typed.Callee = normalizeTlagoLambdaExpr(typed.Callee, next)
		transformAll(typed.Args)
	case *tlago.IfExpr:
		typed.Cond = normalizeTlagoLambdaExpr(typed.Cond, next)
		typed.Then = normalizeTlagoLambdaExpr(typed.Then, next)
		typed.Else = normalizeTlagoLambdaExpr(typed.Else, next)
	case *tlago.LetExpr:
		for index := range typed.Definitions {
			typed.Definitions[index].Expr = normalizeTlagoLambdaExpr(typed.Definitions[index].Expr, next)
		}
		for index := range typed.Instances {
			for substitutionIndex := range typed.Instances[index].SubstitutionList {
				substitution := &typed.Instances[index].SubstitutionList[substitutionIndex]
				substitution.Expr = normalizeTlagoLambdaExpr(substitution.Expr, next)
			}
		}
		typed.Body = normalizeTlagoLambdaExpr(typed.Body, next)
	case *tlago.QuantifierExpr:
		typed.Set = normalizeTlagoLambdaExpr(typed.Set, next)
		typed.Body = normalizeTlagoLambdaExpr(typed.Body, next)
	case *tlago.CaseExpr:
		for index := range typed.Arms {
			typed.Arms[index].Test = normalizeTlagoLambdaExpr(typed.Arms[index].Test, next)
			typed.Arms[index].Value = normalizeTlagoLambdaExpr(typed.Arms[index].Value, next)
		}
		typed.Other = normalizeTlagoLambdaExpr(typed.Other, next)
	case *tlago.ChooseExpr:
		typed.Set = normalizeTlagoLambdaExpr(typed.Set, next)
		typed.Body = normalizeTlagoLambdaExpr(typed.Body, next)
	case *tlago.TupleExpr:
		transformAll(typed.Elems)
	case *tlago.SetExpr:
		transformAll(typed.Elems)
	case *tlago.RecordExpr:
		for index := range typed.Fields {
			typed.Fields[index].Value = normalizeTlagoLambdaExpr(typed.Fields[index].Value, next)
		}
	case *tlago.RecordComponentExpr:
		typed.Record = normalizeTlagoLambdaExpr(typed.Record, next)
	case *tlago.RecordSetExpr:
		for index := range typed.Fields {
			typed.Fields[index].Set = normalizeTlagoLambdaExpr(typed.Fields[index].Set, next)
		}
	case *tlago.FunctionExpr:
		for index := range typed.Bounds {
			typed.Bounds[index].Set = normalizeTlagoLambdaExpr(typed.Bounds[index].Set, next)
		}
		typed.Body = normalizeTlagoLambdaExpr(typed.Body, next)
		if typed.IsLambda {
			*next++
			name := fmt.Sprintf("__tlago_lambda_%d", *next)
			parameters := make([]string, len(typed.Bounds))
			arities := make(map[string]int, len(typed.Bounds))
			positions := make(map[string]tlago.Position, len(typed.Bounds))
			for index, bound := range typed.Bounds {
				parameters[index] = bound.Name
				arities[bound.Name] = 0
				positions[bound.Name] = bound.Pos
			}
			definition := tlago.Definition{
				Name: name, Params: parameters, ParamArities: arities, ParamPositions: positions,
				Expr: typed.Body, Pos: typed.Pos, Source: typed.Pos, LHSPos: typed.Pos,
			}
			return &tlago.LetExpr{
				Definitions: []tlago.Definition{definition},
				Body:        &tlago.IdentExpr{Name: name, Pos: typed.Pos}, Pos: typed.Pos,
			}
		}
	case *tlago.FunctionAppExpr:
		typed.Function = normalizeTlagoLambdaExpr(typed.Function, next)
		transformAll(typed.Args)
	case *tlago.ExceptExpr:
		typed.Base = normalizeTlagoLambdaExpr(typed.Base, next)
		for specIndex := range typed.Specs {
			for componentIndex := range typed.Specs[specIndex].Components {
				transformAll(typed.Specs[specIndex].Components[componentIndex].Indices)
			}
			if nested, ok := typed.Specs[specIndex].Value.(*tlago.ExceptExpr); ok {
				if base, isAt := nested.Base.(*tlago.IdentExpr); isAt && base.Name == "@" {
					nested.Base = tlagoExceptOldValue(typed.Base, typed.Specs[specIndex])
				}
			}
			typed.Specs[specIndex].Value = normalizeTlagoLambdaExpr(typed.Specs[specIndex].Value, next)
		}
	case *tlago.LabelExpr:
		typed.Body = normalizeTlagoLambdaExpr(typed.Body, next)
	case *tlago.ActionExpr:
		typed.Action = normalizeTlagoLambdaExpr(typed.Action, next)
		typed.Subscript = normalizeTlagoLambdaExpr(typed.Subscript, next)
	case *tlago.FairnessExpr:
		typed.Subscript = normalizeTlagoLambdaExpr(typed.Subscript, next)
		typed.Action = normalizeTlagoLambdaExpr(typed.Action, next)
	case *tlago.FunctionSetExpr:
		typed.Domain = normalizeTlagoLambdaExpr(typed.Domain, next)
		typed.Range = normalizeTlagoLambdaExpr(typed.Range, next)
	case *tlago.SetComprehensionExpr:
		typed.Element = normalizeTlagoLambdaExpr(typed.Element, next)
		for index := range typed.Bounds {
			typed.Bounds[index].Set = normalizeTlagoLambdaExpr(typed.Bounds[index].Set, next)
		}
		typed.Predicate = normalizeTlagoLambdaExpr(typed.Predicate, next)
		if lowered, ok := lowerTlagoTupleSetFilter(typed, next); ok {
			return lowered
		}
	}
	return expression
}

func tlagoExceptOldValue(base tlago.Expr, specification tlago.ExceptSpec) tlago.Expr {
	current := base
	for _, component := range specification.Components {
		var path []tlago.Expr
		if component.Field != "" {
			path = []tlago.Expr{&tlago.LiteralExpr{Kind: "string", Value: component.Field, Pos: component.FieldPos}}
		} else if len(component.Indices) == 1 {
			path = []tlago.Expr{component.Indices[0]}
		} else if len(component.Indices) > 1 {
			path = []tlago.Expr{&tlago.TupleExpr{Elems: append([]tlago.Expr(nil), component.Indices...), Pos: component.Pos}}
		}
		for _, index := range path {
			current = &tlago.FunctionAppExpr{Function: current, Args: []tlago.Expr{index}, Pos: specification.Pos}
		}
	}
	return current
}

func lowerTlagoFunctionOverride(expression *tlago.BinaryExpr, next *int) tlago.Expr {
	*next++
	name := fmt.Sprintf("__tlago_override_arg_%d", *next)
	bound := func() tlago.Expr { return &tlago.IdentExpr{Name: name, Pos: expression.Pos} }
	domain := func(function tlago.Expr) tlago.Expr {
		return &tlago.UnaryExpr{Op: "DOMAIN", Expr: function, Pos: expression.Pos}
	}
	application := func(function tlago.Expr) tlago.Expr {
		return &tlago.FunctionAppExpr{Function: function, Args: []tlago.Expr{bound()}, Pos: expression.Pos}
	}
	combinedDomain := &tlago.BinaryExpr{
		Op: "\\union", Left: domain(expression.Left), Right: domain(expression.Right), Pos: expression.Pos,
	}
	inLeftDomain := &tlago.BinaryExpr{
		Op: "\\in", Left: bound(), Right: domain(expression.Left), Pos: expression.Pos,
	}
	body := &tlago.IfExpr{
		Cond: inLeftDomain, Then: application(expression.Left), Else: application(expression.Right), Pos: expression.Pos,
	}
	return &tlago.FunctionExpr{
		Bounds: []tlago.BoundVar{{Name: name, Set: combinedDomain, Pos: expression.Pos}},
		Body:   body, Pos: expression.Pos,
	}
}

func lowerTlagoSingletonFunction(expression *tlago.BinaryExpr, next *int) tlago.Expr {
	*next++
	name := fmt.Sprintf("__tlago_singleton_arg_%d", *next)
	domain := &tlago.SetExpr{Elems: []tlago.Expr{expression.Left}, Pos: expression.Pos}
	return &tlago.FunctionExpr{
		Bounds: []tlago.BoundVar{{Name: name, Set: domain, Pos: expression.Pos}},
		Body:   expression.Right, Pos: expression.Pos,
	}
}

func lowerTlagoBagUnion(expression *tlago.BinaryExpr, next *int) tlago.Expr {
	*next++
	name := fmt.Sprintf("__tlago_bag_union_arg_%d", *next)
	bound := func() tlago.Expr { return &tlago.IdentExpr{Name: name, Pos: expression.Pos} }
	domain := func(function tlago.Expr) tlago.Expr {
		return &tlago.UnaryExpr{Op: "DOMAIN", Expr: function, Pos: expression.Pos}
	}
	count := func(function tlago.Expr) tlago.Expr {
		member := &tlago.BinaryExpr{Op: "\\in", Left: bound(), Right: domain(function), Pos: expression.Pos}
		application := &tlago.FunctionAppExpr{Function: function, Args: []tlago.Expr{bound()}, Pos: expression.Pos}
		return &tlago.IfExpr{Cond: member, Then: application,
			Else: &tlago.LiteralExpr{Kind: "number", Value: "0", Pos: expression.Pos}, Pos: expression.Pos}
	}
	combinedDomain := &tlago.BinaryExpr{
		Op: "\\union", Left: domain(expression.Left), Right: domain(expression.Right), Pos: expression.Pos,
	}
	body := &tlago.BinaryExpr{Op: "+", Left: count(expression.Left), Right: count(expression.Right), Pos: expression.Pos}
	return &tlago.FunctionExpr{
		Bounds: []tlago.BoundVar{{Name: name, Set: combinedDomain, Pos: expression.Pos}},
		Body:   body, Pos: expression.Pos,
	}
}

func lowerTlagoBagDifference(expression *tlago.BinaryExpr, next *int) tlago.Expr {
	*next++
	name := fmt.Sprintf("__tlago_bag_difference_arg_%d", *next)
	bound := func() tlago.Expr { return &tlago.IdentExpr{Name: name, Pos: expression.Pos} }
	domain := func(function tlago.Expr) tlago.Expr {
		return &tlago.UnaryExpr{Op: "DOMAIN", Expr: function, Pos: expression.Pos}
	}
	application := func(function tlago.Expr) tlago.Expr {
		return &tlago.FunctionAppExpr{Function: function, Args: []tlago.Expr{bound()}, Pos: expression.Pos}
	}
	count := func() tlago.Expr {
		inRight := &tlago.BinaryExpr{Op: "\\in", Left: bound(), Right: domain(expression.Right), Pos: expression.Pos}
		difference := &tlago.BinaryExpr{Op: "-", Left: application(expression.Left), Right: application(expression.Right), Pos: expression.Pos}
		return &tlago.IfExpr{Cond: inRight, Then: difference, Else: application(expression.Left), Pos: expression.Pos}
	}
	positive := &tlago.BinaryExpr{
		Op: ">", Left: count(), Right: &tlago.LiteralExpr{Kind: "number", Value: "0", Pos: expression.Pos}, Pos: expression.Pos,
	}
	filteredDomain := &tlago.SetComprehensionExpr{
		Element: bound(), Bounds: []tlago.BoundVar{{Name: name, Set: domain(expression.Left), Pos: expression.Pos}},
		Predicate: positive, Pos: expression.Pos,
	}
	return &tlago.FunctionExpr{
		Bounds: []tlago.BoundVar{{Name: name, Set: filteredDomain, Pos: expression.Pos}},
		Body:   count(), Pos: expression.Pos,
	}
}

func lowerTlagoSubBag(expression *tlago.BinaryExpr, next *int) tlago.Expr {
	*next++
	name := fmt.Sprintf("__tlago_subbag_arg_%d", *next)
	bound := func() tlago.Expr { return &tlago.IdentExpr{Name: name, Pos: expression.Pos} }
	domain := func(function tlago.Expr) tlago.Expr {
		return &tlago.UnaryExpr{Op: "DOMAIN", Expr: function, Pos: expression.Pos}
	}
	application := func(function tlago.Expr) tlago.Expr {
		return &tlago.FunctionAppExpr{Function: function, Args: []tlago.Expr{bound()}, Pos: expression.Pos}
	}
	domainSubset := &tlago.BinaryExpr{
		Op: "\\subseteq", Left: domain(expression.Left), Right: domain(expression.Right), Pos: expression.Pos,
	}
	countsOrdered := &tlago.QuantifierExpr{
		Kind: "\\A", Var: name, Set: domain(expression.Left),
		Body: &tlago.BinaryExpr{Op: "\\leq", Left: application(expression.Left), Right: application(expression.Right), Pos: expression.Pos},
		Pos:  expression.Pos,
	}
	return &tlago.BinaryExpr{Op: "/\\", Left: domainSubset, Right: countsOrdered, Pos: expression.Pos}
}

func lowerTlagoTupleSetFilter(expression *tlago.SetComprehensionExpr, next *int) (tlago.Expr, bool) {
	if expression == nil || expression.Predicate == nil || len(expression.Bounds) < 2 {
		return nil, false
	}
	tuple, ok := expression.Element.(*tlago.TupleExpr)
	if !ok || len(tuple.Elems) != len(expression.Bounds) {
		return nil, false
	}
	domain := expression.Bounds[0].Set
	for index, bound := range expression.Bounds {
		name, named := tuple.Elems[index].(*tlago.IdentExpr)
		if !named || name.Name != bound.Name || !bound.TupleBound || bound.Set != domain {
			return nil, false
		}
	}
	*next++
	tupleName := fmt.Sprintf("__tlago_tuple_filter_%d", *next)
	definitions := make([]tlago.Definition, len(expression.Bounds))
	for index, bound := range expression.Bounds {
		projection := &tlago.FunctionAppExpr{
			Function: &tlago.IdentExpr{Name: tupleName, Pos: expression.Pos},
			Args:     []tlago.Expr{&tlago.LiteralExpr{Kind: "number", Value: fmt.Sprintf("%d", index+1), Pos: bound.Pos}},
			Pos:      bound.Pos,
		}
		definitions[index] = tlago.Definition{
			Name: bound.Name, ParamArities: map[string]int{}, Expr: projection,
			Pos: bound.Pos, Source: bound.Pos, LHSPos: bound.Pos,
		}
	}
	predicate := &tlago.LetExpr{Definitions: definitions, Body: expression.Predicate, Pos: expression.Pos}
	return &tlago.SetComprehensionExpr{
		Element:   &tlago.IdentExpr{Name: tupleName, Pos: expression.Pos},
		Bounds:    []tlago.BoundVar{{Name: tupleName, Set: domain, Pos: expression.Pos}},
		Predicate: predicate,
		Pos:       expression.Pos,
	}, true
}

func normalizeTlagoLambdaValues(module *Module, sources *SourceStore) *Module {
	if module == nil {
		return nil
	}
	var tracker TransformationTracker = IdleTracker{}
	if sources != nil {
		tracker = NewListenerTracker(sources)
	}
	rewrite := RewriteExpressions(tracker, func(expression Expr) (Expr, bool) {
		let, ok := expression.(*LetInExpr)
		if !ok || len(let.Declarations()) != 1 {
			return nil, false
		}
		declaration := let.Declarations()[0]
		if len(declaration.FormalParams()) == 0 {
			return nil, false
		}
		application, ok := let.Body().(*OperExpr)
		if !ok || application.Operator().Name() != "OPER_APP" {
			return nil, false
		}
		arguments := application.Arguments()
		if len(arguments) != 1 {
			return nil, false
		}
		name, ok := arguments[0].(*NameExpr)
		if !ok || name.Name() != declaration.Name() || !strings.HasPrefix(name.Name(), "__tlago_lambda_") {
			return nil, false
		}
		return NewLetInExpr(NewNameExpr(name.Name(), name.Tag()), let.Tag(), declaration), true
	})
	return transformTlagoModuleExpressions(module, rewrite, sources)
}

// normalizeTlagoRecursiveFunctions reconstructs FUN_REC_CTOR/FUN_REC_REF.
// tlago v0.1.9 exports a recursive function as an ordinary constructor whose
// body calls the enclosing nullary declaration by name.
func normalizeTlagoRecursiveFunctions(module *Module, sources *SourceStore) *Module {
	if module == nil {
		return nil
	}
	var tracker TransformationTracker = IdleTracker{}
	if sources != nil {
		tracker = NewListenerTracker(sources)
	}
	declarations := module.Declarations()
	changedModule := false
	for index, declaration := range declarations {
		operator, ok := declaration.(*OperDecl)
		if !ok || len(operator.FormalParams()) != 0 {
			continue
		}
		constructor, ok := operator.Body().(*OperExpr)
		if !ok || constructor.Operator().Name() != "FUN_CTOR" {
			continue
		}
		foundReference := false
		rewrite := RewriteExpressions(tracker, func(expression Expr) (Expr, bool) {
			application, ok := expression.(*OperExpr)
			if !ok || application.Operator().Name() != "OPER_APP" {
				return nil, false
			}
			arguments := application.Arguments()
			if len(arguments) != 1 {
				return nil, false
			}
			name, named := arguments[0].(*NameExpr)
			if !named || name.Name() != operator.Name() {
				return nil, false
			}
			foundReference = true
			return MustOperExpr(Op("FUN_REC_REF"), application.Tag()), true
		})
		arguments := constructor.Arguments()
		arguments[0] = rewrite(arguments[0])
		if !foundReference {
			continue
		}
		recursive := MustOperExpr(Op("FUN_REC_CTOR"), constructor.Tag(), arguments...)
		copy := operator.copyWithTag(operator.Tag())
		copy.SetBody(recursive)
		declarations[index] = tracker.TrackOperatorDeclaration(func(*OperDecl) *OperDecl { return copy })(operator)
		changedModule = true
	}
	if !changedModule {
		return module
	}
	result := NewModule(module.Name(), declarations)
	result.properties = cloneMap(module.properties)
	return result
}

// normalizeTlagoOperatorValues repairs a non-nullary operator passed as a
// higher-order argument. tlago v0.1.9 wraps such a name in a zero-argument
// OPER_APP, which is never a valid call to a non-nullary definition.
func normalizeTlagoOperatorValues(module *Module, sources *SourceStore) *Module {
	if module == nil {
		return nil
	}
	arities := make(map[string]map[int]struct{})
	add := func(declaration *OperDecl) {
		byArity := arities[declaration.Name()]
		if byArity == nil {
			byArity = make(map[int]struct{})
			arities[declaration.Name()] = byArity
		}
		byArity[len(declaration.FormalParams())] = struct{}{}
	}
	for _, declaration := range module.OperDeclarations() {
		add(declaration)
	}
	collect := RewriteExpressions(IdleTracker{}, func(expression Expr) (Expr, bool) {
		if let, ok := expression.(*LetInExpr); ok {
			for _, declaration := range let.Declarations() {
				add(declaration)
			}
		}
		return nil, false
	})
	for _, declaration := range module.Declarations() {
		switch typed := declaration.(type) {
		case *OperDecl:
			collect(typed.Body())
		case *AssumeDecl:
			collect(typed.Body())
		}
	}
	var tracker TransformationTracker = IdleTracker{}
	if sources != nil {
		tracker = NewListenerTracker(sources)
	}
	rewrite := RewriteExpressions(tracker, func(expression Expr) (Expr, bool) {
		operation, ok := expression.(*OperExpr)
		if !ok || operation.Operator().Name() != "OPER_APP" {
			return nil, false
		}
		arguments := operation.Arguments()
		if len(arguments) != 1 {
			return nil, false
		}
		name, named := arguments[0].(*NameExpr)
		if !named {
			return nil, false
		}
		byArity := arities[name.Name()]
		if len(byArity) == 0 {
			return nil, false
		}
		if _, nullary := byArity[0]; nullary {
			return nil, false
		}
		return name, true
	})
	return transformTlagoModuleExpressions(module, rewrite, sources)
}

// normalizeTlagoNamedAssumptionReferences repairs named ASSUME references
// that tlago exports as first-class (() => Bool) values. A named assumption
// denotes its formula body, so inline a fresh copy at every reference.
func normalizeTlagoNamedAssumptionReferences(module *Module, sources *SourceStore) *Module {
	if module == nil {
		return nil
	}
	namedAssumptions := make(map[string]Expr)
	for _, declaration := range module.AssumeDeclarations() {
		if name, named := declaration.DefinedName(); named {
			namedAssumptions[name] = declaration.Body()
		}
	}
	var tracker TransformationTracker = IdleTracker{}
	if sources != nil {
		tracker = NewListenerTracker(sources)
	}
	rewrite := RewriteExpressions(tracker, func(expression Expr) (Expr, bool) {
		name, ok := expression.(*NameExpr)
		if !ok {
			return nil, false
		}
		if body, assumption := namedAssumptions[name.Name()]; assumption {
			return NewDeepCopier(tracker).Expression(body), true
		}
		return nil, false
	})
	return transformTlagoModuleExpressions(module, rewrite, sources)
}

func removeTlagoNativeVariantDeclarations(module *Module, sources *SourceStore, variantsImported bool) *Module {
	if module == nil || !variantsImported {
		return module
	}
	declarations := module.Declarations()
	kept := make([]Decl, 0, len(declarations))
	for _, declaration := range declarations {
		_, native := tlagoNativeVariantDeclarations[declaration.Name()]
		if native {
			continue
		}
		kept = append(kept, declaration)
	}
	result := NewModule(module.Name(), kept)
	result.properties = cloneMap(module.properties)
	return result
}

// normalizeTlagoTLCCalls uses the typed TLC rewiring module for SANY, then
// removes its executable stubs before native type checking. Print-like
// operators are pure erasures in Apalache; stateful register reads remain an
// explicit unsupported native operator instead of acquiring CHOOSE semantics.
func normalizeTlagoTLCCalls(module *Module, sources *SourceStore, tlcImported bool) *Module {
	if module == nil || !tlcImported {
		return module
	}
	var tracker TransformationTracker = IdleTracker{}
	if sources != nil {
		tracker = NewListenerTracker(sources)
	}
	baseName := func(name string) string {
		if index := strings.LastIndex(name, "!"); index >= 0 {
			return name[index+1:]
		}
		return name
	}
	rewrite := RewriteExpressions(tracker, func(expression Expr) (Expr, bool) {
		if name, ok := expression.(*NameExpr); ok && baseName(name.Name()) == "JavaTime" {
			return NewValueExpr(Int64Value(123), Typed(IntT1)), true
		}
		operation, ok := expression.(*OperExpr)
		if !ok || operation.Operator().Name() != "OPER_APP" {
			return nil, false
		}
		arguments := operation.Arguments()
		if len(arguments) == 0 {
			return nil, false
		}
		callee, ok := arguments[0].(*NameExpr)
		if !ok {
			return nil, false
		}
		switch baseName(callee.Name()) {
		case "Print":
			if len(arguments) == 3 {
				return arguments[2], true
			}
		case "PrintT", "TLCSet":
			return NewValueExpr(BoolValue(true), Typed(BoolT1)), true
		case "Assert":
			if len(arguments) == 3 {
				return arguments[1], true
			}
		case "TLCGet":
			if len(arguments) == 2 {
				return MustOperExpr(Op("TLC!TLCGet"), operation.Tag(), arguments[1]), true
			}
		case "RandomElement":
			if len(arguments) == 2 {
				return MustOperExpr(Op("Apalache!Guess"), operation.Tag(), arguments[1]), true
			}
		case "ToString":
			if len(arguments) == 2 {
				return MustOperExpr(Op("TLC!ToString"), operation.Tag(), arguments[1]), true
			}
		case "TLCEval":
			if len(arguments) == 2 {
				return arguments[1], true
			}
		}
		return nil, false
	})
	transformed := transformTlagoModuleExpressions(module, rewrite, sources)
	declarations := transformed.Declarations()
	kept := make([]Decl, 0, len(declarations))
	for _, declaration := range declarations {
		if _, native := tlagoNativeTLCDeclarations[baseName(declaration.Name())]; native {
			continue
		}
		kept = append(kept, declaration)
	}
	result := NewModule(transformed.Name(), kept)
	result.properties = cloneMap(transformed.properties)
	return result
}

func normalizeTlagoVariantUnit(module *Module, sources *SourceStore, variantsImported bool) *Module {
	if module == nil || !variantsImported {
		return module
	}
	for _, declaration := range module.Declarations() {
		if declaration.Name() == "UNIT" {
			return module
		}
	}
	var tracker TransformationTracker = IdleTracker{}
	if sources != nil {
		tracker = NewListenerTracker(sources)
	}
	rewrite := RewriteExpressions(tracker, func(expression Expr) (Expr, bool) {
		name, ok := expression.(*NameExpr)
		if !ok || name.Name() != "UNIT" {
			return nil, false
		}
		return NewValueExpr(StringValue("U_OF_UNIT"), name.Tag()), true
	})
	return transformTlagoModuleExpressions(module, rewrite, sources)
}

// normalizeTlagoApalacheAssignments works around tlago v0.1.9's ApalacheIR
// exporter rejecting the imported infix := operator. Replacing it by its
// standard-library definition is semantics-preserving: x := e is defined as
// x = e, and the assignment-analysis pass subsequently recovers assignments.
// It also retains tuple-function-binder metadata that the exporter drops.
func normalizeTlagoApalacheAssignments(spec *tlago.Spec) []tlagoBinderLayout {
	if spec == nil || spec.Root == nil {
		return nil
	}
	var binderLayouts []tlagoBinderLayout
	for index := range spec.Root.Definitions {
		normalizeTlagoAssignmentExpr(spec.Root.Definitions[index].Expr, &binderLayouts)
	}
	for index := range spec.Root.Assumptions {
		normalizeTlagoAssignmentExpr(spec.Root.Assumptions[index].Expr, &binderLayouts)
	}
	for index := range spec.Root.Theorems {
		normalizeTlagoAssignmentExpr(spec.Root.Theorems[index].Expr, &binderLayouts)
	}
	return binderLayouts
}

func normalizeTlagoAssignmentExpr(expression tlago.Expr, binderLayouts *[]tlagoBinderLayout) {
	if expression == nil {
		return
	}
	switch typed := expression.(type) {
	case *tlago.IdentExpr, *tlago.LiteralExpr:
	case *tlago.UnaryExpr:
		normalizeTlagoAssignmentExpr(typed.Expr, binderLayouts)
	case *tlago.BinaryExpr:
		if typed.Op == ":=" {
			typed.Op = "="
		}
		normalizeTlagoAssignmentExpr(typed.Left, binderLayouts)
		normalizeTlagoAssignmentExpr(typed.Right, binderLayouts)
	case *tlago.CallExpr:
		normalizeTlagoAssignmentExpr(typed.Callee, binderLayouts)
		normalizeTlagoAssignmentExprs(typed.Args, binderLayouts)
	case *tlago.IfExpr:
		normalizeTlagoAssignmentExpr(typed.Cond, binderLayouts)
		normalizeTlagoAssignmentExpr(typed.Then, binderLayouts)
		normalizeTlagoAssignmentExpr(typed.Else, binderLayouts)
	case *tlago.LetExpr:
		for index := range typed.Definitions {
			normalizeTlagoAssignmentExpr(typed.Definitions[index].Expr, binderLayouts)
		}
		for index := range typed.Instances {
			for _, substitution := range typed.Instances[index].SubstitutionList {
				normalizeTlagoAssignmentExpr(substitution.Expr, binderLayouts)
			}
		}
		normalizeTlagoAssignmentExpr(typed.Body, binderLayouts)
	case *tlago.QuantifierExpr:
		normalizeTlagoAssignmentExpr(typed.Set, binderLayouts)
		normalizeTlagoAssignmentExpr(typed.Body, binderLayouts)
	case *tlago.CaseExpr:
		for _, arm := range typed.Arms {
			normalizeTlagoAssignmentExpr(arm.Test, binderLayouts)
			normalizeTlagoAssignmentExpr(arm.Value, binderLayouts)
		}
		normalizeTlagoAssignmentExpr(typed.Other, binderLayouts)
	case *tlago.ChooseExpr:
		normalizeTlagoAssignmentExpr(typed.Set, binderLayouts)
		normalizeTlagoAssignmentExpr(typed.Body, binderLayouts)
	case *tlago.TupleExpr:
		normalizeTlagoAssignmentExprs(typed.Elems, binderLayouts)
	case *tlago.SetExpr:
		normalizeTlagoAssignmentExprs(typed.Elems, binderLayouts)
	case *tlago.RecordExpr:
		for _, field := range typed.Fields {
			normalizeTlagoAssignmentExpr(field.Value, binderLayouts)
		}
	case *tlago.RecordComponentExpr:
		normalizeTlagoAssignmentExpr(typed.Record, binderLayouts)
	case *tlago.RecordSetExpr:
		for _, field := range typed.Fields {
			normalizeTlagoAssignmentExpr(field.Set, binderLayouts)
		}
	case *tlago.FunctionExpr:
		*binderLayouts = append(*binderLayouts, collectTlagoBinderLayout(typed.Pos, typed.Bounds, "FUN_CTOR"))
		for _, bound := range typed.Bounds {
			normalizeTlagoAssignmentExpr(bound.Set, binderLayouts)
		}
		normalizeTlagoAssignmentExpr(typed.Body, binderLayouts)
	case *tlago.FunctionAppExpr:
		normalizeTlagoAssignmentExpr(typed.Function, binderLayouts)
		normalizeTlagoAssignmentExprs(typed.Args, binderLayouts)
	case *tlago.ExceptExpr:
		normalizeTlagoAssignmentExpr(typed.Base, binderLayouts)
		for _, spec := range typed.Specs {
			for _, component := range spec.Components {
				normalizeTlagoAssignmentExprs(component.Indices, binderLayouts)
			}
			normalizeTlagoAssignmentExpr(spec.Value, binderLayouts)
		}
	case *tlago.LabelExpr:
		normalizeTlagoAssignmentExpr(typed.Body, binderLayouts)
	case *tlago.ActionExpr:
		normalizeTlagoAssignmentExpr(typed.Action, binderLayouts)
		normalizeTlagoAssignmentExpr(typed.Subscript, binderLayouts)
	case *tlago.FairnessExpr:
		normalizeTlagoAssignmentExpr(typed.Subscript, binderLayouts)
		normalizeTlagoAssignmentExpr(typed.Action, binderLayouts)
	case *tlago.FunctionSetExpr:
		normalizeTlagoAssignmentExpr(typed.Domain, binderLayouts)
		normalizeTlagoAssignmentExpr(typed.Range, binderLayouts)
	case *tlago.SetComprehensionExpr:
		if typed.Predicate == nil {
			*binderLayouts = append(*binderLayouts, collectTlagoBinderLayout(typed.Pos, typed.Bounds, "SET_MAP"))
		}
		normalizeTlagoAssignmentExpr(typed.Element, binderLayouts)
		for _, bound := range typed.Bounds {
			normalizeTlagoAssignmentExpr(bound.Set, binderLayouts)
		}
		normalizeTlagoAssignmentExpr(typed.Predicate, binderLayouts)
	}
}

func normalizeTlagoAssignmentExprs(expressions []tlago.Expr, binderLayouts *[]tlagoBinderLayout) {
	for _, expression := range expressions {
		normalizeTlagoAssignmentExpr(expression, binderLayouts)
	}
}

func collectTlagoBinderLayout(position tlago.Position, bounds []tlago.BoundVar, operator string) tlagoBinderLayout {
	result := tlagoBinderLayout{position: position, operator: operator}
	for index := 0; index < len(bounds); {
		bound := bounds[index]
		group := tlagoBinderGroup{names: []string{bound.Name}, isTuple: bound.TupleBound}
		index++
		if bound.TupleBound {
			for index < len(bounds) && bounds[index].TupleBound && bounds[index].Set == bound.Set {
				group.names = append(group.names, bounds[index].Name)
				index++
			}
		}
		result.groups = append(result.groups, group)
	}
	return result
}

// normalizeTlagoTupleBinders reconstructs tuple binders that tlago v0.1.9
// flattens while exporting FUN_CTOR and SET_MAP. Unlike an IR-only heuristic,
// the saved AST metadata distinguishes <<x, y>> \in S from x, y \in S.
func normalizeTlagoTupleBinders(module *Module, sources *SourceStore, layouts []tlagoBinderLayout) *Module {
	if module == nil || sources == nil || len(layouts) == 0 {
		return module
	}
	byRegion := make(map[SourceRegion][]tlagoBinderLayout, len(layouts))
	for _, layout := range layouts {
		region := NewSourceRegion(layout.position.Line, layout.position.Column, layout.position.EndLine, layout.position.EndColumn)
		byRegion[region] = append(byRegion[region], layout)
	}
	tracker := NewListenerTracker(sources)
	rewrite := RewriteExpressions(tracker, func(expression Expr) (Expr, bool) {
		operation, ok := expression.(*OperExpr)
		if !ok || (operation.Operator().Name() != "FUN_CTOR" && operation.Operator().Name() != "SET_MAP") {
			return nil, false
		}
		location, found := sources.Find(operation.ID())
		if !found {
			return nil, false
		}
		candidates := byRegion[location.Region]
		if len(candidates) != 1 || candidates[0].operator != operation.Operator().Name() {
			return nil, false
		}
		layout := candidates[0]
		arguments := operation.Arguments()
		expectedBounds := 0
		for _, group := range layout.groups {
			expectedBounds += len(group.names)
		}
		if len(arguments) != 1+2*expectedBounds {
			return nil, false
		}
		result := []Expr{arguments[0]}
		offset := 1
		changed := false
		for _, group := range layout.groups {
			if !group.isTuple {
				result = append(result, arguments[offset], arguments[offset+1])
				offset += 2
				continue
			}
			binders := make([]Expr, 0, len(group.names))
			domain := arguments[offset+1]
			for index, name := range group.names {
				binder, named := arguments[offset+2*index].(*NameExpr)
				if !named || binder.Name() != name || !EqualExpr(arguments[offset+2*index+1], domain) {
					return nil, false
				}
				binders = append(binders, binder)
			}
			result = append(result, MustOperExpr(Op("TUPLE"), Untyped(), binders...), domain)
			offset += 2 * len(group.names)
			changed = true
		}
		if !changed {
			return nil, false
		}
		return MustOperExpr(operation.Operator(), operation.Tag(), result...), true
	})
	return transformTlagoModuleExpressions(module, rewrite, sources)
}

func transformTlagoModuleExpressions(module *Module, transform ExprTransform, sources *SourceStore) *Module {
	original := module.Declarations()
	result := TransformModuleExpressions(module, transform, nil)
	if sources == nil {
		return result
	}
	transformed := result.Declarations()
	for index := range original {
		sources.OnDeclarationTransformation(original[index], transformed[index])
	}
	return result
}

// normalizeTlagoApalacheBuiltinCalls repairs imported Apalache operators that
// tlago v0.1.9 exports as unresolved OPER_APP calls instead of builtin IR
// operators. A declaration in the root module intentionally shadows a builtin.
func normalizeTlagoApalacheBuiltinCalls(module *Module, sources *SourceStore) *Module {
	if module == nil {
		return nil
	}
	declared := make(map[string]struct{}, len(module.Declarations()))
	parameters := make(map[string][]OperatorParam, len(module.Declarations()))
	for _, declaration := range module.Declarations() {
		declared[declaration.Name()] = struct{}{}
		if operator, ok := declaration.(*OperDecl); ok {
			parameters[operator.Name()] = operator.FormalParams()
		}
	}
	var tracker TransformationTracker = IdleTracker{}
	if sources != nil {
		tracker = NewListenerTracker(sources)
	}
	rewrite := RewriteExpressions(tracker, func(expression Expr) (Expr, bool) {
		operation, ok := expression.(*OperExpr)
		if !ok || operation.Operator().Name() != "OPER_APP" {
			return nil, false
		}
		arguments := operation.Arguments()
		if len(arguments) == 0 {
			return nil, false
		}
		callee, ok := arguments[0].(*NameExpr)
		if !ok {
			return nil, false
		}
		changed := false
		if formalParameters, known := parameters[callee.Name()]; known && len(formalParameters) == len(arguments)-1 {
			for index, parameter := range formalParameters {
				if !parameter.IsOperator() {
					continue
				}
				argument := arguments[index+1]
				if application, isApplication := argument.(*OperExpr); isApplication && application.Operator().Name() == "OPER_APP" {
					applicationArguments := application.Arguments()
					if len(applicationArguments) == 1 {
						if name, isName := applicationArguments[0].(*NameExpr); isName {
							argument = name
							changed = true
						}
					}
				}
				if name, isName := argument.(*NameExpr); isName {
					if _, shadowed := declared[name.Name()]; !shadowed {
						if wrapped, wrappedBuiltin := wrapTlagoBuiltinOperator(name, parameter.Arity); wrappedBuiltin {
							argument = wrapped
							changed = true
						}
					}
				}
				arguments[index+1] = argument
			}
		}
		if _, shadowed := declared[callee.Name()]; !shadowed &&
			(callee.Name() == "FunAsSeq" || strings.HasSuffix(callee.Name(), "!FunAsSeq")) {
			if expanded, expandedOK := expandTlagoFunAsSeq(operation, arguments); expandedOK {
				return expanded, true
			}
		}
		if _, shadowed := declared[callee.Name()]; !shadowed &&
			(callee.Name() == "SelectSeq" || strings.HasSuffix(callee.Name(), "!SelectSeq")) {
			if expanded, expandedOK := expandTlagoSelectSeq(operation, arguments); expandedOK {
				return expanded, true
			}
		}
		if _, shadowed := declared[callee.Name()]; !shadowed &&
			(callee.Name() == "ToSet" || strings.HasSuffix(callee.Name(), "!ToSet")) {
			if len(arguments) == 2 {
				return MustOperExpr(Op("SequencesExt!ToSet"), operation.Tag(), arguments[1]), true
			}
		}
		if _, shadowed := declared[callee.Name()]; !shadowed &&
			(callee.Name() == "SetToSeq" || strings.HasSuffix(callee.Name(), "!SetToSeq")) {
			if expanded, expandedOK := expandTlagoSetToSequence(operation, arguments); expandedOK {
				return expanded, true
			}
		}
		builtin, found := tlagoApalacheBuiltinCalls[callee.Name()]
		if !found {
			if changed {
				return MustOperExpr(operation.Operator(), operation.Tag(), arguments...), true
			}
			return nil, false
		}
		_, internalBuiltin := tlagoInternalBuiltinCalls[callee.Name()]
		if _, shadowed := declared[callee.Name()]; shadowed && !internalBuiltin {
			return nil, false
		}
		builtinArguments := arguments[1:]
		operatorArgument := -1
		switch builtin {
		case "Apalache!ApaFoldSet", "Apalache!ApaFoldSeqLeft", "Apalache!Repeat":
			operatorArgument = 0
		case "Apalache!MkSeq":
			operatorArgument = 1
		}
		if operatorArgument >= 0 && operatorArgument < len(builtinArguments) {
			application, isApplication := builtinArguments[operatorArgument].(*OperExpr)
			if isApplication && application.Operator().Name() == "OPER_APP" {
				applicationArguments := application.Arguments()
				if len(applicationArguments) == 1 {
					if name, isName := applicationArguments[0].(*NameExpr); isName {
						builtinArguments[operatorArgument] = name
					}
				}
			}
		}
		return MustOperExpr(Op(builtin), operation.Tag(), builtinArguments...), true
	})
	return transformTlagoModuleExpressions(module, rewrite, sources)
}

func expandTlagoFunAsSeq(operation *OperExpr, arguments []Expr) (Expr, bool) {
	if len(arguments) != 4 {
		return nil, false
	}
	suffix := operation.ID().String()
	parameterName := "__fun_as_seq_index_" + suffix
	definitionName := "__fun_as_seq_element_" + suffix
	parameter := NewNameExpr(parameterName, Untyped())
	body := MustOperExpr(Op("FUN_APP"), Untyped(), arguments[1], parameter)
	definition := NewOperDecl(definitionName, []OperatorParam{ValueParam(parameterName)}, body, Untyped())
	embedded := NewLetInExpr(NewNameExpr(definitionName, Untyped()), Untyped(), definition)
	sequence := MustOperExpr(Op("Apalache!MkSeq"), Untyped(), arguments[3], embedded)
	return MustOperExpr(Op("Sequences!SubSeq"), operation.Tag(), sequence,
		NewValueExpr(Int64Value(1), Untyped()), arguments[2]), true
}

func expandTlagoSelectSeq(operation *OperExpr, arguments []Expr) (Expr, bool) {
	if len(arguments) != 3 {
		return nil, false
	}
	predicate := arguments[2]
	if application, ok := predicate.(*OperExpr); ok && application.Operator().Name() == "OPER_APP" {
		applicationArguments := application.Arguments()
		if len(applicationArguments) == 1 {
			predicate = applicationArguments[0]
		}
	}
	if _, name := predicate.(*NameExpr); !name {
		if _, embedded := predicate.(*LetInExpr); !embedded {
			return nil, false
		}
	}
	suffix := operation.ID().String()
	accumulatorName := "__select_seq_acc_" + suffix
	elementName := "__select_seq_element_" + suffix
	definitionName := "__select_seq_append_" + suffix
	accumulator := NewNameExpr(accumulatorName, Untyped())
	element := NewNameExpr(elementName, Untyped())
	test := MustOperExpr(Op("OPER_APP"), Untyped(), predicate, element)
	appendElement := MustOperExpr(Op("Sequences!Append"), Untyped(), accumulator, element)
	body := MustOperExpr(Op("IF_THEN_ELSE"), Untyped(), test, appendElement, accumulator)
	definition := NewOperDecl(definitionName,
		[]OperatorParam{ValueParam(accumulatorName), ValueParam(elementName)}, body, Untyped())
	embedded := NewLetInExpr(NewNameExpr(definitionName, Untyped()), Untyped(), definition)
	empty := MustOperExpr(Op("TUPLE"), Untyped())
	return MustOperExpr(Op("Apalache!ApaFoldSeqLeft"), operation.Tag(), embedded, empty, arguments[1]), true
}

func expandTlagoSetToSequence(operation *OperExpr, arguments []Expr) (Expr, bool) {
	if len(arguments) != 2 {
		return nil, false
	}
	suffix := operation.ID().String()
	accumulatorName := "__set_to_seq_acc_" + suffix
	elementName := "__set_to_seq_element_" + suffix
	definitionName := "__set_to_seq_append_" + suffix
	appendElement := MustOperExpr(Op("Sequences!Append"), Untyped(),
		NewNameExpr(accumulatorName, Untyped()), NewNameExpr(elementName, Untyped()))
	definition := NewOperDecl(definitionName,
		[]OperatorParam{ValueParam(accumulatorName), ValueParam(elementName)}, appendElement, Untyped())
	embedded := NewLetInExpr(NewNameExpr(definitionName, Untyped()), Untyped(), definition)
	empty := MustOperExpr(Op("TUPLE"), Untyped())
	return MustOperExpr(Op("Apalache!ApaFoldSet"), operation.Tag(), embedded, empty, arguments[1]), true
}

func wrapTlagoBuiltinOperator(name *NameExpr, arity int) (Expr, bool) {
	byArity, found := tlagoBuiltinOperatorArguments[name.Name()]
	if !found {
		return nil, false
	}
	operatorName, found := byArity[arity]
	if !found {
		return nil, false
	}
	operator := Op(operatorName)
	if !operator.IsCorrectArity(arity) {
		return nil, false
	}
	definitionName := fmt.Sprintf("__builtin_%s_%s", operatorName, name.ID())
	parameters := make([]OperatorParam, arity)
	arguments := make([]Expr, arity)
	for index := range parameters {
		parameterName := fmt.Sprintf("__p%d_%s", index+1, name.ID())
		parameters[index] = ValueParam(parameterName)
		arguments[index] = NewNameExpr(parameterName, Untyped())
	}
	definition := NewOperDecl(definitionName, parameters, MustOperExpr(operator, Untyped(), arguments...), Untyped())
	return NewLetInExpr(NewNameExpr(definitionName, Untyped()), Untyped(), definition), true
}

// normalizeTlagoTupleQuantifiers reconstructs a tuple binder that tlago
// v0.1.9 exports as nested binders over the same Cartesian product. The
// generated quantifiers share one exact source region, which distinguishes
// them from user-written nested quantifiers.
func normalizeTlagoTupleQuantifiers(module *Module, sources *SourceStore) *Module {
	if module == nil || sources == nil {
		return module
	}
	tracker := NewListenerTracker(sources)
	rewrite := RewriteExpressions(tracker, func(expression Expr) (Expr, bool) {
		outer, ok := expression.(*OperExpr)
		if !ok || (outer.Operator().Name() != "EXISTS3" && outer.Operator().Name() != "FORALL3") {
			return nil, false
		}
		outerArguments := outer.Arguments()
		if len(outerArguments) != 3 {
			return nil, false
		}
		product, ok := outerArguments[1].(*OperExpr)
		if !ok || product.Operator().Name() != "SET_TIMES" {
			return nil, false
		}
		location, found := sources.Find(outer.ID())
		if !found {
			return nil, false
		}
		binders := []Expr{outerArguments[0]}
		body := outerArguments[2]
		for {
			inner, isInner := body.(*OperExpr)
			if !isInner || inner.Operator().Name() != outer.Operator().Name() {
				break
			}
			innerArguments := inner.Arguments()
			innerLocation, hasLocation := sources.Find(inner.ID())
			if len(innerArguments) != 3 || !hasLocation || innerLocation != location ||
				!EqualExpr(innerArguments[1], outerArguments[1]) {
				break
			}
			binders = append(binders, innerArguments[0])
			body = innerArguments[2]
		}
		if len(binders) < 2 || len(product.Arguments()) != len(binders) {
			return nil, false
		}
		for _, binder := range binders {
			if _, named := binder.(*NameExpr); !named {
				return nil, false
			}
		}
		tuple := MustOperExpr(Op("TUPLE"), Untyped(), binders...)
		return MustOperExpr(outer.Operator(), outer.Tag(), tuple, outerArguments[1], body), true
	})
	return transformTlagoModuleExpressions(module, rewrite, sources)
}

package apalache

import (
	"fmt"

	"github.com/glycerine/tlago"
)

var tlagoApalacheBuiltinCalls = map[string]string{
	"Guess":          "Apalache!Guess",
	"Gen":            "Apalache!Gen",
	"Expand":         "Apalache!Expand",
	"MkSeq":          "Apalache!MkSeq",
	"SetAsFun":       "Apalache!SetAsFun",
	"Repeat":         "Apalache!Repeat",
	"ApaFoldSet":     "Apalache!ApaFoldSet",
	"ApaFoldSeqLeft": "Apalache!ApaFoldSeqLeft",
	"Distinct":       "Apalache!Distinct",
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
		builtin, found := tlagoApalacheBuiltinCalls[callee.Name()]
		if !found {
			if changed {
				return MustOperExpr(operation.Operator(), operation.Tag(), arguments...), true
			}
			return nil, false
		}
		if _, shadowed := declared[callee.Name()]; shadowed {
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

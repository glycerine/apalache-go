package apalache

import "github.com/glycerine/tlago"

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

// normalizeTlagoApalacheAssignments works around tlago v0.1.9's ApalacheIR
// exporter rejecting the imported infix := operator. Replacing it by its
// standard-library definition is semantics-preserving: x := e is defined as
// x = e, and the assignment-analysis pass subsequently recovers assignments.
func normalizeTlagoApalacheAssignments(spec *tlago.Spec) {
	if spec == nil || spec.Root == nil {
		return
	}
	for index := range spec.Root.Definitions {
		normalizeTlagoAssignmentExpr(spec.Root.Definitions[index].Expr)
	}
	for index := range spec.Root.Assumptions {
		normalizeTlagoAssignmentExpr(spec.Root.Assumptions[index].Expr)
	}
	for index := range spec.Root.Theorems {
		normalizeTlagoAssignmentExpr(spec.Root.Theorems[index].Expr)
	}
}

func normalizeTlagoAssignmentExpr(expression tlago.Expr) {
	if expression == nil {
		return
	}
	switch typed := expression.(type) {
	case *tlago.IdentExpr, *tlago.LiteralExpr:
	case *tlago.UnaryExpr:
		normalizeTlagoAssignmentExpr(typed.Expr)
	case *tlago.BinaryExpr:
		if typed.Op == ":=" {
			typed.Op = "="
		}
		normalizeTlagoAssignmentExpr(typed.Left)
		normalizeTlagoAssignmentExpr(typed.Right)
	case *tlago.CallExpr:
		normalizeTlagoAssignmentExpr(typed.Callee)
		normalizeTlagoAssignmentExprs(typed.Args)
	case *tlago.IfExpr:
		normalizeTlagoAssignmentExpr(typed.Cond)
		normalizeTlagoAssignmentExpr(typed.Then)
		normalizeTlagoAssignmentExpr(typed.Else)
	case *tlago.LetExpr:
		for index := range typed.Definitions {
			normalizeTlagoAssignmentExpr(typed.Definitions[index].Expr)
		}
		for index := range typed.Instances {
			for _, substitution := range typed.Instances[index].SubstitutionList {
				normalizeTlagoAssignmentExpr(substitution.Expr)
			}
		}
		normalizeTlagoAssignmentExpr(typed.Body)
	case *tlago.QuantifierExpr:
		normalizeTlagoAssignmentExpr(typed.Set)
		normalizeTlagoAssignmentExpr(typed.Body)
	case *tlago.CaseExpr:
		for _, arm := range typed.Arms {
			normalizeTlagoAssignmentExpr(arm.Test)
			normalizeTlagoAssignmentExpr(arm.Value)
		}
		normalizeTlagoAssignmentExpr(typed.Other)
	case *tlago.ChooseExpr:
		normalizeTlagoAssignmentExpr(typed.Set)
		normalizeTlagoAssignmentExpr(typed.Body)
	case *tlago.TupleExpr:
		normalizeTlagoAssignmentExprs(typed.Elems)
	case *tlago.SetExpr:
		normalizeTlagoAssignmentExprs(typed.Elems)
	case *tlago.RecordExpr:
		for _, field := range typed.Fields {
			normalizeTlagoAssignmentExpr(field.Value)
		}
	case *tlago.RecordComponentExpr:
		normalizeTlagoAssignmentExpr(typed.Record)
	case *tlago.RecordSetExpr:
		for _, field := range typed.Fields {
			normalizeTlagoAssignmentExpr(field.Set)
		}
	case *tlago.FunctionExpr:
		for _, bound := range typed.Bounds {
			normalizeTlagoAssignmentExpr(bound.Set)
		}
		normalizeTlagoAssignmentExpr(typed.Body)
	case *tlago.FunctionAppExpr:
		normalizeTlagoAssignmentExpr(typed.Function)
		normalizeTlagoAssignmentExprs(typed.Args)
	case *tlago.ExceptExpr:
		normalizeTlagoAssignmentExpr(typed.Base)
		for _, spec := range typed.Specs {
			for _, component := range spec.Components {
				normalizeTlagoAssignmentExprs(component.Indices)
			}
			normalizeTlagoAssignmentExpr(spec.Value)
		}
	case *tlago.LabelExpr:
		normalizeTlagoAssignmentExpr(typed.Body)
	case *tlago.ActionExpr:
		normalizeTlagoAssignmentExpr(typed.Action)
		normalizeTlagoAssignmentExpr(typed.Subscript)
	case *tlago.FairnessExpr:
		normalizeTlagoAssignmentExpr(typed.Subscript)
		normalizeTlagoAssignmentExpr(typed.Action)
	case *tlago.FunctionSetExpr:
		normalizeTlagoAssignmentExpr(typed.Domain)
		normalizeTlagoAssignmentExpr(typed.Range)
	case *tlago.SetComprehensionExpr:
		normalizeTlagoAssignmentExpr(typed.Element)
		for _, bound := range typed.Bounds {
			normalizeTlagoAssignmentExpr(bound.Set)
		}
		normalizeTlagoAssignmentExpr(typed.Predicate)
	}
}

func normalizeTlagoAssignmentExprs(expressions []tlago.Expr) {
	for _, expression := range expressions {
		normalizeTlagoAssignmentExpr(expression)
	}
}

// normalizeTlagoApalacheBuiltinCalls repairs imported Apalache operators that
// tlago v0.1.9 exports as unresolved OPER_APP calls instead of builtin IR
// operators. A declaration in the root module intentionally shadows a builtin.
func normalizeTlagoApalacheBuiltinCalls(module *Module, sources *SourceStore) *Module {
	if module == nil {
		return nil
	}
	declared := make(map[string]struct{}, len(module.Declarations()))
	for _, declaration := range module.Declarations() {
		declared[declaration.Name()] = struct{}{}
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
		builtin, found := tlagoApalacheBuiltinCalls[callee.Name()]
		if !found {
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
	return TransformModuleExpressions(module, rewrite, nil)
}

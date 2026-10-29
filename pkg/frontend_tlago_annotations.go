package apalache

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glycerine/tlago"
)

type tlagoImportedAnnotations struct {
	types       map[string]Type
	aliases     map[string]Type
	diagnostics []Diagnostic
}

type tlagoAnnotationTarget struct {
	name       string
	position   tlago.Position
	definition bool
}

func collectTlagoImportedAnnotations(spec *tlago.Spec) tlagoImportedAnnotations {
	result := tlagoImportedAnnotations{types: make(map[string]Type), aliases: make(map[string]Type)}
	if spec == nil || spec.Root == nil {
		return result
	}
	parsed := make(map[string]tlagoImportedAnnotations)
	moduleAnnotations := func(module *tlago.Module) tlagoImportedAnnotations {
		if module == nil {
			return tlagoImportedAnnotations{}
		}
		if annotations, found := parsed[module.Name]; found {
			return annotations
		}
		annotations := parseTlagoModuleAnnotations(module)
		parsed[module.Name] = annotations
		result.diagnostics = append(result.diagnostics, annotations.diagnostics...)
		for name, typ := range annotations.aliases {
			if previous, duplicate := result.aliases[name]; duplicate && !EqualTypes(previous, typ) {
				result.diagnostics = append(result.diagnostics, Diagnostic{
					Code: "E-TYPE-ALIAS", Severity: SeverityError,
					Message: fmt.Sprintf("conflicting imported @typeAlias %s", name),
				})
				continue
			}
			result.aliases[name] = typ
		}
		return annotations
	}

	seenExtended := make(map[string]bool)
	var visitExtended func(string)
	visitExtended = func(name string) {
		if name == "" || seenExtended[name] {
			return
		}
		seenExtended[name] = true
		module := spec.Modules[name]
		if module == nil {
			return
		}
		for _, parent := range module.Extends {
			visitExtended(parent)
		}
		annotations := moduleAnnotations(module)
		for declaration, typ := range annotations.types {
			result.types[declaration] = typ
		}
	}
	for _, name := range spec.Root.Extends {
		visitExtended(name)
	}

	ownDefinitions := make(map[string]struct{}, len(spec.Root.Definitions))
	for _, definition := range spec.Root.Definitions {
		ownDefinitions[definition.Name] = struct{}{}
	}
	for _, instance := range spec.Root.Instances {
		seen := make(map[string]bool)
		var visitInstanceModule func(string, string)
		visitInstanceModule = func(moduleName, prefix string) {
			key := prefix + "\x00" + moduleName
			if moduleName == "" || seen[key] {
				return
			}
			seen[key] = true
			module := spec.Modules[moduleName]
			if module == nil {
				return
			}
			for _, parent := range module.Extends {
				visitInstanceModule(parent, prefix)
			}
			for _, nested := range module.Instances {
				nestedPrefix := prefix
				if nested.Name != "" {
					if nestedPrefix != "" {
						nestedPrefix += "!"
					}
					nestedPrefix += nested.Name
				}
				visitInstanceModule(nested.Module, nestedPrefix)
			}
			annotations := moduleAnnotations(module)
			for declaration, typ := range annotations.types {
				name := declaration
				if prefix != "" {
					name = prefix + "!" + declaration
				} else if _, shadowed := ownDefinitions[name]; shadowed {
					continue
				}
				result.types[name] = typ
			}
		}
		visitInstanceModule(instance.Module, instance.Name)
	}
	return result
}

func parseTlagoModuleAnnotations(module *tlago.Module) tlagoImportedAnnotations {
	result := tlagoImportedAnnotations{types: make(map[string]Type), aliases: make(map[string]Type)}
	if module == nil || module.Source == "" {
		return result
	}
	targets := make([]tlagoAnnotationTarget, 0, len(module.Definitions)+len(module.Declarations))
	for _, definition := range module.Definitions {
		targets = append(targets, tlagoAnnotationTarget{
			name: definition.Name, position: definition.DeclarationPosition(), definition: true,
		})
		collectTlagoLocalAnnotationTargets(definition.Expr, &targets)
	}
	for _, declaration := range module.Declarations {
		for _, name := range declaration.Names {
			position := declaration.NamePositions[name]
			if position.Line == 0 {
				position = declaration.Pos
			}
			targets = append(targets, tlagoAnnotationTarget{name: name, position: position})
		}
	}
	sort.SliceStable(targets, func(left, right int) bool {
		return compareTlagoPosition(targets[left].position, targets[right].position) < 0
	})
	filename := module.SourcePath
	if filename == "" {
		filename = module.Name + ".tla"
	}
	for _, candidate := range FindAnnotationCandidates(module.Source) {
		position := Position{Filename: filename, Line: candidate.Position.Line, Column: candidate.Position.Column}
		annotation, err := ParseAnnotation(candidate.Text)
		if err != nil {
			if candidate.UnterminatedInline() {
				continue
			}
			result.diagnostics = append(result.diagnostics, Diagnostic{
				Code: "E-ANNOTATION", Severity: SeverityError, Message: err.Error(), Position: position,
			})
			continue
		}
		text, inline := singleAnnotationString(annotation)
		switch annotation.Name {
		case "typeAlias":
			if !inline {
				continue
			}
			name, typ, parseErr := ParseTypeAlias(text)
			if parseErr != nil {
				result.diagnostics = append(result.diagnostics, Diagnostic{
					Code: "E-TYPE-ALIAS", Severity: SeverityError, Message: parseErr.Error(), Position: position,
				})
				continue
			}
			result.aliases[name] = typ
		case "type":
			if !inline {
				continue
			}
			target := nextTlagoAnnotationTarget(candidate.Position, targets)
			if target == nil {
				continue
			}
			typ, parseErr := ParseType(text)
			if parseErr != nil {
				result.diagnostics = append(result.diagnostics, typeAnnotationParseDiagnostics(text, target.name, parseErr, position)...)
				continue
			}
			result.types[target.name] = typ
		}
	}
	return result
}

func collectTlagoLocalAnnotationTargets(expression tlago.Expr, targets *[]tlagoAnnotationTarget) {
	if expression == nil {
		return
	}
	visit := func(expressions ...tlago.Expr) {
		for _, child := range expressions {
			collectTlagoLocalAnnotationTargets(child, targets)
		}
	}
	switch typed := expression.(type) {
	case *tlago.IdentExpr, *tlago.LiteralExpr:
	case *tlago.UnaryExpr:
		visit(typed.Expr)
	case *tlago.BinaryExpr:
		visit(typed.Left, typed.Right)
	case *tlago.CallExpr:
		visit(typed.Callee)
		visit(typed.Args...)
	case *tlago.IfExpr:
		visit(typed.Cond, typed.Then, typed.Else)
	case *tlago.LetExpr:
		for _, definition := range typed.Definitions {
			*targets = append(*targets, tlagoAnnotationTarget{
				name: definition.Name, position: definition.DeclarationPosition(), definition: true,
			})
			visit(definition.Expr)
		}
		for _, instance := range typed.Instances {
			for _, substitution := range instance.SubstitutionList {
				visit(substitution.Expr)
			}
		}
		visit(typed.Body)
	case *tlago.QuantifierExpr:
		visit(typed.Set, typed.Body)
	case *tlago.CaseExpr:
		for _, arm := range typed.Arms {
			visit(arm.Test, arm.Value)
		}
		visit(typed.Other)
	case *tlago.ChooseExpr:
		visit(typed.Set, typed.Body)
	case *tlago.TupleExpr:
		visit(typed.Elems...)
	case *tlago.SetExpr:
		visit(typed.Elems...)
	case *tlago.RecordExpr:
		for _, field := range typed.Fields {
			visit(field.Value)
		}
	case *tlago.RecordComponentExpr:
		visit(typed.Record)
	case *tlago.RecordSetExpr:
		for _, field := range typed.Fields {
			visit(field.Set)
		}
	case *tlago.FunctionExpr:
		for _, bound := range typed.Bounds {
			visit(bound.Set)
		}
		visit(typed.Body)
	case *tlago.FunctionAppExpr:
		visit(typed.Function)
		visit(typed.Args...)
	case *tlago.ExceptExpr:
		visit(typed.Base)
		for _, spec := range typed.Specs {
			for _, component := range spec.Components {
				visit(component.Indices...)
			}
			visit(spec.Value)
		}
	case *tlago.LabelExpr:
		visit(typed.Body)
	case *tlago.ActionExpr:
		visit(typed.Action, typed.Subscript)
	case *tlago.FairnessExpr:
		visit(typed.Subscript, typed.Action)
	case *tlago.FunctionSetExpr:
		visit(typed.Domain, typed.Range)
	case *tlago.SetComprehensionExpr:
		visit(typed.Element)
		for _, bound := range typed.Bounds {
			visit(bound.Set)
		}
		visit(typed.Predicate)
	}
}

func nextTlagoAnnotationTarget(position SourcePosition, targets []tlagoAnnotationTarget) *tlagoAnnotationTarget {
	for index := range targets {
		target := &targets[index]
		start := SourcePosition{Line: target.position.Line, Column: target.position.Column}
		if start.Compare(position) >= 0 {
			return target
		}
	}
	return nil
}

func compareTlagoPosition(left, right tlago.Position) int {
	return SourcePosition{Line: left.Line, Column: left.Column}.Compare(
		SourcePosition{Line: right.Line, Column: right.Column})
}

func mergeTlagoImportedAnnotations(result *TypeAnnotationResult, module *Module, imported tlagoImportedAnnotations) {
	if result == nil {
		return
	}
	result.Diagnostics = append(result.Diagnostics, imported.diagnostics...)
	aliases := result.Aliases.Context()
	for name, typ := range imported.aliases {
		if previous, duplicate := aliases[name]; duplicate && !EqualTypes(previous, typ) {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Code: "E-TYPE-ALIAS", Severity: SeverityError,
				Message: fmt.Sprintf("conflicting @typeAlias %s in imported modules", name),
			})
			continue
		}
		aliases[name] = typ
	}
	closed, err := NewTypeAliasSubstitution(aliases).Closure()
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{
			Code: "E-TYPE-ALIAS", Severity: SeverityError, Message: err.Error(),
		})
	} else {
		result.Aliases = closed
	}
	if module == nil {
		return
	}
	for _, declaration := range module.Declarations() {
		typ, found := imported.types[declaration.Name()]
		if _, explicitlyAnnotated := result.Types[declaration.ID()]; found && !explicitlyAnnotated {
			result.Types[declaration.ID()] = typ
		}
		operator, ok := declaration.(*OperDecl)
		if !ok {
			continue
		}
		prefix := ""
		if separator := strings.LastIndex(operator.Name(), "!"); separator >= 0 {
			prefix = operator.Name()[:separator+1]
		}
		RewriteExpressions(nil, func(expression Expr) (Expr, bool) {
			let, ok := expression.(*LetInExpr)
			if !ok {
				return nil, false
			}
			for _, local := range let.Declarations() {
				annotationName := prefix + local.Name()
				if strings.HasPrefix(local.Name(), "LOCAL$") {
					annotationName = strings.TrimPrefix(local.Name(), "LOCAL$")
				}
				localType, annotated := imported.types[annotationName]
				if !annotated {
					continue
				}
				if _, explicitlyAnnotated := result.Types[local.ID()]; !explicitlyAnnotated {
					result.Types[local.ID()] = localType
				}
			}
			return nil, false
		})(operator.Body())
	}
}

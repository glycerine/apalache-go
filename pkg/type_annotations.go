package apalache

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// TypeAnnotationResult contains parsed declaration types, the closed alias
// environment, and all declaration-attached annotations found in source.
type TypeAnnotationResult struct {
	Types       map[UID]Type
	Aliases     *TypeAliasSubstitution
	Annotations *AnnotationStore
	Diagnostics []Diagnostic
}

func (result TypeAnnotationResult) OK() bool { return !hasErrors(result.Diagnostics) }

type locatedDeclaration struct {
	declaration Decl
	location    SourceLocation
}

// ExtractTypeAnnotations associates source comments with the declaration that
// starts immediately after them, parses @type annotations, and closes the
// module-wide @typeAlias environment.
func ExtractTypeAnnotations(filename, source string, module *Module, sources *SourceStore) TypeAnnotationResult {
	result := TypeAnnotationResult{
		Types: make(map[UID]Type), Aliases: EmptyTypeAliasSubstitution(), Annotations: NewAnnotationStore(),
	}
	if module == nil || sources == nil {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{
			Code: "E-TYPE-ANNOTATION", Severity: SeverityError,
			Message:  "type annotation extraction requires a module with source locations",
			Position: Position{Filename: filename},
		})
		return result
	}

	declarations := locatedDeclarations(filename, module, sources)
	aliases := make(map[string]Type)
	for _, candidate := range FindAnnotationCandidates(source) {
		position := Position{Filename: filename, Line: candidate.Position.Line, Column: candidate.Position.Column}
		annotation, err := ParseAnnotation(candidate.Text)
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Code: "E-ANNOTATION", Severity: SeverityError, Message: err.Error(), Position: position,
			})
			continue
		}
		if annotation.Name == "typeAlias" {
			text, ok := singleAnnotationString(annotation)
			if !ok {
				result.Diagnostics = append(result.Diagnostics, Diagnostic{
					Code: "E-TYPE-ALIAS", Severity: SeverityError,
					Message: "@typeAlias requires the inline form @typeAlias: NAME = TYPE;", Position: position,
				})
				continue
			}
			name, typ, parseErr := ParseTypeAlias(text)
			if parseErr != nil {
				result.Diagnostics = append(result.Diagnostics, Diagnostic{
					Code: "E-TYPE-ALIAS", Severity: SeverityError, Message: parseErr.Error(), Position: position,
				})
				continue
			}
			if _, duplicate := aliases[name]; duplicate {
				result.Diagnostics = append(result.Diagnostics, Diagnostic{
					Code: "E-TYPE-ALIAS", Severity: SeverityError,
					Message: fmt.Sprintf("duplicate type alias %s", name), Position: position,
				})
				continue
			}
			aliases[name] = typ
			continue
		}

		declaration := nextDeclaration(candidate.Position, declarations)
		if declaration == nil {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Code: "E-ANNOTATION-TARGET", Severity: SeverityError,
				Message: fmt.Sprintf("annotation %s is not followed by a declaration", annotation.PrettyString()), Position: position,
			})
			continue
		}
		attached := result.Annotations.Get(declaration.ID())
		attached = append(attached, annotation)
		result.Annotations.Set(declaration.ID(), attached)
		if annotation.Name != "type" {
			continue
		}
		text, ok := singleAnnotationString(annotation)
		if !ok {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Code: "E-TYPE-ANNOTATION", Severity: SeverityError,
				Message: "@type requires the inline form @type: TYPE;", Position: position,
			})
			continue
		}
		if _, duplicate := result.Types[declaration.ID()]; duplicate {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Code: "E-TYPE-ANNOTATION", Severity: SeverityError,
				Message: fmt.Sprintf("duplicate @type annotation for declaration %s", declaration.Name()), Position: position,
			})
			continue
		}
		typ, parseErr := ParseType(text)
		if parseErr != nil {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Code: "E-TYPE-ANNOTATION", Severity: SeverityError, Message: parseErr.Error(), Position: position,
			})
			continue
		}
		result.Types[declaration.ID()] = typ
	}

	closed, err := NewTypeAliasSubstitution(aliases).Closure()
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{
			Code: "E-TYPE-ALIAS", Severity: SeverityError, Message: err.Error(), Position: Position{Filename: filename},
		})
	} else {
		result.Aliases = closed
	}
	return result
}

func singleAnnotationString(annotation Annotation) (string, bool) {
	if len(annotation.Arguments) != 1 {
		return "", false
	}
	value, ok := annotation.Arguments[0].(AnnotationString)
	return value.Text, ok
}

func nextDeclaration(position SourcePosition, declarations []locatedDeclaration) Decl {
	for _, item := range declarations {
		if item.location.Region.Start.Compare(position) >= 0 {
			return item.declaration
		}
	}
	return nil
}

func locatedDeclarations(filename string, module *Module, sources *SourceStore) []locatedDeclaration {
	seen := make(map[UID]struct{})
	result := make([]locatedDeclaration, 0)
	var visitExpression func(Expr)
	var visitDeclaration func(Decl)
	visitExpression = func(expression Expr) {
		switch typed := expression.(type) {
		case *OperExpr:
			for _, argument := range typed.Arguments() {
				visitExpression(argument)
			}
		case *LetInExpr:
			for _, declaration := range typed.Declarations() {
				visitDeclaration(declaration)
			}
			visitExpression(typed.Body())
		}
	}
	visitDeclaration = func(declaration Decl) {
		if declaration == nil {
			return
		}
		if location, ok := sources.Find(declaration.ID()); ok && sameSourceFilename(location.Filename, filename) {
			if _, found := seen[declaration.ID()]; !found {
				seen[declaration.ID()] = struct{}{}
				result = append(result, locatedDeclaration{declaration: declaration, location: location})
			}
		}
		switch typed := declaration.(type) {
		case *OperDecl:
			visitExpression(typed.Body())
		case *AssumeDecl:
			visitExpression(typed.Body())
		case *TheoremDecl:
			visitExpression(typed.Body())
		}
	}
	for _, declaration := range module.Declarations() {
		visitDeclaration(declaration)
	}
	sort.SliceStable(result, func(left, right int) bool {
		comparison := result[left].location.Region.Start.Compare(result[right].location.Region.Start)
		if comparison != 0 {
			return comparison < 0
		}
		return result[left].location.Region.End.Compare(result[right].location.Region.End) < 0
	})
	return result
}

func sameSourceFilename(left, right string) bool {
	if left == right {
		return true
	}
	stem := func(name string) string {
		base := filepath.Base(name)
		return strings.TrimSuffix(base, filepath.Ext(base))
	}
	return stem(left) == stem(right)
}

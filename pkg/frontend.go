package apalache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/glycerine/tlago"
)

// FrontendOptions controls TLA+ loading and the exported intermediate form.
type FrontendOptions struct {
	LibraryPaths  []string
	IncludeSource bool
}

// FrontendResult is the checked TLA+ frontend result. Native is the in-memory
// Go Apalache IR used by subsequent passes. IR retains the canonical JSON form
// for interoperability and conformance testing.
type FrontendResult struct {
	Module      string                 `json:"module,omitempty"`
	Extends     []string               `json:"-"`
	IR          json.RawMessage        `json:"ir,omitempty"`
	Diagnostics []Diagnostic           `json:"diagnostics"`
	Native      *Module                `json:"-"`
	Sources     *SourceStore           `json:"-"`
	Types       map[UID]Type           `json:"-"`
	Annotations *AnnotationStore       `json:"-"`
	Aliases     *TypeAliasSubstitution `json:"-"`
	imported    tlagoImportedAnnotations
}

// OK reports whether the frontend completed without errors.
func (r FrontendResult) OK() bool { return !hasErrors(r.Diagnostics) }

// ParseFile reads either canonical Apalache IR JSON or parses and semantically
// checks a TLA+ module using tlago.
func ParseFile(ctx context.Context, filename string, opts FrontendOptions) FrontendResult {
	source, err := NewFileInputSource(filename)
	if err != nil {
		return FrontendResult{Diagnostics: []Diagnostic{{Code: "E-IO", Severity: SeverityError, Message: err.Error(), Position: Position{Filename: filename}}}}
	}
	return ParseInputSource(ctx, source, opts)
}

// ParseInputSource implements the parser-pass format dispatch for file and
// in-memory sources. Specification ITF is deliberately unsupported.
func ParseInputSource(ctx context.Context, input InputSource, opts FrontendOptions) FrontendResult {
	if err := contextError(ctx); err != nil {
		return canceledResult(err)
	}
	if input == nil {
		return FrontendResult{Diagnostics: []Diagnostic{{Code: "E-IO", Severity: SeverityError, Message: "cannot parse a nil input source"}}}
	}
	source, err := input.ReadUTF8()
	if err != nil {
		return FrontendResult{Diagnostics: []Diagnostic{{
			Code:     "E-IO",
			Severity: SeverityError,
			Message:  err.Error(),
			Position: Position{Filename: input.String()},
		}}}
	}
	switch input.Format() {
	case InputFormatITF:
		return FrontendResult{Diagnostics: []Diagnostic{{Code: "E-SPEC-PARSE", Severity: SeverityError,
			Message: "Parsing the ITF format is not supported", Position: Position{Filename: input.String()}}}}
	case InputFormatJSON:
		return sortFrontendModule(parseIRJSONFile(ctx, input.String(), []byte(source)))
	case InputFormatQuint:
		module, parseErr := ParseQuintJSON([]byte(source))
		if parseErr != nil {
			return FrontendResult{Diagnostics: []Diagnostic{{Code: "E-SPEC-PARSE", Severity: SeverityError,
				Message: parseErr.Error(), Position: Position{Filename: input.String()}}}}
		}
		ir, marshalErr := MarshalSingleModuleIRJSON(module, IRJSONOptions{})
		if marshalErr != nil {
			return FrontendResult{Diagnostics: []Diagnostic{{Code: "E-IR-CONVERSION", Severity: SeverityError, Message: marshalErr.Error()}}}
		}
		return sortFrontendModule(FrontendResult{Module: module.Name(), IR: ir, Native: module, Sources: NewSourceStore()})
	case InputFormatTLA:
		filename := input.String()
		var memory *StringInputSource
		switch typed := input.(type) {
		case StringInputSource:
			memory = &typed
		case *StringInputSource:
			memory = typed
		}
		if memory != nil {
			filename = tlaSourceFilename(memory.Content)
			if len(memory.Auxiliary) != 0 {
				directory, tempErr := os.MkdirTemp("", "apalache-aux-")
				if tempErr != nil {
					return FrontendResult{Diagnostics: []Diagnostic{{Code: "E-IO", Severity: SeverityError, Message: tempErr.Error()}}}
				}
				defer os.RemoveAll(directory)
				for _, auxiliary := range memory.Auxiliary {
					path := filepath.Join(directory, tlaSourceFilename(auxiliary))
					if writeErr := os.WriteFile(path, []byte(auxiliary), 0o600); writeErr != nil {
						return FrontendResult{Diagnostics: []Diagnostic{{Code: "E-IO", Severity: SeverityError, Message: writeErr.Error()}}}
					}
				}
				opts.LibraryPaths = append(append([]string(nil), opts.LibraryPaths...), directory)
			}
		}
		return ParseSource(ctx, filename, source, opts)
	default:
		return FrontendResult{Diagnostics: []Diagnostic{{Code: "E-SPEC-PARSE", Severity: SeverityError,
			Message: fmt.Sprintf("unsupported source format: %s", input.Format())}}}
	}
}

func sortFrontendModule(result FrontendResult) FrontendResult {
	if !result.OK() || result.Native == nil {
		return result
	}
	sorted, err := SortDeclarations(result.Native)
	if err != nil {
		result.Native = nil
		result.IR = nil
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: "E-SPEC-PARSE", Severity: SeverityError,
			Message: fmt.Sprintf("cannot sort declarations: %v", err)})
		return result
	}
	encoded, err := MarshalSingleModuleIRJSON(sorted, IRJSONOptions{SourceStore: result.Sources})
	if err == nil {
		result.IR = encoded
	}
	result.Native = sorted
	return result
}

var tlaSourceModulePattern = regexp.MustCompile(`(?m)\bMODULE[ \t]+([A-Za-z_][A-Za-z0-9_]*)`)

func tlaSourceFilename(source string) string {
	match := tlaSourceModulePattern.FindStringSubmatch(source)
	if len(match) == 2 {
		return match[1] + ".tla"
	}
	return "Source.tla"
}

func parseIRJSONFile(ctx context.Context, filename string, data []byte) FrontendResult {
	if err := contextError(ctx); err != nil {
		return canceledResult(err)
	}
	sources := NewSourceStore()
	module, err := UnmarshalSingleModuleIRJSON(data, sources)
	if err != nil {
		return FrontendResult{Diagnostics: []Diagnostic{{
			Code: "E-IR-JSON", Severity: SeverityError,
			Message: fmt.Sprintf("cannot read Apalache IR JSON: %v", err), Position: Position{Filename: filename},
		}}}
	}
	return FrontendResult{
		Module: module.Name(), IR: append(json.RawMessage(nil), data...), Native: module, Sources: sources,
	}
}

// ParseSource parses and semantically checks source using tlago, then exports
// canonical ApalacheIR JSON. tlago's frontend is currently synchronous, so
// cancellation is observed before and after that call rather than interrupting
// it midway.
func ParseSource(ctx context.Context, filename, source string, opts FrontendOptions) FrontendResult {
	if err := contextError(ctx); err != nil {
		return canceledResult(err)
	}

	libraryPaths, cleanupLibraries, mountErr := mountBundledTLAModules(opts.LibraryPaths)
	if mountErr != nil {
		return FrontendResult{Diagnostics: []Diagnostic{{
			Code: "E-IO", Severity: SeverityError, Message: mountErr.Error(), Position: Position{Filename: filename},
		}}}
	}
	defer cleanupLibraries()
	// tlago always searches the root filename's directory before explicit
	// library paths. Give in-memory source a deliberately nonexistent root
	// directory inside our temporary mount so it cannot discover modules from
	// the process working directory. Adjacent source modules are added below as
	// an explicit library path when the caller supplied a directory.
	sanyFilename := filepath.Join(libraryPaths[len(libraryPaths)-1], "__source__", filepath.Base(filename))
	if directory := filepath.Dir(filename); directory != "." && directory != "" {
		// The source itself is already supplied in memory. Decouple its logical
		// filename from its directory so PreferLibraryModules can select bundled
		// Apalache rewrites before an adjacent module with the same name.
		libraryPaths = append(libraryPaths, directory)
	}
	spec, tlagoDiags := tlago.CheckSanySourceWithOptions(sanyFilename, source, tlago.LoadOptions{
		LibraryPaths: libraryPaths, PreferLibraryModules: true,
	})
	result := FrontendResult{Diagnostics: frontendDiagnostics(tlagoDiags)}
	if spec != nil && spec.Root != nil {
		result.Module = spec.Root.Name
		result.Extends = append([]string(nil), spec.Root.Extends...)
	}
	if hasErrors(result.Diagnostics) {
		return result
	}
	normalizeTlagoInstanceExtendedDefinitions(spec)
	result.imported = collectTlagoImportedAnnotations(spec)
	if err := contextError(ctx); err != nil {
		return canceledResult(err)
	}

	normalizeTlagoRootLocalInstances(spec)
	normalizeTlagoNestedInstances(spec)
	instantiatedLocals := exposeTlagoRootLocalDefinitions(spec)
	instantiatedLocals = append(instantiatedLocals, exposeTlagoInstantiatedLocalDefinitions(spec)...)
	normalizeTlagoLambdas(spec)
	binderLayouts := normalizeTlagoApalacheAssignments(spec)
	builtinInstanceModules := tlagoBuiltinInstanceModules(spec)
	recursiveScopes := tlagoRecursiveOperatorScopes(spec)
	instantiatedScopes := tlagoInstantiatedOperatorScopes(spec)
	ir, irDiags := tlago.ApalacheIRJSON(spec, tlago.ApalacheIROptions{IncludeSource: opts.IncludeSource})
	result.Diagnostics = append(result.Diagnostics, frontendDiagnostics(irDiags)...)
	if !hasErrors(result.Diagnostics) {
		result.IR = json.RawMessage(ir)
		result.Sources = NewSourceStore()
		native, conversionErr := UnmarshalSingleModuleIRJSON(ir, result.Sources)
		result.Native = normalizeTlagoLambdaValues(native, result.Sources)
		result.Native = normalizeTlagoRecursiveFunctions(result.Native, result.Sources)
		result.Native = normalizeTlagoOperatorValues(result.Native, result.Sources)
		result.Native = normalizeTlagoNamedAssumptionReferences(result.Native, result.Sources)
		variantsImported := spec.Modules["Variants"] != nil
		tlcImported := spec.Modules["TLC"] != nil
		realsImported := spec.Modules["Reals"] != nil
		result.Native = removeTlagoNativeVariantDeclarations(result.Native, result.Sources, variantsImported)
		result.Native = normalizeTlagoTLCCalls(result.Native, result.Sources, tlcImported)
		result.Native = normalizeTlagoRealValues(result.Native, result.Sources, realsImported)
		result.Native = normalizeTlagoApalacheBuiltinCalls(result.Native, result.Sources, builtinInstanceModules)
		result.Native = normalizeTlagoInstantiatedOperatorCalls(result.Native, result.Sources, instantiatedScopes)
		result.Native = normalizeTlagoInstantiatedLocalDefinitions(result.Native, result.Sources, instantiatedLocals)
		result.Native = normalizeTlagoLocalDeclarationTags(result.Native, result.Sources)
		result.Native = normalizeTlagoRecursiveOperators(result.Native, result.Sources, recursiveScopes)
		result.Native = normalizeTlagoVariantUnit(result.Native, result.Sources, variantsImported)
		result.Native = normalizeTlagoTupleBinders(result.Native, result.Sources, binderLayouts)
		result.Native = normalizeTlagoTupleQuantifiers(result.Native, result.Sources)
		if conversionErr != nil {
			result.IR = nil
			result.Native = nil
			result.Sources = nil
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Code:     "E-IR-CONVERSION",
				Severity: SeverityError,
				Message:  fmt.Sprintf("cannot convert frontend output to native IR: %v", conversionErr),
			})
		} else if diagnostic := tlagoSubformulaReferenceDiagnostic(result.Native, result.Sources); diagnostic != nil {
			result.IR = nil
			result.Native = nil
			result.Sources = nil
			result.Diagnostics = append(result.Diagnostics, *diagnostic)
		}
	}
	return sortFrontendModule(result)
}

// ParseAndTypeCheckSource runs the native Go type checker using @type and
// @typeAlias comments from source. Native is the typed module on success; IR
// remains the canonical frontend export used to construct that module.
func ParseAndTypeCheckSource(ctx context.Context, filename, source string, frontendOptions FrontendOptions, typeOptions TypeCheckOptions) FrontendResult {
	frontendOptions.IncludeSource = true
	result := ParseSource(ctx, filename, source, frontendOptions)
	if !result.OK() {
		return result
	}
	annotations := ExtractTypeAnnotations(filename, source, result.Native, result.Sources)
	mergeTlagoImportedAnnotations(&annotations, result.Native, result.imported)
	result.Annotations = annotations.Annotations
	result.Aliases = annotations.Aliases
	result.Diagnostics = append(result.Diagnostics, annotations.Diagnostics...)
	if !annotations.OK() {
		return result
	}
	typeOptions.Annotations = annotations.Types
	typeOptions.Aliases = annotations.Aliases
	typeOptions.Sources = result.Sources
	typed := TypeCheckModule(result.Native, typeOptions)
	result.Types = typed.Types
	result.Diagnostics = append(result.Diagnostics, typed.Diagnostics...)
	if typed.Module != nil {
		result.Annotations = remapDeclarationAnnotations(result.Native, typed.Module, result.Annotations)
		result.Native = typed.Module
	}
	return result
}

func remapDeclarationAnnotations(from, to *Module, annotations *AnnotationStore) *AnnotationStore {
	if annotations == nil || from == nil || to == nil {
		return annotations
	}
	fromDeclarations := nestedModuleDeclarationsByPath(from)
	toDeclarations := nestedModuleDeclarationsByPath(to)
	result := NewAnnotationStore()
	for path, declaration := range fromDeclarations {
		target, found := toDeclarations[path]
		if !found || declaration.Name() != target.Name() {
			continue
		}
		if values := annotations.Get(declaration.ID()); len(values) > 0 {
			result.Set(target.ID(), values)
		}
	}
	return result
}

func nestedModuleDeclarationsByPath(module *Module) map[string]Decl {
	result := make(map[string]Decl)
	if module == nil {
		return result
	}
	var visitExpression func(Expr, string, *int)
	visitExpression = func(expression Expr, parent string, ordinal *int) {
		switch typed := expression.(type) {
		case *OperExpr:
			for _, argument := range typed.Arguments() {
				visitExpression(argument, parent, ordinal)
			}
		case *LetInExpr:
			for _, declaration := range typed.Declarations() {
				path := fmt.Sprintf("%s/let:%d:%s", parent, *ordinal, declaration.Name())
				*ordinal++
				result[path] = declaration
				nestedOrdinal := 0
				visitExpression(declaration.Body(), path, &nestedOrdinal)
			}
			visitExpression(typed.Body(), parent, ordinal)
		}
	}
	for _, declaration := range module.Declarations() {
		path := "top:" + declaration.Name()
		result[path] = declaration
		ordinal := 0
		switch typed := declaration.(type) {
		case *OperDecl:
			visitExpression(typed.Body(), path, &ordinal)
		case *AssumeDecl:
			visitExpression(typed.Body(), path, &ordinal)
		}
	}
	return result
}

func nestedModuleDeclarations(module *Module) []Decl {
	if module == nil {
		return nil
	}
	result := make([]Decl, 0, len(module.Declarations()))
	var visitExpression func(Expr)
	visitExpression = func(expression Expr) {
		switch typed := expression.(type) {
		case *OperExpr:
			for _, argument := range typed.Arguments() {
				visitExpression(argument)
			}
		case *LetInExpr:
			for _, declaration := range typed.Declarations() {
				result = append(result, declaration)
				visitExpression(declaration.Body())
			}
			visitExpression(typed.Body())
		}
	}
	for _, declaration := range module.Declarations() {
		result = append(result, declaration)
		switch typed := declaration.(type) {
		case *OperDecl:
			visitExpression(typed.Body())
		case *AssumeDecl:
			visitExpression(typed.Body())
		}
	}
	return result
}

// ParseAndTypeCheckFile reads a module and runs ParseAndTypeCheckSource.
func ParseAndTypeCheckFile(ctx context.Context, filename string, frontendOptions FrontendOptions, typeOptions TypeCheckOptions) FrontendResult {
	if err := contextError(ctx); err != nil {
		return canceledResult(err)
	}
	input, err := NewFileInputSource(filename)
	if err != nil {
		return FrontendResult{Diagnostics: []Diagnostic{{Code: "E-IO", Severity: SeverityError, Message: err.Error(), Position: Position{Filename: filename}}}}
	}
	source, err := os.ReadFile(filename)
	if err != nil {
		return FrontendResult{Diagnostics: []Diagnostic{{
			Code: "E-IO", Severity: SeverityError, Message: err.Error(), Position: Position{Filename: filename},
		}}}
	}
	if input.Format() != InputFormatTLA {
		result := ParseInputSource(ctx, input, frontendOptions)
		if !result.OK() {
			return result
		}
		if (TypedLanguagePredicate{}).CheckModule(result.Native).OK() {
			result.Native.AddProperty(PropertyTypeChecked)
			return result
		}
		typeOptions.Sources = result.Sources
		typed := TypeCheckModule(result.Native, typeOptions)
		result.Types = typed.Types
		result.Diagnostics = append(result.Diagnostics, typed.Diagnostics...)
		if typed.Module != nil {
			result.Native = typed.Module
		}
		return result
	}
	return ParseAndTypeCheckSource(ctx, filename, string(source), frontendOptions, typeOptions)
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func canceledResult(err error) FrontendResult {
	message := "operation canceled"
	if errors.Is(err, context.DeadlineExceeded) {
		message = "operation deadline exceeded"
	} else if err != nil && !errors.Is(err, context.Canceled) {
		message = fmt.Sprintf("operation canceled: %v", err)
	}
	return FrontendResult{Diagnostics: []Diagnostic{{
		Code:     "E-CANCELED",
		Severity: SeverityError,
		Message:  message,
	}}}
}

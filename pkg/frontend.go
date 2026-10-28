package apalache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	if err := contextError(ctx); err != nil {
		return canceledResult(err)
	}
	source, err := os.ReadFile(filename)
	if err != nil {
		return FrontendResult{Diagnostics: []Diagnostic{{
			Code:     "E-IO",
			Severity: SeverityError,
			Message:  err.Error(),
			Position: Position{Filename: filename},
		}}}
	}
	if strings.EqualFold(filepath.Ext(filename), ".json") {
		return parseIRJSONFile(ctx, filename, source)
	}
	return ParseSource(ctx, filename, string(source), opts)
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
	sanyFilename := filename
	if directory := filepath.Dir(filename); directory != "." && directory != "" {
		// The source itself is already supplied in memory. Decouple its logical
		// filename from its directory so PreferLibraryModules can select bundled
		// Apalache rewrites before an adjacent module with the same name.
		libraryPaths = append(libraryPaths, directory)
		sanyFilename = filepath.Base(filename)
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
	instantiatedLocals := exposeTlagoInstantiatedLocalDefinitions(spec)
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
		}
	}
	return result
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
	byName := make(map[string][]Annotation)
	for _, declaration := range from.Declarations() {
		if values := annotations.Get(declaration.ID()); len(values) > 0 {
			byName[declaration.Name()] = values
		}
	}
	result := NewAnnotationStore()
	for _, declaration := range to.Declarations() {
		if values := byName[declaration.Name()]; len(values) > 0 {
			result.Set(declaration.ID(), values)
		}
	}
	return result
}

// ParseAndTypeCheckFile reads a module and runs ParseAndTypeCheckSource.
func ParseAndTypeCheckFile(ctx context.Context, filename string, frontendOptions FrontendOptions, typeOptions TypeCheckOptions) FrontendResult {
	if err := contextError(ctx); err != nil {
		return canceledResult(err)
	}
	source, err := os.ReadFile(filename)
	if err != nil {
		return FrontendResult{Diagnostics: []Diagnostic{{
			Code: "E-IO", Severity: SeverityError, Message: err.Error(), Position: Position{Filename: filename},
		}}}
	}
	if strings.EqualFold(filepath.Ext(filename), ".json") {
		result := parseIRJSONFile(ctx, filename, source)
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

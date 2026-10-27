package apalache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

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

// ParseFile reads, parses, and semantically checks a TLA+ module using tlago.
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
	return ParseSource(ctx, filename, string(source), opts)
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
	spec, tlagoDiags := tlago.CheckSanySourceWithOptions(filename, source, tlago.LoadOptions{LibraryPaths: libraryPaths})
	result := FrontendResult{Diagnostics: frontendDiagnostics(tlagoDiags)}
	if spec != nil && spec.Root != nil {
		result.Module = spec.Root.Name
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
	normalizeTlagoLambdas(spec)
	binderLayouts := normalizeTlagoApalacheAssignments(spec)
	ir, irDiags := tlago.ApalacheIRJSON(spec, tlago.ApalacheIROptions{IncludeSource: opts.IncludeSource})
	result.Diagnostics = append(result.Diagnostics, frontendDiagnostics(irDiags)...)
	if !hasErrors(result.Diagnostics) {
		result.IR = json.RawMessage(ir)
		result.Sources = NewSourceStore()
		native, conversionErr := UnmarshalSingleModuleIRJSON(ir, result.Sources)
		result.Native = normalizeTlagoLambdaValues(native, result.Sources)
		variantsImported := spec.Modules["Variants"] != nil
		result.Native = removeTlagoNativeVariantDeclarations(result.Native, result.Sources, variantsImported)
		result.Native = normalizeTlagoApalacheBuiltinCalls(result.Native, result.Sources)
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
		result.Native = typed.Module
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

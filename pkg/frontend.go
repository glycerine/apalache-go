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
	Module      string          `json:"module,omitempty"`
	IR          json.RawMessage `json:"ir,omitempty"`
	Diagnostics []Diagnostic    `json:"diagnostics"`
	Native      *Module         `json:"-"`
	Sources     *SourceStore    `json:"-"`
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

	spec, tlagoDiags := tlago.CheckSanySourceWithOptions(filename, source, tlago.LoadOptions{
		LibraryPaths: append([]string(nil), opts.LibraryPaths...),
	})
	result := FrontendResult{Diagnostics: frontendDiagnostics(tlagoDiags)}
	if spec != nil && spec.Root != nil {
		result.Module = spec.Root.Name
	}
	if hasErrors(result.Diagnostics) {
		return result
	}
	if err := contextError(ctx); err != nil {
		return canceledResult(err)
	}

	ir, irDiags := tlago.ApalacheIRJSON(spec, tlago.ApalacheIROptions{IncludeSource: opts.IncludeSource})
	result.Diagnostics = append(result.Diagnostics, frontendDiagnostics(irDiags)...)
	if !hasErrors(result.Diagnostics) {
		result.IR = json.RawMessage(ir)
		result.Sources = NewSourceStore()
		native, conversionErr := UnmarshalSingleModuleIRJSON(ir, result.Sources)
		result.Native = native
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

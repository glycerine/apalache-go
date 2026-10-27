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

// FrontendResult is the checked TLA+ frontend result. IR is canonical
// ApalacheIR JSON produced from tlago's checked AST. It is intentionally the
// first interchange boundary; the native Go Apalache IR will replace it as the
// subsequent port phases land.
type FrontendResult struct {
	Module      string          `json:"module,omitempty"`
	IR          json.RawMessage `json:"ir,omitempty"`
	Diagnostics []Diagnostic    `json:"diagnostics"`
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

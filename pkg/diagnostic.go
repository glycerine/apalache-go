package apalache

import (
	"fmt"
	"path/filepath"

	"github.com/glycerine/tlago"
)

// Severity classifies a diagnostic without exposing the frontend's concrete
// diagnostic types through Apalache's public API.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Position is a one-based, inclusive source range.
type Position struct {
	Filename  string `json:"filename,omitempty"`
	Line      int    `json:"line,omitempty"`
	Column    int    `json:"column,omitempty"`
	EndLine   int    `json:"endLine,omitempty"`
	EndColumn int    `json:"endColumn,omitempty"`
}

// Diagnostic is the stable diagnostic representation shared by the library
// and command-line interface.
type Diagnostic struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Position Position `json:"position,omitempty"`
}

func (d Diagnostic) Error() string {
	location := ""
	if d.Position.Filename != "" {
		location = d.Position.Filename
		if d.Position.Line > 0 {
			location += fmt.Sprintf(":%d:%d", d.Position.Line, d.Position.Column)
			if d.Position.EndLine > 0 {
				location += fmt.Sprintf("-%d:%d", d.Position.EndLine, d.Position.EndColumn)
			}
		}
		location += ": "
	}
	code := d.Code
	if code == "" {
		code = "APALACHE"
	}
	return fmt.Sprintf("%s%s %s: %s", location, d.Severity, code, d.Message)
}

func frontendDiagnostics(in tlago.Diagnostics) []Diagnostic {
	out := make([]Diagnostic, 0, len(in))
	for _, d := range in.Deduplicated() {
		out = append(out, Diagnostic{
			Code:     d.Code,
			Severity: Severity(d.Severity),
			Message:  d.Message,
			Position: Position{
				Filename:  d.Pos.File,
				Line:      d.Pos.Line,
				Column:    d.Pos.Column,
				EndLine:   d.Pos.EndLine,
				EndColumn: d.Pos.EndColumn,
			},
		})
	}
	return out
}

// normalizeFrontendDiagnosticFilenames removes parser mount paths and restores
// the root module's .tla basename. tlago parses an in-memory root from a
// temporary __source__ directory and Apalache IR source locations use the
// module name without an extension; neither implementation detail should leak
// into stable diagnostics.
func normalizeFrontendDiagnosticFilenames(diagnostics []Diagnostic, logicalFilename, rootModule string) {
	logicalBase := filepath.Base(logicalFilename)
	logicalStem := logicalBase[:len(logicalBase)-len(filepath.Ext(logicalBase))]
	for index := range diagnostics {
		filename := diagnostics[index].Position.Filename
		if filename == "" {
			continue
		}
		base := filepath.Base(filename)
		if base == logicalBase || base == logicalStem || filename == rootModule || base == rootModule {
			diagnostics[index].Position.Filename = logicalBase
			continue
		}
		if filepath.IsAbs(filename) || base != filename {
			diagnostics[index].Position.Filename = base
		}
	}
}

func hasErrors(diags []Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

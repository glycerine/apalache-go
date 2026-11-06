package pdr

// Source: pdr_context.cpp context::solve TRACE/IF_VERBOSE sites.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type pdrVerboseWriter struct{ stream *utext.NumericTextStream }

func (w pdrVerboseWriter) Write(body []byte) (int, error) {
	if err := w.stream.AppendCString(string(body)); err != nil {
		return 0, err
	}
	return len(body), nil
}

// Source ostream insertion failure only sets badbit; it does not throw out of
// context::solve or suppress computation of later operands.
type pdrDiagnosticWriter struct{ out pdrVerboseWriter }

func (w pdrDiagnosticWriter) Write(body []byte) (int, error) {
	_, _ = w.out.Write(body)
	return len(body), nil
}

func (c *Context) traceSolveStart() error {
	tags := c.manager.BorrowSourceTraceTags()
	err := tags.EmitConditional("pdr", "solve", "src/muz/pdr/pdr_context.cpp", 1888,
		func() bool { return true }, func(out io.Writer) error {
			_, err := io.WriteString(out, "solve\n")
			return err
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (c *Context) traceUnreachableCertificate() error {
	tags := c.manager.BorrowSourceTraceTags()
	err := tags.EmitConditional("pdr", "solve", "src/muz/pdr/pdr_context.cpp", 1909,
		func() bool { return true }, func(out io.Writer) error {
			return c.DisplayCertificate(out, c.queryDecl, false)
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func pdrVerboseStream(minimum uint32) (pdrVerboseWriter, bool, error) {
	enabled, err := diagnostic.ProcessVerboseEnabled(minimum)
	if err != nil || !enabled {
		return pdrVerboseWriter{}, false, err
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	return pdrVerboseWriter{stream: stream}, err == nil, err
}

func (c *Context) verboseReachableSearch() error {
	out, enabled, err := pdrVerboseStream(1)
	if err != nil || !enabled {
		return err
	}
	diagnosticOut := pdrDiagnosticWriter{out: out}
	_, _ = io.WriteString(diagnosticOut, "\n")
	return c.search.WriteDisplay(diagnosticOut)
}

func (c *Context) verboseReachableCertificate() error {
	enabled, err := diagnostic.ProcessVerboseEnabled(1)
	if err != nil || !enabled {
		return err
	}
	if !c.printBoogieCert {
		return nil
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	return c.DisplayCertificate(pdrDiagnosticWriter{out: pdrVerboseWriter{stream: stream}}, c.queryDecl, true)
}

func (c *Context) verboseUnreachableCertificate() error {
	out, enabled, err := pdrVerboseStream(1)
	if err != nil || !enabled {
		return err
	}
	return c.displayUnreachableCertificate(pdrDiagnosticWriter{out: out}, c.queryDecl, false)
}

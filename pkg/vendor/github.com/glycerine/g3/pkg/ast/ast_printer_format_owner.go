package ast

// Source: ast_smt2_pp.cpp's scoped format_ref and ast_pp_util.cpp's printer
// temporaries. A failed native finalizer keeps the concrete document and
// output stream available through a retryable cleanup receipt.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/memory"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type astPrinterFormatOwner struct {
	manager *ASTManager
	formats *ASTManager
	format  ASTRef
	stream  *utext.NumericTextStream
	nested  memory.ConstructionFailure
}

func (p *SimpleASTPrinterContext) reserveFormat() (*astPrinterFormatOwner, error) {
	if p == nil || p.manager == nil {
		return nil, ErrASTHandle
	}
	formats, err := p.manager.GetFormatManager()
	if err != nil {
		return nil, err
	}
	owner := &astPrinterFormatOwner{manager: p.manager, formats: formats}
	owner.format, err = formats.NewRef(ASTHandle{})
	if err != nil {
		return nil, owner.finish(err)
	}
	return owner, nil
}

func (o *astPrinterFormatOwner) adopt(format ASTHandle, cause error) error {
	if !format.IsNull() {
		cause = errors.Join(cause, o.formats.SetRef(o.format, format))
	}
	return cause
}

func (o *astPrinterFormatOwner) close() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	if o.stream != nil {
		if err := o.stream.Close(); err != nil {
			return err
		}
		o.stream = nil
	}
	if o.format != (ASTRef{}) {
		return o.formats.DelRef(&o.format)
	}
	return nil
}

func (o *astPrinterFormatOwner) finish(cause error) error {
	errors.As(cause, &o.nested)
	if cleanup := o.close(); cleanup != nil {
		reported := errors.Join(cause, cleanup)
		var runtime *memory.Runtime
		if numbers := o.manager.NumeralService(); numbers != nil {
			runtime = numbers.RuntimeOwner()
		}
		return &astPrinterFormatFailure{owner: o, runtime: runtime, original: cause, cleanup: cleanup, reported: reported, message: reported.Error()}
	}
	return cause
}

type astPrinterFormatFailure struct {
	owner                       *astPrinterFormatOwner
	runtime                     *memory.Runtime
	original, cleanup, reported error
	message                     string
}

func (f *astPrinterFormatFailure) Error() string                 { return f.message }
func (f *astPrinterFormatFailure) Unwrap() error                 { return f.reported }
func (f *astPrinterFormatFailure) OriginalCause() error          { return f.original }
func (f *astPrinterFormatFailure) CleanupCause() error           { return f.cleanup }
func (f *astPrinterFormatFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *astPrinterFormatFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}

var _ memory.ConstructionFailure = (*astPrinterFormatFailure)(nil)

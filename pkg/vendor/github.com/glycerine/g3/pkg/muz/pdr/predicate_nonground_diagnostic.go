package pdr

// Source: pred_transformer::init_rule, pdr_context.cpp:603-607.

import (
	"bytes"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (b *predicateRuleBuilder) nonGroundTailError(tail ast.ASTHandle) error {
	printer, err := ast.NewSimpleASTPrinterContext(b.transformer.manager, nil, nil)
	if err != nil {
		return err
	}
	var body bytes.Buffer
	if err = printer.DisplayExpr(&body, tail, 0); err != nil {
		return err
	}
	message := "PDR cannot solve non-ground tails: " + body.String()
	enabled, err := diagnostic.ProcessVerboseEnabled(0)
	if err != nil {
		return err
	}
	if enabled {
		stream, x := diagnostic.BorrowProcessVerboseStream()
		if x != nil {
			return x
		}
		// Source ostream errors are recorded on the stream and do not replace the exception.
		_ = stream.AppendCString(message + "\n")
	}
	return fmt.Errorf("%s", message)
}

package tactic

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/text"
)

func subpavingWriteDisplay(out io.Writer, text string) error {
	n, err := io.WriteString(out, text)
	if err != nil {
		return err
	}
	if n != len(text) {
		return io.ErrShortWrite
	}
	return nil
}

// writeSubpavingSMT2Expression follows the source mk_ismt2_pp route for every
// expression in the inverse variable map.
func writeSubpavingSMT2Expression(out io.Writer, manager *ast.ASTManager, expression ast.ASTHandle) (result error) {
	format, err := manager.SMT2PPExprTree(expression, nil)
	if err != nil {
		return err
	}
	formats, err := manager.GetFormatManager()
	if err != nil {
		return err
	}
	ref, err := formats.NewRef(format)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, formats.DelRef(&ref)) }()
	stream, err := text.NewNumericTextStream(text.NumericTextStreamConfig{Budget: manager.Budget(), BorrowedWriter: out})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	return manager.PrintFormat(stream, format, nil, nil, nil)
}

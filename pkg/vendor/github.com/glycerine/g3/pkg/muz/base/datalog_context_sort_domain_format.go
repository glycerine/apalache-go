package base

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// WriteConstant ports context::print_constant_name. Unregistered sorts use
// the raw table element number.
func (c *DatalogContext) WriteConstant(out io.Writer, sort ast.ASTHandle, number uint64) error {
	if err := c.open(); err != nil {
		return err
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	if _, found := c.sorts[sort]; !found {
		_, err := io.WriteString(out, formatUint64(number))
		return err
	}
	text, err := c.FormatConstant(sort, number)
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, text)
	return err
}

func formatUint64(value uint64) string {
	const digits = "0123456789"
	if value == 0 {
		return "0"
	}
	var buffer [20]byte
	index := len(buffer)
	for value != 0 {
		index--
		buffer[index] = digits[value%10]
		value /= 10
	}
	return string(buffer[index:])
}

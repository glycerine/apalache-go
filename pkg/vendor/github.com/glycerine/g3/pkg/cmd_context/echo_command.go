package cmd_context

// Source: src/cmd_context/basic_cmds.cpp:305-320, echo_cmd.

import (
	"io"
	"strings"
)

func echoCommandWrite(out io.Writer, value string) error {
	n, err := io.WriteString(out, value)
	if err == nil && n != len(value) {
		return io.ErrShortWrite
	}
	return err
}

// WriteEcho emits the source echo command representation. SMT-LIB-compliant
// mode surrounds the decoded string with quotes and doubles embedded quotes.
func (c *Context) WriteEcho(out io.Writer, value string) error {
	if err := c.valid(); err != nil {
		return err
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	compliant, err := c.Compliant()
	if err != nil {
		return err
	}
	if compliant {
		value = "\"" + strings.ReplaceAll(value, "\"", "\"\"") + "\""
	}
	return echoCommandWrite(out, value+"\n")
}

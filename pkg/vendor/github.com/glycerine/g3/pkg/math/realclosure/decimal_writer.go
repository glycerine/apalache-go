package realclosure

// Source: ostream insertion boundary used by realclosure.cpp:6149-6180.

import (
	"io"
	"strconv"
)

type realDecimalWriter struct{ writer io.Writer }

func (w realDecimalWriter) AppendInt32(value int32) error {
	_, err := io.WriteString(w.writer, strconv.FormatInt(int64(value), 10))
	return err
}

func (w realDecimalWriter) AppendCString(value string) error {
	_, err := io.WriteString(w.writer, value)
	return err
}

func (w realDecimalWriter) AppendNumeralText(value string) error {
	_, err := io.WriteString(w.writer, value)
	return err
}

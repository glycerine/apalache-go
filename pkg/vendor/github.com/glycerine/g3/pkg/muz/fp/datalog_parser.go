package fp

// Source: src/muz/fp/datalog_parser.{h,cpp}, retained legacy Datalog parser.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/glycerine/g3/pkg/muz/base"
)

// DatalogParseError reports the source location of a legacy Datalog grammar
// error. Line and Column are one-based.
type DatalogParseError struct {
	Path         string
	Line, Column int
	Message      string
}

func (e *DatalogParseError) Error() string {
	where := fmt.Sprintf("%d:%d", e.Line, e.Column)
	if e.Path != "" {
		where = e.Path + ":" + where
	}
	return where + ": " + e.Message
}

// ParseDatalog reads the legacy line-oriented Datalog format into context.
func ParseDatalog(context *base.DatalogContext, input io.Reader) error {
	return parseDatalog(context, input, "", "")
}

func ParseDatalogString(context *base.DatalogContext, input string) error {
	return parseDatalog(context, newStringReader(input), "", "")
}

func ParseDatalogFile(context *base.DatalogContext, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("muz/fp: open Datalog file: %w", err)
	}
	defer file.Close()
	return parseDatalog(context, file, path, filepath.Dir(path))
}

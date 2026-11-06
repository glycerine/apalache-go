package ast

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// EmitTrace emits one source-style trace record using this manager's trace
// configuration, or the process trace configuration when none was supplied.
func (m *ASTManager) EmitTrace(tag, function, file string, line int, body func(io.Writer) error) error {
	if m == nil || m.Self != m || m.Closed {
		return ErrASTHandle
	}
	tags := m.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	return tags.EmitConditional(tag, function, file, line, func() bool { return true }, body)
}

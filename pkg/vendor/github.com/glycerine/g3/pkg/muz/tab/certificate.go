package tab

// Source: src/muz/tab/tab_context.cpp imp::display_certificate and get_answer.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// DisplayCertificate prints a borrowed tabulation answer. Reachable answers
// are hyper-resolution proofs and unreachable answers are true.
func DisplayCertificate(manager *ast.ASTManager, out io.Writer, answer ast.ASTRef) error {
	if manager == nil {
		return ast.ErrASTHandle
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	handle, err := manager.RefGet(answer)
	if err != nil {
		return err
	}
	if err = ast.DisplayLegacySMT(out, manager, handle, 0, nil); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out)
	return err
}

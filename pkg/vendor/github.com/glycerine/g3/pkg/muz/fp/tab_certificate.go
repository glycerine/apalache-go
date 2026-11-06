package fp

// Source: src/muz/tab/tab_context.cpp tab::display_certificate and
// src/muz/base/dl_context.cpp context::display_certificate.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/tab"
)

func (e *PropositionalEngine) writeTabCertificate(out io.Writer, declaration ast.ASTHandle) (found bool, resultErr error) {
	if err := e.requireResultSnapshot(EngineTab); err != nil {
		return false, err
	}
	answer, err := e.Answer(declaration)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&answer)) }()
	return true, tab.DisplayCertificate(e.manager, out, answer)
}

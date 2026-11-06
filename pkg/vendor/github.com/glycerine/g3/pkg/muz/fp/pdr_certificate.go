package fp

// Source: pdr::context::display_certificate in pdr_context.cpp and
// pdr_dl_interface::display_certificate forwarding.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *PropositionalEngine) pdrUnknownAnswer(status lbool.LBool) (ast.ASTRef, bool, error) {
	if status != lbool.Undef {
		return ast.ASTRef{}, false, nil
	}
	truth, err := e.manager.MkTrue()
	if err != nil {
		return ast.ASTRef{}, true, err
	}
	answer, err := e.manager.NewRef(truth)
	return answer, true, err
}

func writePDRUnknownCertificate(out io.Writer, status lbool.LBool) (bool, error) {
	if status != lbool.Undef {
		return false, nil
	}
	_, err := io.WriteString(out, "unknown")
	return true, err
}

func (e *PropositionalEngine) pdrBoogieCertificateEnabled() (_ bool, resultErr error) {
	view, err := e.fixedpointParamView()
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, view.Close()) }()
	return view.Bool("print_boogie_certificate", false)
}

func (e *PropositionalEngine) writePDRBoogieCertificate(out io.Writer, declaration ast.ASTHandle) (resultErr error) {
	proof, err := e.Proof(declaration)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&proof)) }()
	handle, err := e.manager.RefGet(proof)
	if err != nil {
		return err
	}
	certificate, err := base.NewBoogieProof(e.manager, base.BoogieProofConfig{Context: e.params, Globals: e.globals, Diagnostic: io.Discard})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, certificate.Close()) }()
	if err = certificate.SetProof(handle); err != nil {
		return err
	}
	return certificate.PP(out)
}

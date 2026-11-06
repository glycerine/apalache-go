package pdr

// Source: context::display_certificate in src/muz/pdr/pdr_context.cpp.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// DisplayCertificate writes the source diagnostic certificate for the latest
// result. boogie selects the source print_boogie_certificate presentation for
// reachable proof traces; queryDeclaration must identify the latest query.
func (c *Context) DisplayCertificate(out io.Writer, queryDeclaration ast.ASTHandle, boogie bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if c.queryDecl != queryDeclaration {
		return ast.ErrASTDomain
	}
	switch c.lastResult {
	case lbool.Undef:
		_, err := io.WriteString(out, "unknown")
		return err
	case lbool.False:
		return c.displayUnreachableCertificate(out, queryDeclaration, true)
	case lbool.True:
		if boogie {
			return c.displayBoogieCertificate(out, queryDeclaration)
		}
		return c.displayReachableCertificate(out, queryDeclaration)
	default:
		return ast.ErrASTDomain
	}
}

func (c *Context) displayBoogieCertificate(out io.Writer, queryDeclaration ast.ASTHandle) error {
	attempt := newPDRValidationAttempt(c.manager)
	proofsEnabled, err := c.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	if !proofsEnabled {
		if err = c.manager.ToggleProofMode(ast.ASTProofEnabled); err != nil {
			return err
		}
		attempt.proofModes = append(attempt.proofModes, ast.ASTProofDisabled)
	}
	proof, err := c.Proof(queryDeclaration)
	if err != nil {
		return finishPDRValidation(attempt, fmt.Errorf("pdr: construct Boogie certificate proof: %w", err))
	}
	attempt.references = append(attempt.references, proof)
	handle, err := c.manager.RefGet(proof)
	if err != nil {
		return finishPDRValidation(attempt, err)
	}
	config := c.owner.contexts.config
	certificate, err := base.NewBoogieProof(c.manager, base.BoogieProofConfig{Context: config.Params, Globals: config.Globals, Diagnostic: io.Discard})
	if err != nil {
		return finishPDRValidation(attempt, err)
	}
	attempt.boogieProofs = append(attempt.boogieProofs, certificate)
	if err = certificate.SetProof(handle); err != nil {
		return finishPDRValidation(attempt, fmt.Errorf("pdr: normalize Boogie certificate proof: %w", err))
	}
	if err = certificate.SetModel(nil); err != nil {
		return finishPDRValidation(attempt, err)
	}
	if err = certificate.PP(out); err != nil {
		err = fmt.Errorf("pdr: write Boogie certificate proof: %w", err)
	}
	return finishPDRValidation(attempt, err)
}

func (c *Context) displayReachableCertificate(out io.Writer, queryDeclaration ast.ASTHandle) error {
	answer, err := c.Answer(queryDeclaration)
	if err != nil {
		return err
	}
	attempt := newPDRValidationAttempt(c.manager)
	attempt.references = append(attempt.references, answer)
	handle, err := c.manager.RefGet(answer)
	if err != nil {
		return finishPDRValidation(attempt, err)
	}
	printer, err := ast.NewSimpleASTPrinterContext(c.manager, out, nil)
	if err != nil {
		return finishPDRValidation(attempt, err)
	}
	return finishPDRValidation(attempt, printer.DisplayExpr(out, handle, 0))
}

func (c *Context) displayUnreachableCertificate(out io.Writer, queryDeclaration ast.ASTHandle, converted bool) error {
	certificate, err := c.finiteFrameModel(c.inductive, queryDeclaration)
	if err != nil {
		return err
	}
	if converted && c.certificateModel != nil {
		certificate, err = c.certificateModel.ConvertCertificateModel(certificate)
		if err != nil {
			return err
		}
		if certificate == nil {
			return ast.ErrASTHandle
		}
	}
	attempt := newPDRValidationAttempt(c.manager)
	attempt.modelOwners = append(attempt.modelOwners, &pdrModelExportOwner{manager: c.manager, model: certificate})
	config := c.owner.contexts.config
	return finishPDRValidation(attempt, certificate.Core().WriteSMT2(out, config.Params, config.Globals, 0))
}

package duality

// Source: duality_solver.cpp, ConjectureFileReporter::Update.
import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
	"io"
)

// ConjectureReporter borrows its destination and formatting services. The
// owning engine closes the file after search; direct callers own their writer.
type ConjectureReporter struct {
	Output  io.Writer
	Params  *uparams.ParamsContext
	Globals *uparams.GlobalParamsService
}

func (r *ConjectureReporter) writeTerm(m *ast.ASTManager, h ast.ASTHandle, sort bool) (result error) {
	var format ast.ASTHandle
	var err error
	if sort {
		format, err = m.SMT2PPSort(h)
	} else {
		format, err = m.SMT2PPExprTree(h, nil)
	}
	if err != nil {
		return err
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	empty := r.Params.EmptyValue()
	if err := m.PrintFormat(stream, format, r.Params, &empty, r.Globals); err != nil {
		return err
	}
	text, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(r.Output, text)
	return err
}

// Update writes the incoming fact before it is intersected with annotations.
// Eager and lazy updates use the same source file format.
func (r *ConjectureReporter) Update(node *Node, update *Transformer) error {
	if r == nil {
		return nil
	}
	if r.Output == nil || r.Params == nil || r.Globals == nil || node == nil || node.closed || update == nil {
		return ErrWrapperOwner
	}
	if err := update.open(); err != nil {
		return err
	}
	m := node.Owner.context.manager
	if update.context != node.Owner.context {
		return ErrContextMismatch
	}
	decl, err := m.RefGet(node.Name)
	if err != nil {
		return err
	}
	name, err := m.GetASTName(decl)
	if err != nil {
		return err
	}
	text, err := m.Symbols.String(name)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(r.Output, "(define-fun %s (", text); err != nil {
		return err
	}
	for i, ref := range update.individualParams {
		if i > 0 {
			if _, err := io.WriteString(r.Output, " "); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(r.Output, "("); err != nil {
			return err
		}
		h, err := m.RefGet(ref)
		if err != nil {
			return err
		}
		if err := r.writeTerm(m, h, false); err != nil {
			return err
		}
		if _, err := io.WriteString(r.Output, " "); err != nil {
			return err
		}
		sort, err := m.GetSort(h)
		if err != nil {
			return err
		}
		if err := r.writeTerm(m, sort, true); err != nil {
			return err
		}
		if _, err := io.WriteString(r.Output, ")"); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(r.Output, ") Bool \n"); err != nil {
		return err
	}
	h, err := m.RefGet(update.formula)
	if err != nil {
		return err
	}
	if err := r.writeTerm(m, h, false); err != nil {
		return err
	}
	_, err = io.WriteString(r.Output, ")\n\n")
	return err
}

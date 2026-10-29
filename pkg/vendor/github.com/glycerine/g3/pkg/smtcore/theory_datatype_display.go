package smtcore

// Source: src/smt/theory_datatype.cpp display and display_var.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (t *SMTDatatypeTheory) Display(out io.Writer) error {
	if _, err := t.State(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if t.NumVariables() == 0 {
		return nil
	}
	if _, err := io.WriteString(out, "Theory datatype:\n"); err != nil {
		return err
	}
	for variable := range t.variables {
		if err := t.DisplayVariable(out, SMTTheoryVar(variable)); err != nil {
			return err
		}
	}
	return nil
}

func (t *SMTDatatypeTheory) DisplayVariable(out io.Writer, variable SMTTheoryVar) (result error) {
	s, err := t.State()
	if err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	node, err := t.Enode(variable)
	if err != nil {
		return err
	}
	id, err := node.OwnerID()
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(out, "v%d #%d -> v%d ", variable, id, t.find.Find(uint32(variable))); err != nil {
		return err
	}
	// Source prints this variable's own metadata, even for a non-root.
	constructor := t.data[variable].constructor
	if constructor == nil {
		_, err = io.WriteString(out, "(null)\n")
		return err
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: s.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	if err = s.manager.DisplayBoundedLowLevel(stream, constructor.owner, 3); err != nil {
		return err
	}
	body, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, body+"\n")
	return err
}

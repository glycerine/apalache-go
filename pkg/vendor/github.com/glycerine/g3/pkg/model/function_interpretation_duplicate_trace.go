package model

// Source: src/model/func_interp.cpp:200-216; src/util/trace.h:40-46.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (f *FunctionInterpretation) traceDuplicateNewEntry(old *FunctionEntry, args []ast.ASTHandle, result ast.ASTHandle) error {
	tags := f.manager.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("func_interp_bug") {
		return nil
	}
	return tags.EmitConditional("func_interp_bug", "insert_new_entry", "src/model/func_interp.cpp", 202,
		func() bool { return true }, func(w io.Writer) error {
			write := func(s string) error { _, err := io.WriteString(w, s); return err }
			print := func(h ast.ASTHandle) error { return f.writeTraceSMT2(w, h) }
			if err := write("Old: "); err != nil {
				return err
			}
			if err := print(old.result); err != nil {
				return err
			}
			if err := write("\nArgs:"); err != nil {
				return err
			}
			for _, arg := range old.args {
				if err := print(arg); err != nil {
					return err
				}
				if err := write("\n"); err != nil {
					return err
				}
			}
			if err := write("New: "); err != nil {
				return err
			}
			if err := print(result); err != nil {
				return err
			}
			if err := write("\nArgs:"); err != nil {
				return err
			}
			for _, arg := range args {
				if err := print(arg); err != nil {
					return err
				}
				if err := write("\n"); err != nil {
					return err
				}
			}
			if err := write("Old: "); err != nil {
				return err
			}
			if err := print(old.result); err != nil {
				return err
			}
			return write("\n")
		})
}

func (f *FunctionInterpretation) writeTraceSMT2(w io.Writer, h ast.ASTHandle) (result error) {
	globals := uparams.BorrowProcessGlobalParamsService()
	if globals == nil || globals.Context == nil {
		return uparams.ErrGlobalParamsOwner
	}
	format, err := f.manager.SMT2PPExprTree(h, nil)
	if err != nil {
		return err
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: f.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	empty := globals.Context.EmptyValue()
	if err = f.manager.PrintFormat(stream, format, globals.Context, &empty, globals); err != nil {
		return err
	}
	body, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, body)
	return err
}

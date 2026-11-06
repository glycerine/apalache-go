package api

// Source: generated api_commands.cpp, api_ast.cpp and api_parsers.cpp.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

func replayBenchmarkString(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	fields := make([]string, 4)
	for i := range fields {
		fields[i], err = r.String(uint32(i + 1))
		if err != nil {
			return err
		}
	}
	assumptions, err := replayASTArray(r, owner, 5, 6)
	if err != nil {
		return err
	}
	formula, err := r.astObject(owner, 7)
	if err != nil {
		return err
	}
	_, err = owner.context.BenchmarkSMT2(formula.handle, APIBenchmarkConfig{
		Name: fields[0], Logic: fields[1], Status: fields[2], Attributes: fields[3], Assumptions: assumptions,
	})
	return err
}

func replayEvaluateSMT2(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	input, err := r.String(1)
	if err != nil {
		return err
	}
	output, err := owner.context.EvaluateSMT2(input)
	if err != nil {
		owner.parserError = output
		if owner.parserError == "" {
			owner.parserError = err.Error()
		}
		owner.results.setErrorCode(4) // Z3_PARSER_ERROR.
	}
	return nil
}

func replayParseSMT2String(r *Z3Replayer) error { return replayParseSMT2(r, false) }
func replayParseSMT2File(r *Z3Replayer) error   { return replayParseSMT2(r, true) }

func replayParseSMT2(r *Z3Replayer, file bool) (err error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	input, err := r.String(1)
	if err != nil {
		return err
	}
	var bindings SMT2Bindings
	for _, positions := range [][3]uint32{{2, 3, 4}, {5, 6, 7}} {
		count, e := r.Uint(positions[0])
		if e != nil {
			return e
		}
		array, e := r.value(positions[1], replaySymbolArray)
		if e != nil {
			return e
		}
		names := r.symbolArrays[array.u]
		values, e := r.ObjectArray(positions[2])
		if e != nil {
			return e
		}
		if uint64(count) > uint64(len(names)) || uint64(count) > uint64(len(values)) {
			return ErrZ3ReplayReference
		}
		for i := uint32(0); i < count; i++ {
			if names[i].numericSymbol || names[i].nullSymbol {
				return fmt.Errorf("parser replay: numeric and null binding symbols are not supported")
			}
			value, e := r.astByID(owner, values[i])
			if e != nil {
				return e
			}
			if positions[0] == 2 {
				bindings.Sorts = append(bindings.Sorts, SMT2SortBinding{Name: names[i].s, Sort: value.handle})
			} else {
				bindings.Declarations = append(bindings.Declarations, SMT2DeclarationBinding{Name: names[i].s, Declaration: value.handle})
			}
		}
	}
	var reader io.Reader = strings.NewReader(input)
	if file {
		f, e := os.Open(input)
		if e != nil {
			owner.results.setErrorCode(4)
			return r.StoreResult(0)
		}
		defer func() { err = errors.Join(err, f.Close()) }()
		reader = f
	}
	ref, e := owner.context.ParseSMT2(reader, bindings)
	if e != nil {
		owner.parserError = e.Error()
		owner.results.setErrorCode(4)
		value, x := owner.results.manager.MkTrue()
		if x != nil {
			return x
		}
		return r.storeAST(owner, value)
	}
	defer func() { err = errors.Join(err, owner.results.manager.DelRef(&ref)) }()
	value, err := owner.results.manager.RefGet(ref)
	if err != nil {
		return err
	}
	return r.storeAST(owner, value)
}

func replayGetParserError(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	owner.results.setErrorCode(0)
	_ = owner.parserError
	return nil
}

func replayGetErrorCode(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	_ = owner.results.errorCode
	return nil
}

func replaySetErrorCode(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	code, err := r.Uint(1)
	if err != nil {
		return err
	}
	owner.results.setErrorCode(code)
	return nil
}

func replayErrorMessage(r *Z3Replayer) error {
	id, err := r.Object(0)
	if err != nil {
		return err
	}
	if id != 0 && r.contexts[id] == nil {
		return ErrZ3ReplayReference
	}
	_, err = r.Uint(1)
	return err
}

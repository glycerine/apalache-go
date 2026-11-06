package api

// Source: generated api_commands.cpp, Z3_mk_pattern through Z3_mk_quantifier_ex.

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func registerZ3GeneratedQuantifierCommands(r *Z3Replayer) error {
	entries := []struct {
		id      uint32
		name    string
		command ReplayCommand
	}{
		{190, "Z3_mk_pattern", replayPattern}, {191, "Z3_mk_bound", replayBound},
		{192, "Z3_mk_forall", func(r *Z3Replayer) error { return replayQuantifier(r, 192) }},
		{193, "Z3_mk_exists", func(r *Z3Replayer) error { return replayQuantifier(r, 193) }},
		{194, "Z3_mk_quantifier", func(r *Z3Replayer) error { return replayQuantifier(r, 194) }},
		{195, "Z3_mk_quantifier_ex", func(r *Z3Replayer) error { return replayQuantifier(r, 195) }},
		{196, "Z3_mk_forall_const", func(r *Z3Replayer) error { return replayConstantQuantifier(r, 196) }},
		{197, "Z3_mk_exists_const", func(r *Z3Replayer) error { return replayConstantQuantifier(r, 197) }},
		{198, "Z3_mk_quantifier_const", func(r *Z3Replayer) error { return replayConstantQuantifier(r, 198) }},
		{199, "Z3_mk_quantifier_const_ex", func(r *Z3Replayer) error { return replayConstantQuantifier(r, 199) }},
	}
	for _, entry := range entries {
		if err := r.RegisterCommand(entry.id, entry.name, entry.command); err != nil {
			return err
		}
	}
	return nil
}
func replayPattern(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	terms, err := replayASTArray(r, owner, 1, 2)
	if err != nil {
		return err
	}
	h, err := owner.context.Pattern(terms...)
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayBound(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	index, err := r.Uint(1)
	if err != nil {
		return err
	}
	sort, err := r.astObject(owner, 2)
	if err != nil {
		return err
	}
	h, err := owner.context.BoundVariable(index, sort.handle)
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}

// replayQuantifier keeps numerical and string binder symbols distinct, as the
// generated ABI does. The ordinary and extended signatures use shifted slots.
func replayQuantifier(r *Z3Replayer, id uint32) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	forall := id == 192
	weightPos, patternCount, patternArray, declCount, sortArray, nameArray, bodyPos := uint32(1), uint32(2), uint32(3), uint32(4), uint32(5), uint32(6), uint32(7)
	if id == 194 || id == 195 {
		forall, err = r.Bool(1)
		if err != nil {
			return err
		}
		weightPos, patternCount, patternArray, declCount, sortArray, nameArray, bodyPos = 2, 3, 4, 5, 6, 7, 8
	}
	var qid, skid usymbol.Symbol
	var noPatterns []ast.ASTHandle
	if id == 195 {
		qid, err = replayTypedSymbol(r, owner, 3)
		if err != nil {
			return err
		}
		skid, err = replayTypedSymbol(r, owner, 4)
		if err != nil {
			return err
		}
		patternCount, patternArray, declCount, sortArray, nameArray, bodyPos = 5, 6, 9, 10, 11, 12
		noPatterns, err = replayASTArray(r, owner, 7, 8)
		if err != nil {
			return err
		}
	}
	weight, err := r.Uint(weightPos)
	if err != nil {
		return err
	}
	patterns, err := replayASTArray(r, owner, patternCount, patternArray)
	if err != nil {
		return err
	}
	sorts, err := replayASTArray(r, owner, declCount, sortArray)
	if err != nil {
		return err
	}
	names, err := replaySymbolValues(r, owner, nameArray, uint32(len(sorts)))
	if err != nil {
		return err
	}
	body, err := r.astObject(owner, bodyPos)
	if err != nil {
		return err
	}
	m := owner.results.manager
	bodySort, err := m.GetSort(body.handle)
	if err != nil {
		return err
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return err
	}
	if bodySort != boolean || len(patterns) != 0 && len(noPatterns) != 0 {
		return ast.ErrASTDomain
	}
	if err = validateQuantifierPatterns(m, qid, uint32(len(sorts)), patterns); err != nil {
		return err
	}
	h := body.handle
	if len(sorts) != 0 {
		h, err = m.MkQuantifier(ast.ASTQuantifierSpec{Forall: forall, Sorts: sorts, Names: names, Body: body.handle, Weight: int32(weight), QID: qid, SkolemID: skid, Patterns: patterns, NoPatterns: noPatterns})
		if err != nil {
			return err
		}
	}
	return r.storeAST(owner, h)
}

// replayConstantQuantifier abstracts named constants in both body and patterns.
func replayConstantQuantifier(r *Z3Replayer, id uint32) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	forall := id == 196
	weightPos, boundCount, boundArray, patternCount, patternArray, bodyPos := uint32(1), uint32(2), uint32(3), uint32(4), uint32(5), uint32(6)
	if id == 198 || id == 199 {
		forall, err = r.Bool(1)
		if err != nil {
			return err
		}
		weightPos, boundCount, boundArray, patternCount, patternArray, bodyPos = 2, 3, 4, 5, 6, 7
	}
	var qid, skid usymbol.Symbol
	var noPatterns []ast.ASTHandle
	if id == 199 {
		qid, err = replayTypedSymbol(r, owner, 3)
		if err != nil {
			return err
		}
		skid, err = replayTypedSymbol(r, owner, 4)
		if err != nil {
			return err
		}
		boundCount, boundArray, patternCount, patternArray, bodyPos = 5, 6, 7, 8, 11
		noPatterns, err = replayASTArray(r, owner, 9, 10)
		if err != nil {
			return err
		}
	}
	weight, err := r.Uint(weightPos)
	if err != nil {
		return err
	}
	bound, err := replayASTArray(r, owner, boundCount, boundArray)
	if err != nil {
		return err
	}
	patterns, err := replayASTArray(r, owner, patternCount, patternArray)
	if err != nil {
		return err
	}
	body, err := r.astObject(owner, bodyPos)
	if err != nil {
		return err
	}
	h, err := owner.context.constantQuantifierSymbols(ConstantQuantifierSpec{Forall: forall, Bound: bound, Body: body.handle, Weight: int32(weight), Patterns: patterns, NoPatterns: noPatterns}, qid, skid)
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}

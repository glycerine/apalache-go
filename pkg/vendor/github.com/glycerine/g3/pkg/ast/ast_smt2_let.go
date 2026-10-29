package ast

// Source: src/ast/ast_smt2_pp.cpp:801-844.

import "strconv"

// smt2LetBinding records an alias definition and the level of aliases used
// by that definition. Definitions at the same level share one let form.
type smt2LetBinding struct {
	name   string
	format ASTHandle
	level  uint32
}

func (m *ASTManager) smt2PPLet(body ASTHandle, bindings []smt2LetBinding) (ASTHandle, error) {
	return m.smt2PPLetAt(body, bindings, 0)
}

// oldSize is the enclosing alias count captured by begin_scope. The native
// binding slice contains only declarations created in this scope.
func (m *ASTManager) smt2PPLetAt(body ASTHandle, bindings []smt2LetBinding, oldSize int) (ASTHandle, error) {
	if err := m.traceSMT2Let(806, "old_sz: "+strconv.Itoa(oldSize)+", sz: ", oldSize+len(bindings)); err != nil {
		return ASTHandle{}, err
	}
	if len(bindings) == 0 {
		return body, nil
	}
	var levels [][]ASTHandle
	for _, binding := range bindings {
		for uint32(len(levels)) <= binding.level {
			levels = append(levels, nil)
		}
		decl, err := m.FormatSeq1(binding.name, []ASTHandle{binding.format})
		if err != nil {
			return ASTHandle{}, err
		}
		levels[binding.level] = append(levels[binding.level], decl)
	}
	if err := m.traceSMT2Let(818, "decls.size(): ", len(levels)); err != nil {
		return ASTHandle{}, err
	}
	parts := make([]ASTHandle, 0, 4*len(levels)+2)
	open := 0
	for _, decls := range levels {
		if len(decls) == 0 {
			continue
		}
		if open > 0 {
			line, err := m.FormatLineBreak()
			if err != nil {
				return ASTHandle{}, err
			}
			parts = append(parts, line)
		}
		open++
		prefix, err := m.FormatString("(let ")
		if err != nil {
			return ASTHandle{}, err
		}
		group, err := m.FormatSeq5(decls)
		if err != nil {
			return ASTHandle{}, err
		}
		group, err = m.FormatIndent(5, group)
		if err != nil {
			return ASTHandle{}, err
		}
		parts = append(parts, prefix, group)
	}
	if err := m.traceSMT2Let(833, "num_op: ", open); err != nil {
		return ASTHandle{}, err
	}
	line, err := m.FormatLineBreak()
	if err != nil {
		return ASTHandle{}, err
	}
	inner, err := m.FormatCompose([]ASTHandle{line, body})
	if err != nil {
		return ASTHandle{}, err
	}
	inner, err = m.FormatIndent(2, inner)
	if err != nil {
		return ASTHandle{}, err
	}
	parts = append(parts, inner)
	for i := 0; i < open; i++ {
		close, err := m.FormatString(")")
		if err != nil {
			return ASTHandle{}, err
		}
		parts = append(parts, close)
	}
	return m.FormatCompose(parts)
}

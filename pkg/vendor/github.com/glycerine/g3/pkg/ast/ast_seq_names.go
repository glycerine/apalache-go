package ast

// Source: src/ast/seq_decl_plugin.cpp:826-842.

import usymbol "github.com/glycerine/g3/pkg/util/symbol"

// GetSequenceOpNames appends every published source sequence signature. The
// logic parameter is intentionally ignored by the source plugin.
func (m *ASTManager) GetSequenceOpNames(names *ASTBuiltinNames, _ usymbol.Symbol) error {
	if err := m.lock(); err != nil {
		return err
	}
	defer m.mu.Unlock()
	p := m.sequencePlugin
	if names == nil || p == nil || !p.initialized {
		return ErrASTHandle
	}
	for kind, signature := range p.signatures {
		if signature != nil {
			names.items = append(names.items, ASTBuiltinName{Kind: int32(kind), Name: signature.name})
		}
	}
	return nil
}

// GetSequenceSortNames appends the source sequence sort aliases.
func (m *ASTManager) GetSequenceSortNames(names *ASTBuiltinNames, _ usymbol.Symbol) error {
	if err := m.lock(); err != nil {
		return err
	}
	defer m.mu.Unlock()
	p := m.sequencePlugin
	if names == nil || p == nil || !p.initialized {
		return ErrASTHandle
	}
	for _, entry := range [...]struct {
		name string
		kind int32
	}{{"Seq", 0}, {"RegEx", 1}, {"String", 2}, {"StringSequence", 2}} {
		if err := m.appendBuiltinName(names, entry.name, entry.kind); err != nil {
			return err
		}
	}
	return nil
}

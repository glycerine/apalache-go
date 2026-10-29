package smtcore

// Source: active implementation in src/smt/smt_cg_table.h and
// src/smt/smt_cg_table.cpp.

import (
	"encoding/binary"
	"github.com/glycerine/g3/pkg/ast"
)

// Native Go signature keys replace the source per-declaration unary, binary,
// commutative-binary and n-ary hash tables. Binary keys use root pointers
// directly; n-ary suffix owner identities select collision buckets whose
// entries still pass the exact source congruence comparison.
type smtCongruenceKey struct {
	first, second *SMTEnode
	tail          string
}
type smtCongruenceBucket struct {
	id      uint32
	entries map[smtCongruenceKey][]*SMTEnode
}

func smtCongruenceSignature(node *SMTEnode) smtCongruenceKey {
	key := smtCongruenceKey{first: node.arguments[0].root}
	if len(node.arguments) > 1 {
		key.second = node.arguments[1].root
	}
	if len(node.arguments) > 2 {
		data := make([]byte, 0, 16*(len(node.arguments)-2))
		for _, arg := range node.arguments[2:] {
			h := arg.root.owner
			data = binary.LittleEndian.AppendUint64(data, h.Origin())
			data = binary.LittleEndian.AppendUint32(data, h.Slot())
			data = binary.LittleEndian.AppendUint32(data, h.Generation())
		}
		key.tail = string(data)
	}
	return key
}

type SMTCongruenceInsertResult struct {
	Node     *SMTEnode
	Commuted bool
}

type SMTCongruenceTable struct {
	self         *SMTCongruenceTable
	manager      *ast.ASTManager
	tables       map[ast.ASTHandle]*smtCongruenceBucket
	declarations []ast.ASTHandle
	resetIndex   int
	closed       bool
}

func NewSMTCongruenceTable(manager *ast.ASTManager) (*SMTCongruenceTable, error) {
	if manager == nil {
		return nil, ast.ErrASTDomain
	}
	table := &SMTCongruenceTable{manager: manager, tables: make(map[ast.ASTHandle]*smtCongruenceBucket)}
	table.self = table
	return table, nil
}

func (t *SMTCongruenceTable) open() error {
	if t == nil || t.self != t || t.manager == nil || t.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (t *SMTCongruenceTable) validateNode(node *SMTEnode) error {
	if err := t.open(); err != nil {
		return err
	}
	if node == nil || node.manager != t.manager || len(node.arguments) == 0 {
		return ast.ErrASTDomain
	}
	return node.valid()
}

func (t *SMTCongruenceTable) ensureTable(node *SMTEnode) error {
	if bucket, found := t.tables[node.declaration]; found {
		node.functionDeclID = bucket.id
		return nil
	}
	if err := t.manager.IncRef(node.declaration); err != nil {
		return err
	}
	node.functionDeclID = uint32(len(t.declarations))
	t.declarations = append(t.declarations, node.declaration)
	t.tables[node.declaration] = &smtCongruenceBucket{id: node.functionDeclID, entries: make(map[smtCongruenceKey][]*SMTEnode)}
	return nil
}

func (t *SMTCongruenceTable) Insert(node *SMTEnode) (SMTCongruenceInsertResult, error) {
	if err := t.validateNode(node); err != nil {
		return SMTCongruenceInsertResult{}, err
	}
	if err := t.ensureTable(node); err != nil {
		return SMTCongruenceInsertResult{}, err
	}
	if candidate, commuted, err := t.Find(node); err != nil {
		return SMTCongruenceInsertResult{}, err
	} else if candidate != nil {
		return SMTCongruenceInsertResult{Node: candidate, Commuted: commuted}, nil
	}
	key := smtCongruenceSignature(node)
	bucket := t.tables[node.declaration]
	bucket.entries[key] = append(bucket.entries[key], node)
	return SMTCongruenceInsertResult{Node: node}, nil
}

func (t *SMTCongruenceTable) Find(node *SMTEnode) (*SMTEnode, bool, error) {
	if err := t.validateNode(node); err != nil {
		return nil, false, err
	}
	bucket := t.tables[node.declaration]
	if bucket == nil {
		return nil, false, nil
	}
	key := smtCongruenceSignature(node)
	if found, commuted, err := smtFindCongruence(bucket.entries[key], node); found != nil || err != nil {
		return found, commuted, err
	}
	if node.commutative && len(node.arguments) == 2 && key.first != key.second {
		key.first, key.second = key.second, key.first
		return smtFindCongruence(bucket.entries[key], node)
	}
	return nil, false, nil
}

func smtFindCongruence(entries []*SMTEnode, node *SMTEnode) (*SMTEnode, bool, error) {
	for _, candidate := range entries {
		congruent, commuted, err := SMTEnodesCongruent(candidate, node)
		if err != nil {
			return nil, false, err
		}
		if congruent {
			return candidate, commuted, nil
		}
	}
	return nil, false, nil
}

func (t *SMTCongruenceTable) Contains(node *SMTEnode) (bool, error) {
	found, _, err := t.Find(node)
	return found != nil, err
}

func (t *SMTCongruenceTable) ContainsPointer(node *SMTEnode) (bool, error) {
	found, _, err := t.Find(node)
	return found == node, err
}

func (t *SMTCongruenceTable) Erase(node *SMTEnode) (bool, error) {
	if err := t.validateNode(node); err != nil {
		return false, err
	}
	candidate, _, err := t.Find(node)
	if err != nil {
		return false, err
	}
	if candidate == nil {
		return false, nil
	}
	key := smtCongruenceSignature(candidate)
	entries := t.tables[node.declaration].entries
	bucket := entries[key]
	for i, entry := range bucket {
		if entry != candidate {
			continue
		}
		copy(bucket[i:], bucket[i+1:])
		bucket[len(bucket)-1] = nil
		if len(bucket) == 1 {
			delete(entries, key)
		} else {
			entries[key] = bucket[:len(bucket)-1]
		}
		return true, nil
	}
	return false, nil
}

func (t *SMTCongruenceTable) Reset() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.resetIndex == 0 {
		t.resetIndex = len(t.declarations)
	}
	for t.resetIndex > 0 {
		i := t.resetIndex - 1
		handle := t.declarations[i]
		consumed, err := t.manager.DecRefConsumed(handle)
		if consumed {
			t.declarations[i] = ast.ASTHandle{}
			t.resetIndex--
		}
		if err != nil {
			return err
		}
	}
	t.declarations = nil
	t.tables = make(map[ast.ASTHandle]*smtCongruenceBucket)
	return nil
}

func (t *SMTCongruenceTable) CheckInvariant() error {
	if err := t.open(); err != nil {
		return err
	}
	for _, bucket := range t.tables {
		for _, entries := range bucket.entries {
			for _, node := range entries {
				contained, err := t.ContainsPointer(node)
				if err != nil {
					return err
				}
				if !contained {
					return ast.ErrASTDomain
				}
			}
		}
	}
	return nil
}

// Display and DisplayCompact are intentionally empty in the active source
// implementation.
func (t *SMTCongruenceTable) Display() error        { return t.open() }
func (t *SMTCongruenceTable) DisplayCompact() error { return t.open() }

func (t *SMTCongruenceTable) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if err := t.Reset(); err != nil {
		return err
	}
	t.manager = nil
	t.closed = true
	return nil
}

package smtcore

// Source: src/smt/smt_enode.cpp context-trailed generation and label updates,
// and the corresponding label-hash interface in src/smt/smt_enode.h.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/approx"
	uhash "github.com/glycerine/g3/pkg/util/hash"
)

type smtEnodeTrailKind uint8

const (
	smtEnodeTrailGeneration smtEnodeTrailKind = iota
	smtEnodeTrailLabelHash
	smtEnodeTrailRootLabels
)

type smtEnodeTrailEntry struct {
	kind       smtEnodeTrailKind
	node       *SMTEnode
	generation uint32
	labelHash  int8
	rootLabels approx.UintSet
}

// SMTEnodeTrail owns concrete snapshots of the enode fields changed by the
// source context-trailed enode operations. A mark identifies the stack length
// to restore when the enclosing context pops a scope.
type SMTEnodeTrail struct {
	entries []smtEnodeTrailEntry
}

func (t *SMTEnodeTrail) Mark() int {
	if t == nil {
		return 0
	}
	return len(t.entries)
}

func (t *SMTEnodeTrail) Len() int {
	if t == nil {
		return 0
	}
	return len(t.entries)
}

func (t *SMTEnodeTrail) Undo(mark int) error {
	if t == nil || mark < 0 || mark > len(t.entries) {
		return ast.ErrASTDomain
	}
	for i := len(t.entries) - 1; i >= mark; i-- {
		entry := t.entries[i]
		switch entry.kind {
		case smtEnodeTrailGeneration:
			entry.node.generation = entry.generation
		case smtEnodeTrailLabelHash:
			entry.node.labelHash = entry.labelHash
		case smtEnodeTrailRootLabels:
			entry.node.labels = entry.rootLabels
		default:
			return ast.ErrASTDomain
		}
	}
	t.entries = t.entries[:mark]
	return nil
}

func (n *SMTEnode) SetGeneration(trail *SMTEnodeTrail, generation uint32) error {
	if err := n.valid(); err != nil {
		return err
	}
	if trail == nil {
		return ast.ErrASTDomain
	}
	if n.generation == generation {
		return nil
	}
	trail.entries = append(trail.entries, smtEnodeTrailEntry{
		kind:       smtEnodeTrailGeneration,
		node:       n,
		generation: n.generation,
	})
	n.generation = generation
	return nil
}

func (n *SMTEnode) SetLabelHash(trail *SMTEnodeTrail) error {
	if err := n.valid(); err != nil {
		return err
	}
	if trail == nil || n.labelHash != -1 || n.root == nil {
		return ast.ErrASTDomain
	}
	ownerID, err := n.OwnerID()
	if err != nil {
		return err
	}
	trail.entries = append(trail.entries, smtEnodeTrailEntry{
		kind:      smtEnodeTrailLabelHash,
		node:      n,
		labelHash: n.labelHash,
	})
	n.labelHash = int8(uhash.Uint(ownerID) & 63)
	if !n.root.labels.MayContain(uint32(n.labelHash)) {
		trail.entries = append(trail.entries, smtEnodeTrailEntry{
			kind:       smtEnodeTrailRootLabels,
			node:       n.root,
			rootLabels: n.root.labels,
		})
		n.root.labels.Insert(uint32(n.labelHash))
	}
	return nil
}

func (n *SMTEnode) HasLabelHash() bool {
	return n != nil && n.labelHash >= 0
}

func (n *SMTEnode) LabelHash() (uint8, error) {
	if err := n.valid(); err != nil {
		return 0, err
	}
	if n.labelHash < 0 || n.labelHash >= 64 {
		return 0, ast.ErrASTDomain
	}
	return uint8(n.labelHash), nil
}

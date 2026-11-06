package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

// QENNFNormalizer performs the QE-specific relevant-subformula NNF pass,
// plugin literal normalization, and polarity-set collection.
type QENNFNormalizer struct {
	self      *QENNFNormalizer
	manager   *ast.ASTManager
	relevance QENNFRelevance
	atoms     QENNFAtomNormalizer
	replacer  *astrewriter.ExprSafeReplace
	theory    *astrewriter.TheoryRewriter
	memo      map[qeNNFKey]ast.ASTHandle
	scratch   []ast.ASTRef
	closed    bool
}

func NewQENNFNormalizer(manager *ast.ASTManager, relevance QENNFRelevance, atoms QENNFAtomNormalizer, config Config) (result *QENNFNormalizer, resultErr error) {
	if manager == nil || relevance == nil || atoms == nil || config.Context == nil || config.Globals == nil {
		return nil, ast.ErrASTHandle
	}
	replacer, err := astrewriter.NewExprSafeReplace(manager)
	if err != nil {
		return nil, err
	}
	n := &QENNFNormalizer{manager: manager, relevance: relevance, atoms: atoms, replacer: replacer}
	n.self = n
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, n.Close())
			result = nil
		}
	}()
	n.theory, err = astrewriter.NewTheoryRewriter(manager, config.Context, config.Globals, nil)
	if err != nil {
		return nil, err
	}
	return n, nil
}

func (n *QENNFNormalizer) open() error {
	if n == nil || n.self != n || n.manager == nil || n.replacer == nil || n.theory == nil {
		return ast.ErrASTHandle
	}
	if n.closed {
		return arena.ErrClosed
	}
	return nil
}

func (n *QENNFNormalizer) Normalize(formula ast.ASTRef) (index *BoolAtomIndex, resultErr error) {
	if err := n.open(); err != nil {
		return nil, err
	}
	if err := n.resetCall(); err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, n.resetCall()) }()
	root, err := n.manager.RefGet(formula)
	if err != nil {
		return nil, err
	}
	nnf, err := n.toNNF(root, true)
	if err != nil {
		return nil, err
	}
	normalized, err := n.normalizeLiterals(nnf)
	if err != nil {
		return nil, err
	}
	positive, negative, err := n.collectAtoms(normalized)
	if err != nil {
		return nil, err
	}
	index, err = NewBoolAtomIndex(n.manager, positive, negative)
	if err != nil {
		return nil, err
	}
	if err = n.manager.SetRef(formula, normalized); err != nil {
		return nil, errors.Join(err, index.Close())
	}
	return index, nil
}

func (n *QENNFNormalizer) Close() (result error) {
	if n == nil || n.self != n {
		return ast.ErrASTHandle
	}
	if n.closed {
		return nil
	}
	var theoryErr error
	if n.theory != nil {
		theoryErr = n.theory.Close()
	}
	result = errors.Join(n.resetCall(), theoryErr, n.replacer.Close())
	if result == nil {
		n.closed = true
	}
	return result
}

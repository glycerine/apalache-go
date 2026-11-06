package base

// Source: src/muz/base/hnf.cpp imp::set_name/reset/get_fresh_predicates;
// src/muz/base/dl_rule.cpp rule_manager::mk_rule_core.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type hornCleanup uint8

const (
	hornCleanupNone hornCleanup = iota
	hornCleanupReset
	hornCleanupBody
	hornCleanupClose
)

func (n *HornNormalizer) open() error {
	if n == nil || n.manager == nil || n.closed || n.cleanupPending != hornCleanupNone {
		return ErrHornNormalizationOwner
	}
	return nil
}

// SetName selects the prefix for future definitions. A null symbol restores P.
func (n *HornNormalizer) SetName(name usymbol.Symbol) error {
	if err := n.open(); err != nil {
		return err
	}
	if name.IsNull() {
		var err error
		name, err = n.manager.Symbols.Intern("P")
		if err != nil {
			return err
		}
	}
	n.prefix = name
	return nil
}

// Reset clears source memo/cache roots without changing the name, hoister or
// current body. The next processing step clears that body. Previously returned
// results and fresh-predicate snapshots own their references.
func (n *HornNormalizer) Reset() error {
	if n == nil || n.manager == nil || n.closed || n.cleanupPending == hornCleanupClose {
		return ErrHornNormalizationOwner
	}
	// Finish an interrupted previous step before beginning this reset. A
	// normal reset still preserves the current body, as in the source.
	if n.cleanupPending == hornCleanupBody {
		if err := n.drainHornRoots(&n.bodyRefs); err != nil {
			return err
		}
	}
	n.cleanupPending = hornCleanupReset
	if err := n.resetHornRoots(); err != nil {
		return err
	}
	n.cleanupPending = hornCleanupNone
	return nil
}

// DelRef may consume an owner while returning a later destruction error.
// Preserve each unfinished owner and the untouched prefix for a cleanup retry.
func (n *HornNormalizer) drainHornRoots(refs *[]ast.ASTRef) error {
	for len(*refs) > 0 {
		last := len(*refs) - 1
		err := n.manager.DelRef(&(*refs)[last])
		if (*refs)[last] == (ast.ASTRef{}) {
			*refs = (*refs)[:last]
		}
		if err != nil {
			return err
		}
	}
	*refs = nil
	return nil
}

func (n *HornNormalizer) resetHornRoots() error {
	for _, refs := range []*[]ast.ASTRef{&n.workRefs, &n.freshPredicates, &n.todoRefs} {
		if err := n.drainHornRoots(refs); err != nil {
			return err
		}
	}
	return nil
}

// FreshPredicates copies the source declaration vector into owning references.
// The caller releases every reference; an already-Horn invocation leaves this
// vector unchanged until Reset or a subsequent non-Horn invocation.
func (n *HornNormalizer) FreshPredicates() (result []ast.ASTRef, resultErr error) {
	if err := n.open(); err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, closeASTRefs(n.manager, result))
			result = nil
		}
	}()
	for _, ref := range n.freshPredicates {
		declaration, err := n.manager.RefGet(ref)
		if err != nil {
			return result, err
		}
		copyRef, err := n.manager.NewRef(declaration)
		if err != nil {
			return result, err
		}
		result = append(result, copyRef)
	}
	return result, nil
}

func (n *HornNormalizer) Close() error {
	if n == nil || n.manager == nil {
		return ErrHornNormalizationOwner
	}
	if n.closed {
		return nil
	}
	if _, err := n.manager.ProofMode(); err != nil {
		return err
	}
	n.cleanupPending = hornCleanupClose
	if err := n.resetHornRoots(); err != nil {
		return err
	}
	if err := n.drainHornRoots(&n.bodyRefs); err != nil {
		return err
	}
	if err := n.hoister.Close(); err != nil {
		return err
	}
	n.cleanupPending = hornCleanupNone
	n.closed = true
	return nil
}

func (m *RuleManager) prepareHornNormalizer(name usymbol.Symbol) (*HornNormalizer, error) {
	if err := m.normalizer.Reset(); err != nil {
		return nil, err
	}
	if err := m.normalizer.SetName(name); err != nil {
		return nil, err
	}
	return m.normalizer, nil
}

func (m *RuleManager) registerHornPredicates() (resultErr error) {
	declarations, err := m.normalizer.FreshPredicates()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, closeASTRefs(m.manager, declarations)) }()
	for _, ref := range declarations {
		declaration, err := m.manager.RefGet(ref)
		if err != nil {
			return err
		}
		if err = m.RegisterPredicate(declaration); err != nil {
			return err
		}
	}
	return nil
}

// Source m_refs owns memoized expressions/proofs, while m_body survives reset
// and is cleared at the start of the next mk_horn call or during destruction.
func (n *HornNormalizer) rememberHornRoot(h ast.ASTHandle) error {
	return n.pin(&n.workRefs, h)
}

func (n *HornNormalizer) beginHornStep() error {
	if n == nil || n.manager == nil || n.closed || (n.cleanupPending != hornCleanupNone && n.cleanupPending != hornCleanupBody) {
		return ErrHornNormalizationOwner
	}
	n.cleanupPending = hornCleanupBody
	if err := n.drainHornRoots(&n.bodyRefs); err != nil {
		return err
	}
	n.cleanupPending = hornCleanupNone
	return nil
}

func (n *HornNormalizer) storeHornBody(body []ast.ASTHandle) error {
	for _, h := range body {
		if err := n.pin(&n.bodyRefs, h); err != nil {
			return err
		}
	}
	return nil
}

func (n *HornNormalizer) replaceHornBody(index int, body ast.ASTHandle) error {
	return n.manager.SetRef(n.bodyRefs[index], body)
}

package pdr

// Source: pred_transformer::inherit_properties in src/muz/pdr/pdr_context.cpp.

import "github.com/glycerine/g3/pkg/ast"

// FrameProperty is one borrowed retained lemma and its strongest assigned
// frame. InfinityLevel identifies the invariant frame.
type FrameProperty struct {
	Formula ast.ASTHandle
	Level   uint32
}

// Properties snapshots the effective property-to-level map. Stale entries in
// weaker frame vectors are deliberately omitted.
func (p *PredicateTransformer) Properties() ([]FrameProperty, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	result := make([]FrameProperty, 0, len(p.propertyLevel))
	seen := make(map[ast.ASTHandle]bool, len(p.propertyLevel))
	appendAt := func(formulas []ast.ASTHandle, level uint32) {
		for _, formula := range formulas {
			if seen[formula] || p.propertyLevel[formula] != level {
				continue
			}
			seen[formula] = true
			result = append(result, FrameProperty{Formula: formula, Level: level})
		}
	}
	appendAt(p.invariants, InfinityLevel)
	for level, formulas := range p.levels {
		appendAt(formulas, uint32(level))
	}
	return result, nil
}

// InheritProperties installs the current effective frame lemmas from another
// transformer for the same predicate.
func (p *PredicateTransformer) InheritProperties(other *PredicateTransformer) error {
	if err := p.open(); err != nil {
		return err
	}
	if other == nil {
		return ErrPredicateTransformerOwner
	}
	if err := other.open(); err != nil {
		return err
	}
	if p.head != other.head || p.manager != other.manager || p.owner != other.owner {
		return ast.ErrASTDomain
	}
	properties, err := other.Properties()
	if err != nil {
		return err
	}
	for _, property := range properties {
		if _, err = p.AddProperty(property.Formula, property.Level); err != nil {
			return err
		}
	}
	return nil
}

package rel

// Source: dl_relation_manager.cpp typed storage for the explanation product
// registered by dl_mk_explanations.cpp, and dl_instruction.cpp load/store.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (m *RelationManager) explanationPredicateSignature(predicate ast.ASTHandle) (finite []ast.ASTHandle, explanations int, resultErr error) {
	if err := m.open(); err != nil {
		return nil, 0, err
	}
	kind, err := m.manager.GetKind(predicate)
	if err != nil {
		return nil, 0, err
	}
	if kind != ast.ASTFuncDecl {
		return nil, 0, ErrTableRelation
	}
	arity, err := m.manager.GetArity(predicate)
	if err != nil {
		return nil, 0, err
	}
	for i := uint32(0); i < arity; i++ {
		sort, err := m.manager.GetDomain(predicate, i)
		if err != nil {
			return nil, 0, err
		}
		rule, err := m.manager.IsDatalogRuleSort(sort)
		if err != nil {
			return nil, 0, err
		}
		if rule {
			explanations++
			continue
		}
		if explanations != 0 {
			return nil, 0, ErrTableRelation
		}
		if m.explanationNumericKind != "" {
			accepted, numericErr := relationExplanationNumericCanHandle(m, m.explanationNumericKind, []ast.ASTHandle{sort})
			if numericErr != nil {
				return nil, 0, numericErr
			}
			if !accepted {
				if _, err = m.sortDomain(sort); err != nil {
					return nil, 0, err
				}
			}
		} else if _, err = m.sortDomain(sort); err != nil {
			return nil, 0, err
		}
		finite = append(finite, sort)
	}
	if explanations == 0 {
		return nil, 0, ErrTableRelation
	}
	return finite, explanations, nil
}

// TryGetExplanationRelation returns the borrowed concrete explanation product.
func (m *RelationManager) TryGetExplanationRelation(predicate ast.ASTHandle) (*FiniteExplanationProduct, bool, error) {
	if err := m.open(); err != nil {
		return nil, false, err
	}
	p, found := m.explanations[predicate]
	if p != nil {
		if err := p.open(); err != nil {
			return nil, false, err
		}
	}
	return p, found, nil
}

func (m *RelationManager) GetExplanationRelation(predicate ast.ASTHandle) (*FiniteExplanationProduct, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if p := m.explanations[predicate]; p != nil {
		if err := p.open(); err != nil {
			return nil, err
		}
		return p, nil
	}
	finite, count, err := m.explanationPredicateSignature(predicate)
	if err != nil {
		return nil, err
	}
	p, err := newFiniteExplanationProduct(m, finite, count)
	if err != nil {
		return nil, err
	}
	if err = m.StoreExplanationRelation(predicate, p); err != nil {
		return nil, errors.Join(err, p.Close())
	}
	return p, nil
}

// StoreExplanationRelation transfers the product on success. Its declaration
// pins the complete signature until the manager releases the relation.
func (m *RelationManager) StoreExplanationRelation(predicate ast.ASTHandle, p *FiniteExplanationProduct) error {
	if err := m.open(); err != nil {
		return err
	}
	if err := p.open(); err != nil {
		return err
	}
	finite, count, err := m.explanationPredicateSignature(predicate)
	if err != nil {
		return err
	}
	if p.manager != m || count != p.explanationColumns || len(finite) != len(p.finiteSorts) {
		return ErrTableRelation
	}
	for i, sort := range finite {
		if sort != p.finiteSorts[i] {
			return ErrTableRelation
		}
	}
	if m.relations[predicate] != nil || m.finite[predicate] != nil || m.relationLevel[predicate] != nil {
		return ErrTableRelation
	}
	prior, found := m.explanations[predicate]
	if prior == p {
		return nil
	}
	if found {
		if err = prior.Close(); err != nil {
			return err
		}
	} else if err = m.manager.IncRef(predicate); err != nil {
		return err
	}
	m.explanations[predicate] = p
	return nil
}

func (p *FiniteExplanationProduct) Clone() (result *FiniteExplanationProduct, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	result, resultErr = p.newProduct(p.finiteSorts, p.explanationColumns)
	if resultErr != nil {
		return nil, resultErr
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	rows, err := p.table.Facts()
	if err != nil {
		return result, err
	}
	for _, key := range rows {
		index := uint64(key[len(key)-1])
		if index >= uint64(len(p.inner)) || p.inner[index] == nil {
			return result, ErrTableRelation
		}
		empty, err := p.inner[index].Empty()
		if err != nil {
			return result, err
		}
		if empty {
			continue
		}
		row, err := p.inner[index].Data()
		if err != nil {
			return result, err
		}
		if err = result.addEncodedRow(append(Fact(nil), key...), row); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (m *RelationManager) predicateUsesExplanationColumns(predicate ast.ASTHandle) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	kind, err := m.manager.GetKind(predicate)
	if err != nil {
		return false, err
	}
	if kind != ast.ASTFuncDecl {
		return false, ErrTableRelation
	}
	arity, err := m.manager.GetArity(predicate)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < arity; i++ {
		sort, err := m.manager.GetDomain(predicate, i)
		if err != nil {
			return false, err
		}
		rule, err := m.manager.IsDatalogRuleSort(sort)
		if err != nil {
			return false, err
		}
		if rule {
			return true, nil
		}
	}
	return false, nil
}

package rel

import "errors"

func (i *relationIOInstruction) performExplanation(context *ExecutionContext, relations *RelationManager) (bool, error) {
	if !i.store {
		product, err := relations.GetExplanationRelation(i.predicate)
		if err != nil {
			return false, err
		}
		count, err := product.Len()
		if err != nil {
			return false, err
		}
		if count == 0 {
			return true, context.MakeEmpty(i.register)
		}
		copied, err := product.Clone()
		if err != nil {
			return false, err
		}
		if err = context.SetExplanationRegister(i.register, copied); err != nil {
			return false, errors.Join(err, copied.Close())
		}
		return true, nil
	}
	product, err := context.ExplanationRegister(i.register)
	if err != nil {
		return false, err
	}
	transferred := product != nil
	if transferred {
		product, err = context.ReleaseExplanationRegister(i.register)
		if err != nil {
			return false, err
		}
	}
	if product == nil {
		sorts, count, err := relations.explanationPredicateSignature(i.predicate)
		if err != nil {
			return false, err
		}
		product, err = newFiniteExplanationProduct(relations, sorts, count)
		if err != nil {
			return false, err
		}
	}
	if err = relations.StoreExplanationRelation(i.predicate, product); err != nil {
		if transferred {
			err = errors.Join(err, context.SetExplanationRegister(i.register, product))
		} else {
			err = errors.Join(err, product.Close())
		}
		return false, err
	}
	return true, nil
}

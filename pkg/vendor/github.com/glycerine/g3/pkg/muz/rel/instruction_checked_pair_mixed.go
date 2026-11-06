package rel

import "errors"

func checkedPairMixedJoin(first, second FiniteTable, left, right []int) (result FiniteTable, resultErr error) {
	closeLifted := func(lifted *FiniteCheckedPair) {
		closeErr := lifted.Close()
		if closeErr != nil && result != nil {
			closeErr = errors.Join(closeErr, result.Close())
			result = nil
		}
		resultErr = errors.Join(resultErr, closeErr)
	}
	if pair, ok := first.(*FiniteCheckedPair); ok {
		lifted, err := pair.Lift(second)
		if err != nil {
			return nil, err
		}
		defer func() { closeLifted(lifted) }()
		result, resultErr = pair.Join(lifted, left, right)
		return result, resultErr
	}
	pair, ok := second.(*FiniteCheckedPair)
	if !ok {
		return nil, ErrCheckTable
	}
	lifted, err := pair.Lift(first)
	if err != nil {
		return nil, err
	}
	defer func() { closeLifted(lifted) }()
	result, resultErr = lifted.Join(pair, left, right)
	return result, resultErr
}

func checkedPairMixedJoinProject(first, second FiniteTable, left, right, removed []int) (result FiniteTable, resultErr error) {
	closeLifted := func(lifted *FiniteCheckedPair) {
		closeErr := lifted.Close()
		if closeErr != nil && result != nil {
			closeErr = errors.Join(closeErr, result.Close())
			result = nil
		}
		resultErr = errors.Join(resultErr, closeErr)
	}
	if pair, ok := first.(*FiniteCheckedPair); ok {
		lifted, err := pair.Lift(second)
		if err != nil {
			return nil, err
		}
		defer func() { closeLifted(lifted) }()
		result, resultErr = pair.JoinProject(lifted, left, right, removed)
		return result, resultErr
	}
	pair, ok := second.(*FiniteCheckedPair)
	if !ok {
		return nil, ErrCheckTable
	}
	lifted, err := pair.Lift(first)
	if err != nil {
		return nil, err
	}
	defer func() { closeLifted(lifted) }()
	result, resultErr = lifted.JoinProject(pair, left, right, removed)
	return result, resultErr
}

func checkedPairMixedUnion(target *FiniteCheckedPair, source FiniteTable, delta *FiniteCheckedPair) (resultErr error) {
	lifted, err := target.Lift(source)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, lifted.Close()) }()
	return target.UnionFrom(lifted, delta)
}

func checkedPairMixedNegation(target *FiniteCheckedPair, negative FiniteTable, targetColumns, negativeColumns []int) (resultErr error) {
	lifted, err := target.Lift(negative)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, lifted.Close()) }()
	return target.FilterByNegation(lifted, targetColumns, negativeColumns)
}

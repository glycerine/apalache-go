package realclosure

// Source: src/math/realclosure/realclosure.cpp:1302-1330 and 4149-4170.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
)

func (s *realValueStore) refineTranscendentalRoot(root *algebraicRoot, precision uint32) (err error) {
	if s == nil || root == nil || root.kind != realExtensionTranscendental || !root.isRegistered() || root.interval == nil || root.owner.api == nil || root.transcendentalStep == ^uint32(0) {
		return arena.ErrInvalidSlot
	}
	api := root.owner.api
	if api.transcendentals == nil || api.initialPrecision == 0 || (root.name != "pi" && root.name != "e") {
		return arena.ErrInvalidSlot
	}
	if err := s.saveExtensionIntervalIfTooSmall(root, precision); err != nil {
		return err
	}
	if err := s.checkpoint(); err != nil {
		return err
	}
	root.transcendentalStep++
	rational, err := interval.NewRationalInterval(s.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, rational.Close()) }()
	if root.name == "pi" {
		err = api.transcendentals.Pi(root.transcendentalStep, rational)
	} else {
		err = api.transcendentals.E(root.transcendentalStep, rational)
	}
	if err != nil {
		return err
	}
	updated, err := api.rationalIntervalToBinary(rational, api.initialPrecision)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, updated.Close()) }()
	return root.interval.SetFrom(updated)
}

func (s *realValueStore) refineTranscendentalPolynomialValue(value *realValue, precision uint32) (bool, error) {
	if s == nil || value == nil || value.isRational || value.root == nil || !value.root.isRegistered() || value.root.kind != realExtensionTranscendental || value.functionInterval == nil {
		return false, arena.ErrInvalidSlot
	}
	if precision == 0 {
		precision = 1
	}
	numeratorInterval, err := NewBinaryInterval(value.functionInterval.numbers)
	if err != nil {
		return false, err
	}
	defer numeratorInterval.Close()
	var denominatorInterval *BinaryInterval
	if len(value.functionDenominator.coefficients) != 0 && !s.rankedDenominatorIsOne(value.functionDenominator.coefficients) {
		denominatorInterval, err = NewBinaryInterval(value.functionInterval.numbers)
		if err != nil {
			return false, err
		}
		defer denominatorInterval.Close()
	}
	for workPrecision := precision; ; workPrecision++ {
		if err = s.checkpoint(); err != nil {
			return false, err
		}
		for _, polynomial := range [][]*realValue{value.functionNumerator.coefficients, value.functionDenominator.coefficients} {
			for _, coefficient := range polynomial {
				if coefficient == nil {
					continue
				}
				refined, err := s.refineAnyRetainedInterval(coefficient, value.functionInterval.numbers, workPrecision)
				if err != nil {
					return false, err
				}
				if !refined {
					return false, arena.ErrBounds
				}
			}
		}
		if err := s.refineTranscendentalRoot(value.root, workPrecision); err != nil {
			return false, err
		}
		if err := s.evaluateRankedPolynomialInterval(value.functionNumerator.coefficients, value.root.interval, numeratorInterval); err != nil {
			return false, err
		}
		if denominatorInterval != nil {
			if err := s.evaluateRankedPolynomialInterval(value.functionDenominator.coefficients, value.root.interval, denominatorInterval); err != nil {
				return false, err
			}
			if err := value.functionInterval.Div(numeratorInterval, denominatorInterval, workPrecision); err != nil {
				return false, err
			}
		} else if err := value.functionInterval.SetFrom(numeratorInterval); err != nil {
			return false, err
		}
		precise, err := binaryIntervalPrecise(value.functionInterval, precision)
		if err != nil || precise {
			return precise, err
		}
		if workPrecision == ^uint32(0) {
			return false, arena.ErrBounds
		}
	}
}

func (s *realValueStore) evaluateRankedPolynomialInterval(coefficients []*realValue, argument, result *BinaryInterval) error {
	if s == nil || len(coefficients) == 0 || coefficients[len(coefficients)-1] == nil || argument == nil || result == nil || argument.numbers != result.numbers {
		return arena.ErrInvalidSlot
	}
	coefficientInterval := func(value *realValue) (*BinaryInterval, error) {
		if value == nil {
			return nil, arena.ErrInvalidSlot
		}
		return s.valueInterval(value, argument.numbers)
	}
	leading, err := coefficientInterval(coefficients[len(coefficients)-1])
	if err != nil {
		return err
	}
	if len(coefficients) == 1 {
		return result.SetFrom(leading)
	}
	if err = result.Mul(leading, argument); err != nil {
		return err
	}
	for i := len(coefficients) - 2; i >= 0; i-- {
		if coefficients[i] != nil {
			coefficient, e := coefficientInterval(coefficients[i])
			if e != nil {
				return e
			}
			if e = result.Add(result, coefficient); e != nil {
				return e
			}
		}
		if i > 0 {
			if err = result.Mul(result, argument); err != nil {
				return err
			}
		}
	}
	return nil
}

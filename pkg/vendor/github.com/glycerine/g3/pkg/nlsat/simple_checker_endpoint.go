package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:29-157.

import "github.com/glycerine/g3/pkg/math/polynomial"

type simpleEndpoint struct {
	infinite, open, lower bool
	value                 polynomial.Algebraic
}

func (e *simpleEndpoint) close(am *polynomial.AlgebraicManager) error { return am.Del(&e.value) }
func (e *simpleEndpoint) set(am *polynomial.AlgebraicManager, v *polynomial.Algebraic, open bool) error {
	e.infinite = false
	e.open = open
	return am.Copy(&e.value, v)
}
func (e *simpleEndpoint) compare(am *polynomial.AlgebraicManager, other *simpleEndpoint) (int, error) {
	if e.infinite {
		if other.infinite {
			if e.lower == other.lower {
				return 0, nil
			}
			if e.lower {
				return -1, nil
			}
			return 1, nil
		}
		if e.lower {
			return -1, nil
		}
		return 1, nil
	}
	if other.infinite {
		if other.lower {
			return 1, nil
		}
		return -1, nil
	}
	order, err := am.Compare(&e.value, &other.value)
	if err != nil || order != 0 {
		return order, err
	}
	if e.open == other.open {
		if !e.open || e.lower == other.lower {
			return 0, nil
		}
		if !e.lower {
			return -1, nil
		}
		return 1, nil
	}
	if !e.open {
		if other.lower {
			return -1, nil
		}
		return 1, nil
	}
	if e.lower {
		return 1, nil
	}
	return -1, nil
}

package realclosure

// Source: src/math/realclosure/realclosure.cpp:900-907,2515-2545.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
)

// bindIntervalPrecisions follows the selected generated rcf_params getters:
// local value, module value, then the literal default. The module is borrowed
// from the enclosing parameter owner, which owns its acquisition and release.
func (s *realValueStore) bindIntervalPrecisions(pc *params.ParamsContext, p, module *params.ParamsRef) error {
	if s == nil || s.numbers == nil || s.pendingPrecisionHelper != nil || pc == nil || p == nil {
		return arena.ErrInvalidSlot
	}
	if err := pc.Check(p); err != nil {
		return err
	}
	name := "initial_precision"
	value, err := pc.GetUIntFallbackName(p, &name, module, 24)
	if err != nil {
		return err
	}
	s.initialPrecision = value
	s.initialPrecisionBound = true
	name = "max_precision"
	value, err = pc.GetUIntFallbackName(p, &name, module, 128)
	if err != nil {
		return err
	}
	s.maxPrecision = value
	s.maxPrecisionBound = true
	return nil
}

// bindIntervalPrecisionsFromGlobals executes the generated rcf_params helper's
// get_module acquisition, getter and local reference destruction. A stopped
// destruction remains with this store for explicit owning completion.
func (s *realValueStore) bindIntervalPrecisionsFromGlobals(pc *params.ParamsContext, p *params.ParamsRef, globals *params.GlobalParamsService) (err error) {
	if s == nil || s.numbers == nil || s.pendingPrecisionHelper != nil || pc == nil || p == nil || globals == nil {
		return arena.ErrInvalidSlot
	}
	owner := &params.ModuleParamsOwner{Budget: globals.Config.Budget}
	owner.Self = owner
	if err = owner.Bind(pc, globals); err != nil {
		return err
	}
	owner.Running = true
	var frame *params.ModuleParamsFrame
	defer func() {
		if frame != nil {
			err = owner.FinishHelper(frame, err)
			if owner.Helpers != nil {
				s.pendingPrecisionHelper = owner
			}
		}
		owner.Running = false
	}()
	frame, err = owner.BeginHelper(p, "rcf")
	if err != nil {
		return err
	}
	value, err := owner.FrameUInt(frame, "initial_precision", 24)
	if err != nil {
		return err
	}
	s.initialPrecision = value
	s.initialPrecisionBound = true
	value, err = owner.FrameUInt(frame, "max_precision", 128)
	if err != nil {
		return err
	}
	s.maxPrecision = value
	s.maxPrecisionBound = true
	return nil
}

func (s *realValueStore) cleanupInitialPrecisionHelper() error {
	if s == nil {
		return arena.ErrInvalidSlot
	}
	if s.pendingPrecisionHelper == nil {
		return nil
	}
	if err := s.pendingPrecisionHelper.CloseHelpers(); err != nil {
		return err
	}
	s.pendingPrecisionHelper = nil
	return nil
}

func (s *realValueStore) rationalInitialPrecision() uint32 {
	if s.initialPrecisionBound {
		return s.initialPrecision
	}
	return 24
}

func (s *realValueStore) intervalMaxPrecision() uint32 {
	if s.maxPrecisionBound {
		return s.maxPrecision
	}
	return 128
}

// rationalApproxInterval initializes one nonzero rational coefficient using
// the source initial precision. The returned interval is owned by the
// caller; cachedRationalApproxInterval transfers that ownership to the value.
func (s *realValueStore) rationalApproxInterval(value *realValue, bounds *numeral.BinaryRationalManager) (interval *BinaryInterval, err error) {
	if s == nil || s.numbers == nil || bounds == nil || !s.rationalOperand(value) || value == nil {
		return nil, arena.ErrInvalidSlot
	}
	interval, err = NewBinaryInterval(bounds)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, s.releaseSturmInterval(interval))
			interval = nil
		}
	}()
	lower, upper, width := numeral.NewBinaryRational(), numeral.NewBinaryRational(), numeral.NewBinaryRational()
	defer func() {
		err = errors.Join(err,
			s.releaseSturmBinary(bounds, &width),
			s.releaseSturmBinary(bounds, &upper),
			s.releaseSturmBinary(bounds, &lower))
	}()
	exact, err := bounds.FromRational(s.numbers, &value.rational, &lower)
	if err != nil {
		return nil, err
	}
	if exact {
		err = interval.SetFinite(&lower, &lower, false, false)
		return interval, err
	}
	if err = bounds.Copy(&upper, &lower); err != nil {
		return nil, err
	}
	if err = bounds.Mul2(&upper); err != nil {
		return nil, err
	}
	negative, err := s.numbers.IsNeg(&value.rational)
	if err != nil {
		return nil, err
	}
	if negative {
		if err = bounds.Swap(&lower, &upper); err != nil {
			return nil, err
		}
	}
	for {
		if err = interval.SetFinite(&lower, &upper, true, true); err != nil {
			return nil, err
		}
		contains, e := interval.ContainsZero()
		if e != nil {
			return nil, e
		}
		refine := contains
		if !refine {
			if err = bounds.Sub(&upper, &lower, &width); err != nil {
				return nil, err
			}
			precise, e := bounds.LessOneDiv2k(&width, s.rationalInitialPrecision())
			if e != nil {
				return nil, e
			}
			refine = !precise
		}
		if !refine {
			refine, err = bounds.IsZero(&lower)
			if err != nil {
				return nil, err
			}
		}
		if !refine {
			refine, err = bounds.IsZero(&upper)
			if err != nil {
				return nil, err
			}
		}
		if !refine {
			return interval, nil
		}
		// Source checkpoint before each paired refinement still needs its
		// enclosing real-closure resource/cooperation owner.
		if err = bounds.RefineLower(s.numbers, &value.rational, &lower, &upper); err != nil {
			return nil, err
		}
		if err = bounds.RefineUpper(s.numbers, &value.rational, &lower, &upper); err != nil {
			return nil, err
		}
	}
}

// Source rational_value::m_interval is initialized lazily and survives
// subsequent sign checks so coefficient refinement is not discarded.
func (s *realValueStore) cachedRationalApproxInterval(value *realValue, bounds *numeral.BinaryRationalManager) (*BinaryInterval, error) {
	if s == nil || value == nil || value.store != s || value.deleted || value.refs == 0 || !value.isRational || bounds == nil {
		return nil, arena.ErrInvalidSlot
	}
	if value.rationalInterval != nil {
		if value.rationalInterval.numbers != bounds {
			return nil, arena.ErrInvalidSlot
		}
		return value.rationalInterval, nil
	}
	interval, err := s.rationalApproxInterval(value, bounds)
	if err != nil {
		return nil, err
	}
	value.rationalInterval = interval
	return interval, nil
}

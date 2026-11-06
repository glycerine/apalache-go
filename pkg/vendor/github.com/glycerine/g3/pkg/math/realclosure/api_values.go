package realclosure

// Source: src/math/realclosure/realclosure.cpp, manager numeral ownership and
// isolate_roots. This concrete owner supplies the API's real-closure values.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
)

// APIValues owns the root registry and binary interval manager for one API
// context. Its rational service is borrowed and must outlive this owner.
type APIValues struct {
	store             realValueStore
	bounds            *numeral.BinaryRationalManager
	transcendentals   *interval.RationalTranscendentals
	initialPrecision  uint32
	infPrecision      uint32
	usePrem           bool
	cleanDenominators bool
	lazyNormalization bool
	roots             algebraicRootOwner
	pi, e             *realValue
	live              int
}

// Rational creates a value from the API's rational string syntax.
func (v *APIValues) Rational(text string) (*APIValue, error) {
	if v == nil || v.bounds == nil {
		return nil, arena.ErrInvalidSlot
	}
	raw := numeral.NewRational()
	if err := v.store.numbers.SetString(&raw, text); err != nil {
		return nil, errors.Join(err, v.store.numbers.Del(&raw))
	}
	value, err := v.store.newRationalCopy(&raw)
	cleanup := v.store.numbers.Del(&raw)
	if err != nil {
		return nil, errors.Join(err, cleanup)
	}
	if cleanup != nil {
		return nil, errors.Join(cleanup, v.store.releaseSturmScratch(value))
	}
	return v.wrap(value), nil
}

// IsolateRoots takes coefficients in ascending degree order and returns the
// source's root order. Each result owns one reference, including zero.
func (v *APIValues) IsolateRoots(coefficients []*APIValue) (result []*APIValue, err error) {
	if v == nil || v.bounds == nil || len(coefficients) == 0 {
		return nil, arena.ErrInvalidSlot
	}
	defer v.store.intervalRestoreDefer(&err)()
	p := make([]*realValue, len(coefficients))
	for i, coefficient := range coefficients {
		if coefficient == nil || coefficient.owner != v {
			return nil, arena.ErrInvalidSlot
		}
		p[i] = coefficient.value
	}
	for len(p) > 0 && p[len(p)-1] == nil {
		p = p[:len(p)-1]
	}
	if len(p) == 0 {
		return nil, arena.ErrInvalidSlot
	}
	roots, err := v.roots.isolateAnyRoots(p, v.usePrem)
	if err != nil {
		return nil, errors.Join(err, v.store.releaseSturmCoefficients(roots))
	}
	result = make([]*APIValue, len(roots))
	for i, root := range roots {
		result[i] = v.wrap(root)
	}
	return result, nil
}

// Pi returns the context's cached transcendental pi value.
func (v *APIValues) Pi() (*APIValue, error) { return v.transcendentalConstant(true) }

// E returns the context's cached transcendental Euler constant.
func (v *APIValues) E() (*APIValue, error) { return v.transcendentalConstant(false) }

// Infinitesimal creates a fresh positive infinitesimal extension.
func (v *APIValues) Infinitesimal() (*APIValue, error) {
	if v == nil || v.bounds == nil {
		return nil, arena.ErrInvalidSlot
	}
	for len(v.roots.infinitesimals) > 0 && v.roots.infinitesimals[len(v.roots.infinitesimals)-1] == nil {
		v.roots.infinitesimals = v.roots.infinitesimals[:len(v.roots.infinitesimals)-1]
	}
	index := len(v.roots.infinitesimals) + 1
	enclosure, err := v.infinitesimalInterval()
	if err != nil {
		return nil, err
	}
	name := "eps!" + strconv.Itoa(index)
	prettyName := "&epsilon;<sub>" + strconv.Itoa(index) + "</sub>"
	root, err := v.roots.mkInfinitesimal(name, prettyName, enclosure)
	if err != nil {
		return nil, errors.Join(err, enclosure.Close())
	}
	one, err := v.store.newRational(1, 1)
	if err != nil {
		return nil, errors.Join(err, root.discardUnpublished())
	}
	value, valueErr := v.store.newAlgebraicValue(root, one)
	cleanupErr := v.store.release(one)
	if valueErr != nil || cleanupErr != nil {
		if valueErr == nil {
			valueErr = v.store.release(value)
		} else {
			valueErr = errors.Join(valueErr, root.discardUnpublished())
		}
		if cleanupErr != nil {
			cleanupErr = errors.Join(cleanupErr, v.store.release(one))
		}
		return nil, errors.Join(valueErr, cleanupErr)
	}
	return v.wrap(value), nil
}

func (v *APIValues) transcendentalConstant(pi bool) (*APIValue, error) {
	if v == nil || v.bounds == nil || v.transcendentals == nil {
		return nil, arena.ErrInvalidSlot
	}
	cached := &v.e
	name, prettyName := "e", "e"
	if pi {
		cached, name, prettyName = &v.pi, "pi", "&pi;"
	}
	if *cached == nil {
		var enclosure *BinaryInterval
		var err error
		if pi {
			enclosure, err = v.piInterval()
		} else {
			enclosure, err = v.eInterval()
		}
		if err != nil {
			return nil, err
		}
		root, err := v.roots.mkTranscendental(name, prettyName, enclosure)
		if err != nil {
			return nil, errors.Join(err, enclosure.Close())
		}
		one, err := v.store.newRational(1, 1)
		if err != nil {
			return nil, errors.Join(err, root.discardUnpublished())
		}
		value, valueErr := v.store.newAlgebraicValue(root, one)
		cleanupErr := v.store.release(one)
		if valueErr != nil || cleanupErr != nil {
			if valueErr == nil {
				valueErr = v.store.release(value)
			} else {
				valueErr = errors.Join(valueErr, root.discardUnpublished())
			}
			if cleanupErr != nil {
				cleanupErr = errors.Join(cleanupErr, v.store.release(one))
			}
			return nil, errors.Join(valueErr, cleanupErr)
		}
		*cached = value
	}
	if err := v.store.retain(*cached); err != nil {
		return nil, err
	}
	return v.wrap(*cached), nil
}

// APIValue owns one real-closure numeral, including the nil representation of
// zero. Values must be closed before their APIValues owner.
type APIValue struct {
	owner *APIValues
	value *realValue
}

func NewAPIValues(service *numeral.RationalService) (*APIValues, error) {
	return NewAPIValuesWithExecution(service, nil, nil)
}

// NewAPIValuesWithExecution binds the source manager's resource and
// cooperation services to root refinement and polynomial arithmetic.
func NewAPIValuesWithExecution(service *numeral.RationalService, limit *control.Reslimit, cooperation *cooperate.Cooperation) (*APIValues, error) {
	if service == nil || service.Manager == nil {
		return nil, arena.ErrConfig
	}
	bounds, err := numeral.NewBinaryRationalManager(service.Manager.Z, numeral.BinaryRationalConfig{Diagnostics: service})
	if err != nil {
		return nil, err
	}
	v := &APIValues{
		bounds:            bounds,
		transcendentals:   &interval.RationalTranscendentals{Numbers: service.Manager},
		initialPrecision:  24,
		infPrecision:      24,
		usePrem:           true,
		cleanDenominators: true,
		lazyNormalization: true,
	}
	v.store.numbers = service.Manager
	v.store.intervals = bounds
	v.store.lazyNormalization = true
	v.store.lazyNormalizationBound = true
	v.store.usePrem, v.store.usePremBound = true, true
	v.store.limit = limit
	v.store.cooperation = cooperation
	v.roots.values, v.roots.intervals = &v.store, bounds
	v.roots.api = v
	return v, nil
}

// NewAPIValuesWithRCFParams binds construction-time rcf settings through the
// source local, module, and default lookup path.
func NewAPIValuesWithRCFParams(service *numeral.RationalService, limit *control.Reslimit, cooperation *cooperate.Cooperation, context *params.ParamsContext, local *params.ParamsRef, globals *params.GlobalParamsService) (result *APIValues, err error) {
	result, err = NewAPIValuesWithExecution(service, limit, cooperation)
	if err != nil {
		return nil, err
	}
	configured, err := NewRCFParams(context, globals, local)
	if err != nil {
		return nil, errors.Join(err, result.Close())
	}
	defer func() {
		closeErr := configured.Close()
		err = errors.Join(err, closeErr)
		if closeErr != nil && result != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	if result.usePrem, err = configured.UsePrem(); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	if result.cleanDenominators, err = configured.CleanDenominators(); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	if result.initialPrecision, err = configured.InitialPrecision(); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	result.store.initialPrecision = result.initialPrecision
	result.store.initialPrecisionBound = true
	if result.infPrecision, err = configured.InfPrecision(); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	if result.store.maxPrecision, err = configured.MaxPrecision(); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	result.store.maxPrecisionBound = true
	result.store.usePrem, result.store.usePremBound = result.usePrem, true
	if result.lazyNormalization, err = configured.LazyAlgebraicNormalization(); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	result.store.lazyNormalization = result.lazyNormalization
	result.store.lazyNormalizationBound = true
	result.store.infPrecision = result.infPrecision
	result.store.infPrecisionBound = true
	return result, nil
}

func (v *APIValues) wrap(value *realValue) *APIValue {
	v.live++
	return &APIValue{owner: v, value: value}
}

func (v *APIValues) valid(values ...*APIValue) bool {
	if v == nil || v.bounds == nil {
		return false
	}
	for _, value := range values {
		if value == nil || value.owner != v {
			return false
		}
	}
	return true
}

// Add, Sub, Mul and Div return owning values in the same real-closure context.
func (v *APIValues) Add(a, b *APIValue) (result *APIValue, err error) {
	if !v.valid(a, b) {
		return nil, arena.ErrInvalidSlot
	}
	defer v.store.intervalRestoreDefer(&err)()
	r, err := v.store.addRankedRetainedValues(a.value, b.value, v.usePrem)
	if err != nil {
		return nil, err
	}
	return v.wrap(r), nil
}
func (v *APIValues) Sub(a, b *APIValue) (result *APIValue, err error) {
	if !v.valid(a, b) {
		return nil, arena.ErrInvalidSlot
	}
	defer v.store.intervalRestoreDefer(&err)()
	r, err := v.store.subtractRankedRetainedValues(a.value, b.value, v.usePrem)
	if err != nil {
		return nil, err
	}
	return v.wrap(r), nil
}
func (v *APIValues) Mul(a, b *APIValue) (result *APIValue, err error) {
	if !v.valid(a, b) {
		return nil, arena.ErrInvalidSlot
	}
	defer v.store.intervalRestoreDefer(&err)()
	r, err := v.store.multiplyRankedRetainedValues(a.value, b.value, v.usePrem)
	if err != nil {
		return nil, err
	}
	return v.wrap(r), nil
}
func (v *APIValues) Div(a, b *APIValue) (result *APIValue, err error) {
	if !v.valid(a, b) {
		return nil, arena.ErrInvalidSlot
	}
	defer v.store.intervalRestoreDefer(&err)()
	r, err := v.store.divideRankedRetainedValues(a.value, b.value, v.usePrem)
	if err != nil {
		return nil, err
	}
	return v.wrap(r), nil
}
func (v *APIValues) Compare(a, b *APIValue) (comparison int, err error) {
	if !v.valid(a, b) {
		return 0, arena.ErrInvalidSlot
	}
	defer v.store.intervalRestoreDefer(&err)()
	return v.store.compareRankedRetainedValues(a.value, b.value, v.usePrem)
}

func (v *APIValues) IsZero(a *APIValue) (zero bool, err error) {
	if !v.valid(a) {
		return false, arena.ErrInvalidSlot
	}
	defer v.store.intervalRestoreDefer(&err)()
	sign, err := v.store.determineRankedRetainedValueSign(a.value, v.usePrem)
	return sign == 0, err
}

func (v *APIValues) Neg(a *APIValue) (result *APIValue, err error) {
	if !v.valid(a) {
		return nil, arena.ErrInvalidSlot
	}
	defer v.store.intervalRestoreDefer(&err)()
	r, err := v.store.negateRankedRetainedValue(a.value, v.usePrem)
	if err != nil {
		return nil, err
	}
	return v.wrap(r), nil
}

func (v *APIValues) Inv(a *APIValue) (result *APIValue, err error) {
	if !v.valid(a) {
		return nil, arena.ErrInvalidSlot
	}
	defer v.store.intervalRestoreDefer(&err)()
	r, err := v.store.inverseRankedRetainedValue(a.value, v.usePrem)
	if err != nil {
		return nil, err
	}
	return v.wrap(r), nil
}

func (v *APIValues) Power(a *APIValue, exponent uint32) (result *APIValue, err error) {
	if !v.valid(a) {
		return nil, arena.ErrInvalidSlot
	}
	defer v.store.intervalRestoreDefer(&err)()
	r, err := v.store.powerRankedRetainedValue(a.value, exponent, v.usePrem)
	if err != nil {
		return nil, err
	}
	return v.wrap(r), nil
}
func (v *APIValues) Display(w io.Writer, a *APIValue, compact, pretty bool) (err error) {
	if !v.valid(a) {
		return arena.ErrInvalidSlot
	}
	defer v.store.intervalRestoreDefer(&err)()
	if !a.value.isRational && a.value.root.kind != realExtensionAlgebraic {
		return v.store.displayRankedExpression(w, a.value, compact, pretty)
	}
	return v.store.displayValue(w, a.value, compact, pretty)
}

func (v *APIValues) DisplayDecimal(w io.Writer, a *APIValue, precision uint32) (err error) {
	if !v.valid(a) {
		return arena.ErrInvalidSlot
	}
	defer v.store.intervalRestoreDefer(&err)()
	return v.store.displayValueDecimal(w, a.value, v.bounds, precision)
}

// CleanDenominators returns owned numerator and denominator values whose
// quotient equals a. It follows manager::clean_denominators, including its
// recursive coefficient normalization for real-closure extensions.
func (v *APIValues) CleanDenominators(a *APIValue) (numerator, denominator *APIValue, err error) {
	if !v.valid(a) {
		return nil, nil, arena.ErrInvalidSlot
	}
	defer v.store.intervalRestoreDefer(&err)()
	n, d, err := v.store.cleanTowerValueIfNeeded(a.value, v.usePrem)
	if err != nil {
		return nil, nil, errors.Join(err, v.store.releaseSturmScratch(n), v.store.releaseSturmScratch(d))
	}
	return v.wrap(n), v.wrap(d), nil
}

func (v *APIValues) Close() error {
	if v == nil || v.bounds == nil || v.live != 0 {
		return arena.ErrInvalidSlot
	}
	for _, cached := range []**realValue{&v.pi, &v.e} {
		if *cached != nil {
			if err := v.store.release(*cached); err != nil {
				return err
			}
			*cached = nil
		}
	}
	if v.transcendentals != nil {
		if err := v.transcendentals.Close(); err != nil {
			return err
		}
		v.transcendentals = nil
	}
	if err := v.bounds.CloseChecked(); err != nil {
		return err
	}
	v.bounds = nil
	return nil
}

func (v *APIValue) Close() error {
	if v == nil || v.owner == nil || v.owner.bounds == nil {
		return arena.ErrInvalidSlot
	}
	if err := v.owner.store.release(v.value); err != nil {
		return err
	}
	v.owner.live--
	v.owner, v.value = nil, nil
	return nil
}

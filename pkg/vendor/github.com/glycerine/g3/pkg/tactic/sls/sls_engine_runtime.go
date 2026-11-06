package sls

// Source: src/tactic/sls/sls_engine.h:30-138 and
// src/tactic/sls/sls_engine.cpp:32-532,592-636.

import (
	"errors"
	"math"
	"time"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type SLSMoveType uint8

const (
	SLSMoveFlip SLSMoveType = iota
	SLSMoveIncrement
	SLSMoveDecrement
	SLSMoveInvert
)

type EngineStats struct {
	Restarts, FullEvaluations, IncrementalEvaluations uint32
	Moves, Flips, Increments, Decrements, Inversions  uint32
	Started                                           time.Time
}

type Engine struct {
	self       *Engine
	manager    *ast.ASTManager
	tracker    *Tracker
	search     *SearchTracker
	evaluator  *Evaluator
	numbers    *numeral.IntegerManager
	powers     *powers
	context    *uparams.ParamsContext
	globals    *uparams.GlobalParamsService
	local      *uparams.ParamsRef
	checkpoint func() error

	zero, one, two numeral.Int
	assertions     []ast.ASTHandle
	stats          EngineStats

	maxRestarts, wp, vnsMC, pawsSP, restartBase, restartNext uint32
	walksat, walksatRepick, vnsRepick, paws                  bool
	restartInit, earlyPrune, randomOffset, rescore           bool
	closing                                                  int
	closed                                                   bool
}

func NewEngine(manager *ast.ASTManager, tracker *Tracker, search *SearchTracker, evaluator *Evaluator, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, local *uparams.ParamsRef, checkpoint func() error) (*Engine, error) {
	if manager == nil || tracker == nil || tracker.manager != manager || tracker.open() != nil || search == nil || search.base != tracker || search.open() != nil || evaluator == nil || evaluator.tracker != tracker || evaluator.open() != nil || context == nil || globals == nil || globals.Context != context {
		return nil, ErrSLSSearch
	}
	e := &Engine{manager: manager, tracker: tracker, search: search, evaluator: evaluator, numbers: tracker.numbers, powers: tracker.powers, context: context, globals: globals, local: local, checkpoint: checkpoint, zero: numeral.NewInt(0), one: numeral.NewInt(1), two: numeral.NewInt(2)}
	e.self = e
	e.stats.Started = time.Now()
	if err := e.UpdateParams(local); err != nil {
		return nil, err
	}
	if err := e.tracker.UpdateParams(local); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *Engine) open() error {
	if e == nil || e.self != e || e.closed || e.tracker == nil || e.tracker.open() != nil || e.search == nil || e.search.open() != nil || e.evaluator == nil || e.evaluator.open() != nil {
		return ErrSLSSearch
	}
	return nil
}

func (e *Engine) UpdateParams(local *uparams.ParamsRef) (result error) {
	if e == nil || e.self != e || e.closed {
		return ErrSLSSearch
	}
	p, err := NewParams(e.context, e.globals, local)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, p.Close()) }()
	if e.maxRestarts, err = p.MaxRestarts(); err != nil {
		return err
	}
	if e.walksat, err = p.Walksat(); err != nil {
		return err
	}
	if e.walksatRepick, err = p.WalksatRepick(); err != nil {
		return err
	}
	if e.pawsSP, err = p.PawsSP(); err != nil {
		return err
	}
	e.paws = e.pawsSP < 1024
	if e.wp, err = p.WP(); err != nil {
		return err
	}
	if e.vnsMC, err = p.VNSMC(); err != nil {
		return err
	}
	if e.vnsRepick, err = p.VNSRepick(); err != nil {
		return err
	}
	if e.restartBase, err = p.RestartBase(); err != nil {
		return err
	}
	if e.restartBase == 0 {
		return ErrSLSSearch
	}
	e.restartNext = e.restartBase
	if e.restartInit, err = p.RestartInit(); err != nil {
		return err
	}
	if e.earlyPrune, err = p.EarlyPrune(); err != nil {
		return err
	}
	if e.randomOffset, err = p.RandomOffset(); err != nil {
		return err
	}
	if e.rescore, err = p.Rescore(); err != nil {
		return err
	}
	seed, err := p.RandomSeed()
	if err != nil {
		return err
	}
	if err = e.search.SetRandomSeed(seed); err != nil {
		return err
	}
	if e.walksatRepick && !e.walksat || e.vnsRepick && !e.walksat {
		return ErrEvaluatorUnsupported
	}
	e.local = local
	return nil
}

func (e *Engine) AssertExpression(expression ast.ASTHandle) error {
	if err := e.open(); err != nil {
		return err
	}
	kind, err := e.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	e.assertions = append(e.assertions, expression)
	return nil
}

func (e *Engine) checkpointNow() error {
	canceled, err := e.manager.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return tactic.NewTacticException(e.manager.ResourceLimit().CancelMessage())
	}
	if e.checkpoint != nil {
		return e.checkpoint()
	}
	return nil
}

func (e *Engine) MkAdd(bits uint32, oldValue, addValue, out *numeral.Int) (result error) {
	temp, mask, inverted := numeral.NewInt(0), numeral.NewInt(0), numeral.NewInt(0)
	defer func() {
		result = errors.Join(result, e.numbers.Del(&inverted))
		result = errors.Join(result, e.numbers.Del(&mask))
		result = errors.Join(result, e.numbers.Del(&temp))
	}()
	if err := e.numbers.Add(oldValue, addValue, &temp); err != nil {
		return err
	}
	power, err := e.powers.borrow(bits)
	if err != nil {
		return err
	}
	if err = e.numbers.Copy(&mask, power); err != nil {
		return err
	}
	if err = e.numbers.BitwiseNot(bits, &mask, &inverted); err != nil {
		return err
	}
	return e.numbers.BitwiseAnd(&temp, &inverted, out)
}

func (e *Engine) MkIncrement(bits uint32, oldValue, out *numeral.Int) error {
	if err := e.numbers.Add(oldValue, &e.one, out); err != nil {
		return err
	}
	power, err := e.powers.borrow(bits)
	if err != nil {
		return err
	}
	equal, err := e.numbers.Equal(out, power)
	if err != nil {
		return err
	}
	if equal {
		return e.numbers.Copy(out, &e.zero)
	}
	return nil
}

func (e *Engine) MkDecrement(bits uint32, oldValue, out *numeral.Int) error {
	zero, err := e.numbers.IsZero(oldValue)
	if err != nil {
		return err
	}
	if !zero {
		return e.numbers.Sub(oldValue, &e.one, out)
	}
	power, err := e.powers.borrow(bits)
	if err != nil {
		return err
	}
	if err = e.numbers.Copy(out, power); err != nil {
		return err
	}
	return e.numbers.Dec(out)
}

func (e *Engine) MkInvert(bits uint32, oldValue, out *numeral.Int) error {
	return e.numbers.BitwiseNot(bits, oldValue, out)
}

func (e *Engine) MkFlip(sort ast.ASTHandle, oldValue *numeral.Int, bit uint32, out *numeral.Int) error {
	boolSort, err := e.manager.MkBoolSort()
	if err != nil {
		return err
	}
	if sort == boolSort {
		zero, err := e.numbers.IsZero(oldValue)
		if err != nil {
			return err
		}
		if zero {
			return e.numbers.Copy(out, &e.one)
		}
		return e.numbers.Copy(out, &e.zero)
	}
	isBV, err := e.manager.IsBitVectorSort(sort)
	if err != nil || !isBV {
		if err != nil {
			return err
		}
		return ErrEvaluatorUnsupported
	}
	mask, err := e.powers.borrow(bit)
	if err != nil {
		return err
	}
	return e.numbers.BitwiseXor(oldValue, mask, out)
}

func (e *Engine) Rescore() (float64, error) {
	if err := e.evaluator.UpdateAll(e.search); err != nil {
		return 0, err
	}
	e.stats.FullEvaluations++
	return e.search.TopScore(e.assertions)
}

func (e *Engine) incrementalScore(decl ast.ASTHandle, value *numeral.Int) (float64, error) {
	if err := e.evaluator.UpdateConstant(e.search, decl, value); err != nil {
		return 0, err
	}
	e.stats.IncrementalEvaluations++
	return e.tracker.TopSum()
}

func (e *Engine) incrementalScorePrune(decl ast.ASTHandle, value *numeral.Int) (float64, error) {
	e.stats.IncrementalEvaluations++
	benefit, err := e.evaluator.UpdatePrune(e.search, decl, value)
	if err != nil {
		return 0, err
	}
	if !benefit {
		return -math.MaxFloat64, nil
	}
	return e.tracker.TopSum()
}

func (e *Engine) seriousScore(decl ast.ASTHandle, value *numeral.Int) (float64, error) {
	if err := e.evaluator.SeriousUpdate(e.search, decl, value); err != nil {
		return 0, err
	}
	e.stats.IncrementalEvaluations++
	return e.tracker.TopSum()
}

func (e *Engine) whatIf(decl ast.ASTHandle, index uint32, candidate *numeral.Int, bestScore *float64, bestIndex *uint32, bestValue *numeral.Int) (bool, error) {
	var score float64
	var err error
	if e.earlyPrune {
		score, err = e.incrementalScorePrune(decl, candidate)
	} else {
		score, err = e.incrementalScore(decl, candidate)
	}
	if err != nil {
		return false, err
	}
	if score > *bestScore {
		*bestScore, *bestIndex = score, index
		if err = e.numbers.Copy(bestValue, candidate); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (e *Engine) FindBestMove(declarations []ast.ASTHandle, score float64, bestValue *numeral.Int) (newScore float64, bestIndex uint32, bit uint32, move SLSMoveType, found bool, result error) {
	newScore = score
	bestIndex = ^uint32(0)
	oldValue, candidate := numeral.NewInt(0), numeral.NewInt(0)
	defer func() {
		result = errors.Join(result, e.numbers.Del(&candidate))
		result = errors.Join(result, e.numbers.Del(&oldValue))
	}()
	offset := uint32(0)
	if e.randomOffset && len(declarations) != 0 {
		random, err := e.search.RandomUint(16)
		if err != nil {
			return 0, 0, 0, 0, false, err
		}
		offset = random % uint32(len(declarations))
	}
	for j := uint32(0); j < uint32(len(declarations)); j++ {
		i := j + offset
		if i >= uint32(len(declarations)) {
			i -= uint32(len(declarations))
		}
		decl := declarations[i]
		sort, err := e.manager.GetRange(decl)
		if err != nil {
			return 0, 0, 0, 0, false, err
		}
		tracked, err := e.tracker.DeclValue(decl)
		if err != nil {
			return 0, 0, 0, 0, false, err
		}
		if err = e.numbers.Copy(&oldValue, tracked); err != nil {
			return 0, 0, 0, 0, false, err
		}
		boolSort, err := e.manager.MkBoolSort()
		if err != nil {
			return 0, 0, 0, 0, false, err
		}
		bits := uint32(1)
		if sort != boolSort {
			if bits, err = e.manager.BitVectorSize(sort); err != nil {
				return 0, 0, 0, 0, false, err
			}
		}
		for k := uint32(0); k < bits; k++ {
			if err = e.MkFlip(sort, &oldValue, k, &candidate); err != nil {
				return 0, 0, 0, 0, false, err
			}
			better, err := e.whatIf(decl, i, &candidate, &newScore, &bestIndex, bestValue)
			if err != nil {
				return 0, 0, 0, 0, false, err
			}
			if better {
				bit, move, found = k, SLSMoveFlip, true
			}
		}
		if sort != boolSort && bits > 1 {
			even, err := e.numbers.IsEven(&oldValue)
			if err != nil {
				return 0, 0, 0, 0, false, err
			}
			if !even {
				err = e.MkIncrement(bits, &oldValue, &candidate)
			} else {
				err = e.MkDecrement(bits, &oldValue, &candidate)
			}
			if err != nil {
				return 0, 0, 0, 0, false, err
			}
			better, err := e.whatIf(decl, i, &candidate, &newScore, &bestIndex, bestValue)
			if err != nil {
				return 0, 0, 0, 0, false, err
			}
			found = found || better
			if better {
				if even {
					move = SLSMoveDecrement
				} else {
					move = SLSMoveIncrement
				}
			}
			if err = e.MkInvert(bits, &oldValue, &candidate); err != nil {
				return 0, 0, 0, 0, false, err
			}
			better, err = e.whatIf(decl, i, &candidate, &newScore, &bestIndex, bestValue)
			if err != nil {
				return 0, 0, 0, 0, false, err
			}
			if better {
				move, found = SLSMoveInvert, true
			}
		}
		if _, err = e.incrementalScore(decl, &oldValue); err != nil {
			return 0, 0, 0, 0, false, err
		}
	}
	return newScore, bestIndex, bit, move, found, nil
}

func (e *Engine) FindBestMoveMonteCarlo(declarations []ast.ASTHandle, score float64, bestValue *numeral.Int) (newScore float64, bestIndex uint32, found bool, result error) {
	newScore, bestIndex = score, ^uint32(0)
	oldValue, first, second := numeral.NewInt(0), numeral.NewInt(0), numeral.NewInt(0)
	defer func() {
		result = errors.Join(result, e.numbers.Del(&second))
		result = errors.Join(result, e.numbers.Del(&first))
		result = errors.Join(result, e.numbers.Del(&oldValue))
	}()
	offset := uint32(0)
	if e.randomOffset && len(declarations) != 0 {
		random, err := e.search.RandomUint(16)
		if err != nil {
			return 0, 0, false, err
		}
		offset = random % uint32(len(declarations))
	}
	for j := uint32(0); j < uint32(len(declarations)); j++ {
		i := j + offset
		if i >= uint32(len(declarations)) {
			i -= uint32(len(declarations))
		}
		decl := declarations[i]
		sort, err := e.manager.GetRange(decl)
		if err != nil {
			return 0, 0, false, err
		}
		isBV, err := e.manager.IsBitVectorSort(sort)
		if err != nil {
			return 0, 0, false, err
		}
		value, err := e.tracker.DeclValue(decl)
		if err != nil {
			return 0, 0, false, err
		}
		if err = e.numbers.Copy(&oldValue, value); err != nil {
			return 0, 0, false, err
		}
		if isBV {
			bits, err := e.manager.BitVectorSize(sort)
			if err != nil {
				return 0, 0, false, err
			}
			if bits > 2 {
				for bit := uint32(0); bit < bits; bit++ {
					if err = e.MkFlip(sort, &oldValue, bit, &first); err != nil {
						return 0, 0, false, err
					}
					for sample := uint32(0); sample < e.vnsMC && sample < bits/2; sample++ {
						other, x := e.search.RandomUint(16)
						if x != nil {
							return 0, 0, false, x
						}
						other %= bits
						for other == bit {
							other, x = e.search.RandomUint(16)
							if x != nil {
								return 0, 0, false, x
							}
							other %= bits
						}
						if x = e.MkFlip(sort, &first, other, &second); x != nil {
							return 0, 0, false, x
						}
						better, x := e.whatIf(decl, i, &second, &newScore, &bestIndex, bestValue)
						if x != nil {
							return 0, 0, false, x
						}
						found = found || better
					}
				}
			}
		}
		if _, err = e.incrementalScore(decl, &oldValue); err != nil {
			return 0, 0, false, err
		}
	}
	return newScore, bestIndex, found, nil
}

func (e *Engine) RandomMove(declarations []ast.ASTHandle) (result error) {
	if len(declarations) == 0 {
		return ErrSLSSearch
	}
	width := uint32(32)
	switch count := len(declarations); {
	case count < 16:
		width = 4
	case count < 256:
		width = 8
	case count < 4096:
		width = 12
	case count < 65536:
		width = 16
	}
	random, err := e.search.RandomUint(width)
	if err != nil {
		return err
	}
	decl := declarations[random%uint32(len(declarations))]
	sort, err := e.manager.GetRange(decl)
	if err != nil {
		return err
	}
	oldValue, err := e.tracker.DeclValue(decl)
	if err != nil {
		return err
	}
	value := numeral.NewInt(0)
	defer func() { result = errors.Join(result, e.numbers.Del(&value)) }()
	boolSort, err := e.manager.MkBoolSort()
	if err != nil {
		return err
	}
	if sort == boolSort {
		if err = e.MkFlip(sort, oldValue, 0, &value); err != nil {
			return err
		}
	} else {
		bits, err := e.manager.BitVectorSize(sort)
		if err != nil {
			return err
		}
		first, err := e.search.RandomBool()
		if err != nil {
			return err
		}
		firstOne, err := e.numbers.IsOne(first)
		if err != nil {
			return err
		}
		second, err := e.search.RandomBool()
		if err != nil {
			return err
		}
		secondOne, err := e.numbers.IsOne(second)
		if err != nil {
			return err
		}
		choice := uint32(0)
		if firstOne {
			choice = 2
		}
		if secondOne {
			choice++
		}
		if choice == 3 {
			choice = 0
		}
		switch choice {
		case 0:
			width := uint32(32)
			switch {
			case bits < 16:
				width = 4
			case bits < 256:
				width = 8
			case bits < 4096:
				width = 12
			case bits < 65536:
				width = 16
			}
			position, x := e.search.RandomUint(width)
			if x != nil {
				return x
			}
			err = e.MkFlip(sort, oldValue, position%bits, &value)
		case 1:
			err = e.MkIncrement(bits, oldValue, &value)
		default:
			err = e.MkDecrement(bits, oldValue, &value)
		}
		if err != nil {
			return err
		}
	}
	return e.evaluator.SeriousUpdate(e.search, decl, &value)
}

func (e *Engine) CheckRestart(current uint32) bool {
	if current > e.restartNext {
		if e.stats.Restarts&1 != 0 {
			e.restartNext += e.restartBase
		} else {
			shift := e.stats.Restarts >> 1
			if shift >= 31 {
				e.restartNext = ^uint32(0)
			} else {
				e.restartNext += (2 << shift) * e.restartBase
			}
		}
		return false
	}
	return true
}

func (e *Engine) Search() (status lbool.LBool, result error) {
	score, err := e.Rescore()
	if err != nil {
		return lbool.Undef, err
	}
	bestValue := numeral.NewInt(0)
	defer func() { result = errors.Join(result, e.numbers.Del(&bestValue)) }()
	for e.CheckRestart(e.stats.Moves) {
		if err = e.checkpointNow(); err != nil {
			return lbool.Undef, err
		}
		e.stats.Moves++
		if e.stats.Moves%e.restartBase == 0 {
			if err = e.tracker.ucbForgetExpressions(e.assertions); err != nil {
				return lbool.Undef, err
			}
			if e.rescore {
				if score, err = e.Rescore(); err != nil {
					return lbool.Undef, err
				}
			}
		}
		declarations, err := e.search.UnsatConstants(e.assertions)
		if err != nil {
			return lbool.Undef, err
		}
		if len(declarations) == 0 {
			return lbool.True, nil
		}
		if e.wp != 0 {
			random, x := e.search.RandomUint(10)
			if x != nil {
				return lbool.Undef, x
			}
			if random < e.wp {
				if x = e.RandomMove(declarations); x != nil {
					return lbool.Undef, x
				}
				score, _ = e.tracker.TopSum()
				continue
			}
		}
		oldScore := score
		var index uint32
		var found bool
		score, index, _, _, found, err = e.FindBestMove(declarations, score, &bestValue)
		if err != nil {
			return lbool.Undef, err
		}
		if !found && e.vnsMC != 0 {
			score, index, found, err = e.FindBestMoveMonteCarlo(declarations, score, &bestValue)
			if err != nil {
				return lbool.Undef, err
			}
		}
		if !found && e.vnsRepick {
			root, selected, x := e.search.NewUnsatAssertion(e.assertions)
			if x != nil {
				return lbool.Undef, x
			}
			// For multiple assertions the source repick clears the shared
			// temporary candidate vector, even if no alternate is found.
			if len(e.assertions) > 1 {
				declarations = nil
			}
			if selected {
				alternatives, x := e.search.ConstantsForRoot(root)
				if x != nil {
					return lbool.Undef, x
				}
				// Both source candidate references name the same temporary
				// vector, so the fallback sees the repicked constants too.
				declarations = alternatives
				score, index, _, _, found, x = e.FindBestMove(alternatives, score, &bestValue)
				if x != nil {
					return lbool.Undef, x
				}
				if found {
					if int(index) >= len(alternatives) {
						return lbool.Undef, ErrSLSSearch
					}
					if score, x = e.seriousScore(alternatives[index], &bestValue); x != nil {
						return lbool.Undef, x
					}
					continue
				}
			}
		}
		if !found {
			score = oldScore
			if e.walksatRepick {
				declarations, err = e.search.UnsatConstants(e.assertions)
				if err != nil {
					return lbool.Undef, err
				}
			}
			if err = e.evaluator.RandomizeLocal(e.search, declarations); err != nil {
				return lbool.Undef, err
			}
			score, _ = e.tracker.TopSum()
			if e.paws {
				for _, root := range e.assertions {
					random, x := e.search.RandomUint(10)
					if x != nil {
						return lbool.Undef, x
					}
					value, x := e.tracker.GetValue(root)
					if x != nil {
						return lbool.Undef, x
					}
					one, x := e.numbers.IsOne(value)
					if x != nil {
						return lbool.Undef, x
					}
					if random < e.pawsSP && one {
						x = e.search.DecreaseWeight(root)
					} else if random >= e.pawsSP && !one {
						x = e.search.IncreaseWeight(root)
					}
					if x != nil {
						return lbool.Undef, x
					}
				}
			}
		} else {
			if int(index) >= len(declarations) {
				return lbool.Undef, ErrSLSSearch
			}
			if score, err = e.seriousScore(declarations[index], &bestValue); err != nil {
				return lbool.Undef, err
			}
		}
	}
	return lbool.Undef, nil
}

func (e *Engine) Solve() (lbool.LBool, error) {
	if err := e.open(); err != nil {
		return lbool.Undef, err
	}
	if err := e.search.Initialize(e.assertions); err != nil {
		return lbool.Undef, err
	}
	// The source always establishes the all-zero model before optionally
	// randomizing it. Randomize performs incremental updates against that model.
	if err := e.search.Reset(); err != nil {
		return lbool.Undef, err
	}
	if e.restartInit {
		if err := e.search.Randomize(); err != nil {
			return lbool.Undef, err
		}
	}
	for {
		if err := e.checkpointNow(); err != nil {
			return lbool.Undef, err
		}
		if err := tactic.ReportTacticProgress("Searching... restarts left:", e.maxRestarts-e.stats.Restarts); err != nil {
			return lbool.Undef, err
		}
		status, err := e.Search()
		if err != nil {
			return status, err
		}
		if status == lbool.True {
			return status, e.reportSearchSummary()
		}
		if e.restartInit {
			err = e.search.Randomize()
		} else {
			err = e.search.Reset()
		}
		if err != nil {
			return lbool.Undef, err
		}
		// Preserve the post-increment in the source do/while condition: even a
		// final unsuccessful search is counted as a restart attempt.
		previous := e.stats.Restarts
		e.stats.Restarts++
		if previous >= e.maxRestarts {
			return status, e.reportSearchSummary()
		}
	}
}

func (e *Engine) Stats() (EngineStats, error) {
	if err := e.open(); err != nil {
		return EngineStats{}, err
	}
	return e.stats, nil
}

func (e *Engine) Close() error {
	if e == nil || e.self != e {
		return ErrSLSSearch
	}
	if e.closed {
		return nil
	}
	values := []*numeral.Int{&e.zero, &e.one, &e.two}
	for e.closing < len(values) {
		if err := e.numbers.Del(values[e.closing]); err != nil {
			return err
		}
		e.closing++
	}
	e.assertions = nil
	e.closed = true
	return nil
}

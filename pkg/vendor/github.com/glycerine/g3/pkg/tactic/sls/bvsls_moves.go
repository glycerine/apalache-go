package sls

// Source: src/tactic/sls/bvsls_opt_engine.cpp:118-348.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (o *BVOptimizer) objectiveScore(out *numeral.Int) error {
	value, err := o.objective.GetValue(o.objectiveExpr)
	if err != nil {
		return err
	}
	return o.hard.numbers.Copy(out, value)
}

// whatIf leaves both source trackers at the candidate. findBestMove restores
// each declaration after exhausting its candidate moves.
func (o *BVOptimizer) whatIf(decl ast.ASTHandle, index uint32, candidate, bestScore, bestValue *numeral.Int, bestIndex *uint32) (bool, error) {
	if _, err := o.hard.incrementalScore(decl, candidate); err != nil {
		return false, err
	}
	sat, err := o.hard.tracker.IsSAT()
	if err != nil || !sat {
		return false, err
	}
	if err = o.evaluator.UpdateConstant(o.search, decl, candidate); err != nil {
		return false, err
	}
	current := numeral.NewInt(0)
	defer o.hard.numbers.Del(&current)
	if err = o.objectiveScore(&current); err != nil {
		return false, err
	}
	better, err := o.hard.numbers.Greater(&current, bestScore)
	if err != nil || !better {
		return false, err
	}
	if err = o.hard.numbers.Copy(bestScore, &current); err != nil {
		return false, err
	}
	if err = o.hard.numbers.Copy(bestValue, candidate); err != nil {
		return false, err
	}
	*bestIndex = index
	return true, nil
}

func (o *BVOptimizer) findBestMove(declarations []ast.ASTHandle, score, bestValue, maximum *numeral.Int) (bestScore numeral.Int, bestIndex, newBit uint32, move SLSMoveType, found bool, result error) {
	bestScore = numeral.NewInt(0)
	bestIndex = math.MaxUint32
	oldValue, candidate := numeral.NewInt(0), numeral.NewInt(0)
	defer func() {
		result = errors.Join(result, o.hard.numbers.Del(&candidate))
		result = errors.Join(result, o.hard.numbers.Del(&oldValue))
	}()
	if err := o.hard.numbers.Copy(&bestScore, score); err != nil {
		return bestScore, bestIndex, 0, 0, false, err
	}
	for i, decl := range declarations {
		atMax, err := o.hard.numbers.GreaterEqual(&bestScore, maximum)
		if err != nil || atMax {
			return bestScore, bestIndex, newBit, move, found, err
		}
		sort, err := o.hard.manager.GetRange(decl)
		if err != nil {
			return bestScore, bestIndex, 0, 0, false, err
		}
		bits := uint32(1)
		boolSort, err := o.hard.manager.MkBoolSort()
		if err != nil {
			return bestScore, bestIndex, 0, 0, false, err
		}
		if sort != boolSort {
			bits, err = o.hard.manager.BitVectorSize(sort)
			if err != nil {
				return bestScore, bestIndex, 0, 0, false, err
			}
		}
		tracked, err := o.objective.DeclValue(decl)
		if err != nil {
			return bestScore, bestIndex, 0, 0, false, err
		}
		if err = o.hard.numbers.Copy(&oldValue, tracked); err != nil {
			return bestScore, bestIndex, 0, 0, false, err
		}
		for bit := uint32(0); bit < bits; bit++ {
			if err = o.hard.MkFlip(sort, &oldValue, bit, &candidate); err != nil {
				return bestScore, bestIndex, 0, 0, false, err
			}
			better, x := o.whatIf(decl, uint32(i), &candidate, &bestScore, bestValue, &bestIndex)
			if x != nil {
				return bestScore, bestIndex, 0, 0, false, x
			}
			if better {
				newBit, move, found = bit, SLSMoveFlip, true
			}
		}
		if sort != boolSort && bits > 1 {
			// The source's _USE_ADDSUB_, _USE_UNARY_MINUS_ and
			// _USE_MUL2DIV2_ switches are undefined in the retained build, so
			// only the unconditional bit flips and bitwise inversion run here.
			if x := o.hard.MkInvert(bits, &oldValue, &candidate); x != nil {
				return bestScore, bestIndex, 0, 0, false, x
			}
			better, x := o.whatIf(decl, uint32(i), &candidate, &bestScore, bestValue, &bestIndex)
			if x != nil {
				return bestScore, bestIndex, 0, 0, false, x
			}
			if better {
				move, found = SLSMoveInvert, true
			}
		}
		if _, err = o.hard.incrementalScore(decl, &oldValue); err != nil {
			return bestScore, bestIndex, 0, 0, false, err
		}
		if err = o.evaluator.UpdateConstant(o.search, decl, &oldValue); err != nil {
			return bestScore, bestIndex, 0, 0, false, err
		}
	}
	return bestScore, bestIndex, newBit, move, found, nil
}

func (o *BVOptimizer) randomizeWrtHard() (result bool, resultErr error) {
	declarations, err := o.objective.Constants()
	if err != nil || len(declarations) == 0 {
		return false, err
	}
	randomValue, oldValue := numeral.NewInt(0), numeral.NewInt(0)
	defer func() {
		resultErr = errors.Join(resultErr, o.hard.numbers.Del(&oldValue))
		resultErr = errors.Join(resultErr, o.hard.numbers.Del(&randomValue))
	}()
	for attempts := len(declarations); attempts > 0; attempts-- {
		bits := uint32(32)
		switch n := len(declarations); {
		case n < 16:
			bits = 4
		case n < 256:
			bits = 8
		case n < 4096:
			bits = 12
		case n < 65536:
			bits = 16
		}
		random, err := o.search.RandomUint(bits)
		if err != nil {
			return false, err
		}
		decl := declarations[int(random)%len(declarations)]
		sort, err := o.hard.manager.GetRange(decl)
		if err != nil {
			return false, err
		}
		if err = o.search.RandomValue(sort, &randomValue); err != nil {
			return false, err
		}
		old, err := o.objective.DeclValue(decl)
		if err != nil {
			return false, err
		}
		if err = o.hard.numbers.Copy(&oldValue, old); err != nil {
			return false, err
		}
		equal, err := o.hard.numbers.Equal(&randomValue, &oldValue)
		if err != nil || equal {
			continue
		}
		if err = o.hard.evaluator.UpdateConstant(o.hard.search, decl, &randomValue); err != nil {
			return false, err
		}
		sat, err := o.hard.tracker.IsSAT()
		if err != nil {
			return false, err
		}
		if sat {
			if err = o.evaluator.UpdateConstant(o.search, decl, &randomValue); err != nil {
				return false, err
			}
			return true, nil
		}
		if err = o.hard.evaluator.UpdateConstant(o.hard.search, decl, &oldValue); err != nil {
			return false, err
		}
	}
	return false, nil
}

func (o *BVOptimizer) maximizeObjective() (optimum ast.ASTHandle, result error) {
	score, oldScore, maximum, bestValue := numeral.NewInt(0), numeral.NewInt(0), numeral.NewInt(0), numeral.NewInt(0)
	defer func() {
		result = errors.Join(result, o.hard.numbers.Del(&bestValue))
		result = errors.Join(result, o.hard.numbers.Del(&maximum))
		result = errors.Join(result, o.hard.numbers.Del(&oldScore))
		result = errors.Join(result, o.hard.numbers.Del(&score))
	}()
	if err := o.objectiveScore(&score); err != nil {
		return ast.ASTHandle{}, err
	}
	power, err := o.hard.powers.borrow(o.objectiveBits)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = o.hard.numbers.Copy(&maximum, power); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = o.hard.numbers.Dec(&maximum); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = o.saveModel(&score); err != nil {
		return ast.ASTHandle{}, err
	}
	declarations, err := o.objective.Constants()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for {
		below, err := o.hard.numbers.Less(&score, &maximum)
		if err != nil || !below || !o.hard.CheckRestart(o.hard.stats.Moves) {
			break
		}
		if err = o.hard.checkpointNow(); err != nil {
			return ast.ASTHandle{}, err
		}
		o.hard.stats.Moves++
		if err = o.hard.numbers.Copy(&oldScore, &score); err != nil {
			return ast.ASTHandle{}, err
		}
		bestScore, bestIndex, _, _, found, err := o.findBestMove(declarations, &score, &bestValue, &maximum)
		if err != nil {
			_ = o.hard.numbers.Del(&bestScore)
			return ast.ASTHandle{}, err
		}
		if !found || bestIndex == math.MaxUint32 {
			if err = o.hard.numbers.Copy(&score, &oldScore); err == nil {
				var better bool
				better, err = o.hard.numbers.Greater(&score, &o.bestScore)
				if err == nil && better {
					err = o.saveModel(&score)
				}
			}
			if err != nil {
				_ = o.hard.numbers.Del(&bestScore)
				return ast.ASTHandle{}, err
			}
			randomized, x := o.randomizeWrtHard()
			if x != nil {
				_ = o.hard.numbers.Del(&bestScore)
				return ast.ASTHandle{}, x
			}
			if !randomized {
				_ = o.hard.numbers.Del(&bestScore)
				break
			}
			if err = o.objectiveScore(&score); err != nil {
				_ = o.hard.numbers.Del(&bestScore)
				return ast.ASTHandle{}, err
			}
		} else {
			o.hard.stats.Moves++
			decl := declarations[bestIndex]
			if _, err = o.hard.seriousScore(decl, &bestValue); err == nil {
				err = o.evaluator.SeriousUpdate(o.search, decl, &bestValue)
			}
			if err == nil {
				err = o.objectiveScore(&score)
			}
			if err != nil {
				_ = o.hard.numbers.Del(&bestScore)
				return ast.ASTHandle{}, err
			}
		}
		if err = o.hard.numbers.Del(&bestScore); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	// Preserve the source save points, including its maximum-value early-exit
	// behavior; the returned numeral is the last saved model score.
	return o.optimumAST()
}

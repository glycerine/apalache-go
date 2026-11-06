package opt

// Source: src/opt/pb_sls.h and src/opt/pb_sls.cpp:27-186.

import (
	"errors"
	"math"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	uparams "github.com/glycerine/g3/pkg/util/params"
	z3rand "github.com/glycerine/g3/pkg/util/random"
)

var ErrPBSLS = errors.New("opt: invalid pseudo-Boolean SLS state")

type pbSLSLiteral struct {
	variable uint32
	negated  bool
}

func (l pbSLSLiteral) negate() pbSLSLiteral { l.negated = !l.negated; return l }

var pbSLSNullLiteral = pbSLSLiteral{variable: math.MaxUint32}
var pbSLSTrueLiteral = pbSLSLiteral{}
var pbSLSFalseLiteral = pbSLSLiteral{negated: true}

type pbSLSIndexSet struct {
	elements []uint32
	index    []uint32
}

func (s *pbSLSIndexSet) reset()      { s.elements, s.index = nil, nil }
func (s *pbSLSIndexSet) empty() bool { return len(s.elements) == 0 }
func (s *pbSLSIndexSet) contains(value uint32) bool {
	return int(value) < len(s.index) && int(s.index[value]) < len(s.elements) && s.elements[s.index[value]] == value
}
func (s *pbSLSIndexSet) insert(value uint32) {
	for len(s.index) <= int(value) {
		s.index = append(s.index, math.MaxUint32)
	}
	if s.contains(value) {
		return
	}
	s.index[value] = uint32(len(s.elements))
	s.elements = append(s.elements, value)
}
func (s *pbSLSIndexSet) remove(value uint32) {
	if !s.contains(value) {
		return
	}
	position := s.index[value]
	last := s.elements[len(s.elements)-1]
	s.elements[position] = last
	s.index[last] = position
	s.elements = s.elements[:len(s.elements)-1]
}
func (s *pbSLSIndexSet) choose(random *z3rand.Generator) (uint32, error) {
	if s.empty() {
		return 0, ErrPBSLS
	}
	return s.elements[random.Bounded(uint32(len(s.elements)))], nil
}

type pbSLSClause struct {
	literals []pbSLSLiteral
	weights  []big.Int
	bound    big.Int
	value    big.Int
	equality bool
}

type PBSLSStatistics struct {
	Flips, Improvements uint32
}

type PBSLSConfig struct {
	Context *uparams.ParamsContext
	Globals *uparams.GlobalParamsService
	Model   model.ModelEvaluatorConfig
}

type PBSLS struct {
	self    *PBSLS
	manager *ast.ASTManager
	config  PBSLSConfig
	rewrite *rewriter.TheoryRewriter

	hard, soft                 []pbSLSClause
	hardOrigins                []ast.ASTHandle
	softWeights                []big.Rat
	penalty, best              big.Rat
	hardOccurrences            [][]uint32
	softOccurrences            [][]uint32
	assignment, bestAssignment []bool
	variables                  map[ast.ASTHandle]uint32
	variableExprs              []ast.ASTHandle
	hardFalse, softFalse       pbSLSIndexSet
	maxFlips, nonGreedyPercent uint32
	random                     z3rand.Generator
	stats                      PBSLSStatistics
	originalModel              *model.Model
	closed                     bool
}

func NewPBSLS(manager *ast.ASTManager, config PBSLSConfig) (solver *PBSLS, result error) {
	if manager == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || config.Model.Context != config.Context || config.Model.Globals != config.Globals || config.Model.Memory == nil || config.Model.Cooperation == nil {
		return nil, ErrPBSLS
	}
	if err := config.Context.CheckServices(manager.Symbols, manager.NumeralService()); err != nil {
		return nil, err
	}
	if _, err := manager.RegisterPseudoBooleanPlugin(); err != nil {
		return nil, err
	}
	rw, err := rewriter.NewTheoryRewriter(manager, config.Context, config.Globals, config.Model.Params)
	if err != nil {
		return nil, err
	}
	s := &PBSLS{manager: manager, config: config, rewrite: rw}
	s.self = s
	defer func() {
		if result != nil {
			result = errors.Join(result, s.Close())
			solver = nil
		}
	}()
	if err = s.resetState(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *PBSLS) open() error {
	if s == nil || s.self != s || s.closed || s.manager == nil || s.rewrite == nil {
		return ErrPBSLS
	}
	return nil
}

func (s *PBSLS) releaseState() (result error) {
	for i := len(s.hardOrigins); i > 0; i-- {
		result = errors.Join(result, s.manager.DecRef(s.hardOrigins[i-1]))
	}
	for i := len(s.variableExprs); i > 0; i-- {
		result = errors.Join(result, s.manager.DecRef(s.variableExprs[i-1]))
	}
	if s.originalModel != nil {
		result = errors.Join(result, s.originalModel.Close())
		s.originalModel = nil
	}
	if result != nil {
		return result
	}
	s.hard, s.soft, s.hardOrigins, s.softWeights = nil, nil, nil, nil
	s.hardOccurrences, s.softOccurrences = nil, nil
	s.assignment, s.bestAssignment = nil, nil
	s.variables, s.variableExprs = nil, nil
	s.hardFalse.reset()
	s.softFalse.reset()
	return nil
}

func (s *PBSLS) resetState() (result error) {
	if err := s.releaseState(); err != nil {
		return err
	}
	s.maxFlips, s.nonGreedyPercent = 200, 30
	s.variables = make(map[ast.ASTHandle]uint32)
	s.penalty.SetInt64(0)
	s.best.SetInt64(0)
	truth, err := s.manager.MkTrue()
	if err != nil {
		return err
	}
	if err = s.manager.IncRef(truth); err != nil {
		return err
	}
	s.variables[truth] = 0
	s.variableExprs = append(s.variableExprs, truth)
	s.assignment = append(s.assignment, true)
	s.hardOccurrences = append(s.hardOccurrences, nil)
	s.softOccurrences = append(s.softOccurrences, nil)
	return nil
}

func (s *PBSLS) Reset() error {
	if err := s.open(); err != nil {
		return err
	}
	return s.resetState()
}

func (s *PBSLS) UpdateParams(*uparams.ParamsRef) error { return s.open() }

func (s *PBSLS) Statistics() (map[string]float64, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return map[string]float64{"sls.num_flips": float64(s.stats.Flips), "sls.num_improvements": float64(s.stats.Improvements)}, nil
}

func (s *PBSLS) Close() (result error) {
	if s == nil || s.self != s {
		return ErrPBSLS
	}
	if s.closed {
		return nil
	}
	result = errors.Join(result, s.releaseState())
	if s.rewrite != nil {
		result = errors.Join(result, s.rewrite.Close())
		s.rewrite = nil
	}
	if result == nil {
		s.closed = true
	}
	return result
}

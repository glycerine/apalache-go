package sls

// Source: src/tactic/sls/sls_tracker.h:43-298.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

var ErrTracker = errors.New("sls: invalid tracker owner, expression, or state")

type trackerValueScore struct {
	value      numeral.Int
	score      float64
	scorePrune float64
	hasPosOcc  uint32
	hasNegOcc  uint32
	distance   uint32
	touched    uint32
	destroyed  bool
}

// Tracker owns the source value_score numerals and borrows AST identities,
// parameter services, the integer manager, and the power cache.
type Tracker struct {
	self    *Tracker
	manager *ast.ASTManager
	numbers *numeral.IntegerManager
	powers  *powers
	context *uparams.ParamsContext
	globals *uparams.GlobalParamsService

	zero, one, two numeral.Int
	scores         map[ast.ASTHandle]*trackerValueScore
	scoreOrder     []ast.ASTHandle
	uplinks        map[ast.ASTHandle][]ast.ASTHandle
	entryPoints    map[ast.ASTHandle]ast.ASTHandle
	topExpressions map[ast.ASTHandle]struct{}
	weights        map[ast.ASTHandle]uint32
	constants      []ast.ASTHandle
	constantSet    map[ast.ASTHandle]struct{}

	topSum      float64
	walksat     bool
	ucb         bool
	ucbConstant float64
	ucbInit     bool
	ucbForget   float64
	ucbNoise    float64
	touched     uint32
	scaleUnsat  float64
	pawsInit    uint32
	trackUnsat  bool

	closePhase int
	closeScore int
	closed     bool
}

func NewTracker(manager *ast.ASTManager, numbers *numeral.IntegerManager, powers *powers, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (*Tracker, error) {
	if manager == nil || numbers == nil || numbers.Closed() || powers == nil || powers.closed || powers.manager != numbers || context == nil || globals == nil || globals.Context != context {
		return nil, ErrTracker
	}
	t := &Tracker{
		manager: manager, numbers: numbers, powers: powers, context: context, globals: globals,
		zero: numeral.NewInt(0), one: numeral.NewInt(1), two: numeral.NewInt(2),
		scores:         make(map[ast.ASTHandle]*trackerValueScore),
		uplinks:        make(map[ast.ASTHandle][]ast.ASTHandle),
		entryPoints:    make(map[ast.ASTHandle]ast.ASTHandle),
		topExpressions: make(map[ast.ASTHandle]struct{}),
		weights:        make(map[ast.ASTHandle]uint32),
		constantSet:    make(map[ast.ASTHandle]struct{}),
	}
	t.self = t
	return t, nil
}

func (t *Tracker) open() error {
	if t == nil || t.self != t || t.closed || t.manager == nil || t.numbers == nil || t.numbers.Closed() || t.powers == nil || t.powers.closed {
		return ErrTracker
	}
	return nil
}

func (t *Tracker) requireExpr(h ast.ASTHandle) error {
	if err := t.open(); err != nil {
		return err
	}
	kind, err := t.manager.GetKind(h)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp && kind != ast.ASTVar && kind != ast.ASTQuantifier {
		return ErrTracker
	}
	return nil
}

// trackExpr is the value_score construction prefix of initialize(app*).
func (t *Tracker) trackExpr(h ast.ASTHandle) (*trackerValueScore, error) {
	if err := t.requireExpr(h); err != nil {
		return nil, err
	}
	if score := t.scores[h]; score != nil {
		return score, nil
	}
	score := &trackerValueScore{value: numeral.NewInt(0), touched: 1}
	t.scores[h] = score
	t.scoreOrder = append(t.scoreOrder, h)
	return score, nil
}

func (t *Tracker) existingScore(h ast.ASTHandle) (*trackerValueScore, error) {
	if err := t.requireExpr(h); err != nil {
		return nil, err
	}
	score := t.scores[h]
	if score == nil || score.destroyed {
		return nil, ErrTracker
	}
	return score, nil
}

// UpdateParams preserves source publication order and deliberately keeps
// track_unsat disabled even when the parameter is true.
func (t *Tracker) UpdateParams(local *uparams.ParamsRef) (result error) {
	if err := t.open(); err != nil {
		return err
	}
	p, err := NewParams(t.context, t.globals, local)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, p.Close()) }()
	if t.walksat, err = p.Walksat(); err != nil {
		return err
	}
	if t.ucb, err = p.WalksatUCB(); err != nil {
		return err
	}
	if t.ucbConstant, err = p.WalksatUCBConstant(); err != nil {
		return err
	}
	if t.ucbInit, err = p.WalksatUCBInit(); err != nil {
		return err
	}
	if t.ucbForget, err = p.WalksatUCBForget(); err != nil {
		return err
	}
	if t.ucbNoise, err = p.WalksatUCBNoise(); err != nil {
		return err
	}
	if t.scaleUnsat, err = p.ScaleUnsat(); err != nil {
		return err
	}
	if t.pawsInit, err = p.PawsInit(); err != nil {
		return err
	}
	t.trackUnsat = false
	return nil
}

func (t *Tracker) AdaptTopSum(expr ast.ASTHandle, add, sub float64) error {
	if err := t.requireExpr(expr); err != nil {
		return err
	}
	weight, ok := t.weights[expr]
	if !ok {
		return ErrTracker
	}
	t.topSum += float64(weight) * (add - sub)
	return nil
}

func (t *Tracker) SetTopSum(value float64) error {
	if err := t.open(); err != nil {
		return err
	}
	t.topSum = value
	return nil
}
func (t *Tracker) TopSum() (float64, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	return t.topSum, nil
}

func (t *Tracker) IsSAT() (bool, error) {
	if err := t.open(); err != nil {
		return false, err
	}
	for h := range t.topExpressions {
		value, err := t.GetValue(h)
		if err != nil {
			return false, err
		}
		one, err := t.numbers.IsOne(value)
		if err != nil {
			return false, err
		}
		if !one {
			return false, nil
		}
	}
	return true, nil
}

func (t *Tracker) SetValue(h ast.ASTHandle, value *numeral.Int) error {
	score, err := t.existingScore(h)
	if err != nil {
		return err
	}
	if value == nil {
		return ErrTracker
	}
	return t.numbers.Copy(&score.value, value)
}

// GetValue returns the source-equivalent borrowed mutable numeral.
func (t *Tracker) GetValue(h ast.ASTHandle) (*numeral.Int, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	score := t.scores[h]
	if score == nil || score.destroyed {
		return nil, ErrTracker
	}
	return &score.value, nil
}

func (t *Tracker) SetDeclValue(decl ast.ASTHandle, value *numeral.Int) error {
	h, err := t.EntryPoint(decl)
	if err != nil {
		return err
	}
	return t.SetValue(h, value)
}
func (t *Tracker) DeclValue(decl ast.ASTHandle) (*numeral.Int, error) {
	h, err := t.EntryPoint(decl)
	if err != nil {
		return nil, err
	}
	return t.GetValue(h)
}

func (t *Tracker) SetScore(h ast.ASTHandle, value float64) error {
	score, err := t.existingScore(h)
	if err != nil {
		return err
	}
	score.score = value
	return nil
}
func (t *Tracker) Score(h ast.ASTHandle) (float64, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	score := t.scores[h]
	if score == nil {
		return 0, ErrTracker
	}
	return score.score, nil
}
func (t *Tracker) SetDeclScore(decl ast.ASTHandle, value float64) error {
	h, err := t.EntryPoint(decl)
	if err != nil {
		return err
	}
	return t.SetScore(h, value)
}
func (t *Tracker) DeclScore(decl ast.ASTHandle) (float64, error) {
	h, err := t.EntryPoint(decl)
	if err != nil {
		return 0, err
	}
	return t.Score(h)
}
func (t *Tracker) SetScorePrune(h ast.ASTHandle, value float64) error {
	score, err := t.existingScore(h)
	if err != nil {
		return err
	}
	score.scorePrune = value
	return nil
}
func (t *Tracker) ScorePrune(h ast.ASTHandle) (float64, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	score := t.scores[h]
	if score == nil {
		return 0, ErrTracker
	}
	return score.scorePrune, nil
}
func (t *Tracker) Occurrences(h ast.ASTHandle) (uint32, uint32, error) {
	if err := t.open(); err != nil {
		return 0, 0, err
	}
	score := t.scores[h]
	if score == nil {
		return 0, 0, ErrTracker
	}
	return score.hasPosOcc, score.hasNegOcc, nil
}
func (t *Tracker) setOccurrences(h ast.ASTHandle, positive, negative uint32) error {
	score, err := t.existingScore(h)
	if err != nil {
		return err
	}
	score.hasPosOcc, score.hasNegOcc = positive, negative
	return nil
}
func (t *Tracker) SetDistance(h ast.ASTHandle, value uint32) error {
	score, err := t.existingScore(h)
	if err != nil {
		return err
	}
	score.distance = value
	return nil
}
func (t *Tracker) Distance(h ast.ASTHandle) (uint32, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	score := t.scores[h]
	if score == nil {
		return 0, ErrTracker
	}
	return score.distance, nil
}

func (t *Tracker) setEntryPoint(decl, expr ast.ASTHandle) error {
	if err := t.open(); err != nil {
		return err
	}
	kind, err := t.manager.GetKind(decl)
	if err != nil || kind != ast.ASTFuncDecl {
		return ErrTracker
	}
	if err = t.requireExpr(expr); err != nil {
		return err
	}
	t.entryPoints[decl] = expr
	return nil
}
func (t *Tracker) EntryPoint(decl ast.ASTHandle) (ast.ASTHandle, error) {
	if err := t.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	h, ok := t.entryPoints[decl]
	if !ok {
		return ast.ASTHandle{}, ErrTracker
	}
	return h, nil
}
func (t *Tracker) HasUplinks(h ast.ASTHandle) (bool, error) {
	if err := t.requireExpr(h); err != nil {
		return false, err
	}
	_, ok := t.uplinks[h]
	return ok, nil
}
func (t *Tracker) Uplinks(h ast.ASTHandle) ([]ast.ASTHandle, error) {
	if err := t.requireExpr(h); err != nil {
		return nil, err
	}
	values, ok := t.uplinks[h]
	if !ok {
		return nil, ErrTracker
	}
	return values, nil
}
func (t *Tracker) addUplink(child, parent ast.ASTHandle) error {
	if err := t.requireExpr(child); err != nil {
		return err
	}
	if err := t.requireExpr(parent); err != nil {
		return err
	}
	t.uplinks[child] = append(t.uplinks[child], parent)
	return nil
}
func (t *Tracker) IsTopExpr(h ast.ASTHandle) (bool, error) {
	if err := t.requireExpr(h); err != nil {
		return false, err
	}
	_, ok := t.topExpressions[h]
	return ok, nil
}
func (t *Tracker) setTopExpr(h ast.ASTHandle, weight uint32) error {
	if _, err := t.trackExpr(h); err != nil {
		return err
	}
	t.topExpressions[h] = struct{}{}
	t.weights[h] = weight
	return nil
}

func (t *Tracker) ucbForgetExpressions(expressions []ast.ASTHandle) error {
	if err := t.open(); err != nil {
		return err
	}
	if t.ucbForget >= 1.0 {
		return nil
	}
	for _, h := range expressions {
		score := t.scores[h]
		if score == nil {
			return ErrTracker
		}
		old := score.touched
		fresh := uint32(float64(old-1)*t.ucbForget + 1)
		score.touched = fresh
		t.touched += fresh - old
	}
	return nil
}

// Close executes the source destructor body before value_score map cleanup.
// It is retryable at every numerical destructor boundary.
func (t *Tracker) Close() error {
	if t == nil || t.self != t {
		return ErrTracker
	}
	if t.closed {
		return nil
	}
	if t.numbers == nil || t.numbers.Closed() {
		return ErrTracker
	}
	locals := []*numeral.Int{&t.zero, &t.one, &t.two}
	for t.closePhase < len(locals) {
		if err := t.numbers.Del(locals[t.closePhase]); err != nil {
			return err
		}
		t.closePhase++
	}
	for t.closeScore < len(t.scoreOrder) {
		score := t.scores[t.scoreOrder[t.closeScore]]
		if score != nil && !score.destroyed {
			if err := t.numbers.Del(&score.value); err != nil {
				return err
			}
			score.destroyed = true
		}
		t.closeScore++
	}
	t.scores, t.uplinks, t.entryPoints, t.topExpressions, t.weights, t.constantSet = nil, nil, nil, nil, nil, nil
	t.scoreOrder = nil
	t.constants = nil
	t.closed = true
	return nil
}

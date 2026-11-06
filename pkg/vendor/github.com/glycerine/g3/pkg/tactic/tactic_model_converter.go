package tactic

// Source: src/tactic/model_converter.cpp:23-126,183-187;
// src/tactic/converter.h:26-66,96-124; src/util/ref.h:26-90.

import (
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

var ErrModelConverterOwner = errors.New("model converter: invalid, copied or retired owner")
var errTacticModelConverterGoal = errors.New("model converter: unreachable goal index or unindexed star application")

// Cold virtual-owner boundary for original model converters. These methods
// dispatch original converter operations; they never perform a solver check.
type ModelConverter interface {
	IncRef() error
	DecRef() error
	DecRefConsumed() (bool, error)
	Close() error
	ApplyModel(*ModelRef, uint32) error
	ApplyUnindexed(*ModelRef) error
	ApplyLabels(*ModelLabels, uint32) error
	Cancel() error
	Display(io.Writer) error
	Translate(*ast.ASTTranslation) (ModelConverter, error)
}

// Source owning ref assignment: retain incoming before releasing outgoing.
// A zero value is an empty reference. The borrowed AST manager outlives models.
type ModelRef struct {
	self    *ModelRef
	value   *model.ModelCore
	owned   bool
	cleanup *model.ModelCore // final DEC consumed; model destruction still pending
	pending []*model.ModelCore
	closed  bool
}

func (r *ModelRef) open() error {
	if r == nil || r.closed || (r.self != nil && r.self != r) {
		return ErrModelConverterOwner
	}
	return nil
}
func (r *ModelRef) Set(value *model.ModelCore) error {
	if e := r.open(); e != nil {
		return e
	}
	r.self = r
	if value != nil {
		if e := value.IncRef(); e != nil {
			return e
		}
	}
	if r.value != nil {
		if !r.owned {
			if value != nil {
				r.pending = append(r.pending, value)
			}
			return ErrModelConverterOwner
		}
		consumed, e := r.value.DecRefConsumed()
		if consumed {
			r.owned = false
			if e != nil {
				r.cleanup = r.value
			}
		}
		if e != nil || !consumed {
			if value != nil {
				r.pending = append(r.pending, value)
			}
			if e != nil {
				return e
			}
			return ErrModelConverterOwner
		}
	}
	r.value = value
	r.owned = value != nil
	return nil
}

// SetRef is the concrete ref<model> copy assignment used by model2mc.
// Set retains the incoming model before releasing the outgoing one, including
// when both references already point to the same model.
func (r *ModelRef) SetRef(source *ModelRef) error {
	if e := r.open(); e != nil {
		return e
	}
	if e := source.open(); e != nil {
		return e
	}
	return r.Set(source.value)
}

func (r *ModelRef) Get() (*model.ModelCore, error) {
	if e := r.open(); e != nil {
		return nil, e
	}
	if r.value != nil {
		if e := r.value.Check(); e != nil {
			return nil, e
		}
	}
	return r.value, nil
}
func (r *ModelRef) Close() error {
	if r == nil || (r.self != nil && r.self != r) {
		return ErrModelConverterOwner
	}
	if r.closed {
		return nil
	}
	r.self = r
	if r.cleanup != nil {
		if e := r.cleanup.Close(); e != nil {
			return e
		}
		r.cleanup = nil
	}
	if r.value != nil && r.owned {
		consumed, e := r.value.DecRefConsumed()
		if consumed {
			r.owned = false
			if e != nil {
				r.cleanup = r.value
				return e
			}
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ErrModelConverterOwner
		}
	}
	for len(r.pending) != 0 {
		last := len(r.pending) - 1
		h := r.pending[last]
		consumed, e := h.DecRefConsumed()
		if consumed {
			r.pending = r.pending[:last]
			if e != nil {
				r.cleanup = h
				return e
			}
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ErrModelConverterOwner
		}
	}
	r.value = nil
	r.closed = true
	return nil
}

// holds reports whether assignment acquired a model even if old-model
// destruction stopped before the new raw pointer was published.
func (r *ModelRef) holds(value *model.ModelCore) bool {
	if r == nil || value == nil {
		return false
	}
	if r.value == value && r.owned {
		return true
	}
	for _, pending := range r.pending {
		if pending == value {
			return true
		}
	}
	return false
}

// ModelLabels borrows symbol identities, which stay valid with the source
// symbol manager. Its typed slice is ordinary Go-managed storage.
type ModelLabels struct {
	Values []usymbol.Symbol
	Budget *arena.Budget
}

func (l *ModelLabels) Append(value usymbol.Symbol) error {
	if l == nil {
		return ErrModelConverterOwner
	}
	l.Values = append(l.Values, value)
	return nil
}
func (l *ModelLabels) Close() {
	if l == nil {
		return
	}
	clear(l.Values)
	l.Values = nil
}

type tacticConverterBase struct {
	self       *tacticConverterBase
	owner      ModelConverter
	budget     *arena.Budget
	ownerBytes uint64
	references uint32
	closed     bool
}

func (b *tacticConverterBase) init(owner ModelConverter, budget *arena.Budget, bytes uint64) error {
	if e := budget.Reserve(bytes); e != nil {
		return e
	}
	b.self = b
	b.owner = owner
	b.budget = budget
	b.ownerBytes = bytes
	return nil
}
func (b *tacticConverterBase) open() error {
	if b == nil || b.self != b || b.closed {
		return ErrModelConverterOwner
	}
	return nil
}
func (b *tacticConverterBase) IncRef() error {
	if e := b.open(); e != nil {
		return e
	}
	b.references++
	return nil
}
func (b *tacticConverterBase) DecRef() error {
	_, e := b.DecRefConsumed()
	return e
}
func (b *tacticConverterBase) DecRefConsumed() (bool, error) {
	if e := b.open(); e != nil {
		return false, e
	}
	if b.references == 0 {
		return false, ErrModelConverterOwner
	}
	b.references--
	if b.references == 0 {
		return true, b.owner.Close()
	}
	return true, nil
}

// A final decrement may consume the reference and still stop while the child
// closes. Keep that distinction so a parent resumes Close instead of DEC.
func closeTacticConverterRef(value ModelConverter, cleanupPending *bool) error {
	if *cleanupPending {
		return value.Close()
	}
	consumed, e := value.DecRefConsumed()
	if !consumed && e == nil {
		return ErrModelConverterOwner
	}
	if consumed && e != nil {
		*cleanupPending = true
	}
	return e
}
func (b *tacticConverterBase) release() {
	b.budget.Release(b.ownerBytes)
	b.ownerBytes = 0
	b.owner = nil
	b.closed = true
}

type tacticConcatModelConverter struct {
	tacticConverterBase
	// Concrete ref<model_converter> members from concat_converter.
	// Construction retains first then second; Close releases second then first.
	first, second               ModelConverter
	firstCleanup, secondCleanup bool
}

func ConcatModels(b *arena.Budget, first, second ModelConverter) (result ModelConverter, err error) {
	if first == nil {
		return second, nil
	}
	if second == nil {
		return first, nil
	}
	c := &tacticConcatModelConverter{}
	if err = c.init(c, b, uint64(reflect.TypeOf(*c).Size())); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, c.Close())
		}
	}()
	if err = first.IncRef(); err != nil {
		return nil, err
	}
	c.first = first
	if err = second.IncRef(); err != nil {
		return nil, err
	}
	c.second = second
	return c, nil
}
func (c *tacticConcatModelConverter) ApplyModel(m *ModelRef, index uint32) error {
	if e := c.open(); e != nil {
		return e
	}
	if e := m.open(); e != nil {
		return e
	}
	if e := c.second.ApplyModel(m, index); e != nil {
		return e
	}
	return c.first.ApplyModel(m, 0)
}
func (c *tacticConcatModelConverter) ApplyUnindexed(m *ModelRef) error {
	if e := c.open(); e != nil {
		return e
	}
	if e := m.open(); e != nil {
		return e
	}
	if e := c.second.ApplyUnindexed(m); e != nil {
		return e
	}
	return c.first.ApplyUnindexed(m)
}
func (c *tacticConcatModelConverter) ApplyLabels(labels *ModelLabels, index uint32) error {
	if e := c.open(); e != nil {
		return e
	}
	if labels == nil {
		return ErrModelConverterOwner
	}
	if e := c.second.ApplyLabels(labels, index); e != nil {
		return e
	}
	return c.first.ApplyLabels(labels, 0)
}
func (c *tacticConcatModelConverter) Cancel() error {
	if e := c.open(); e != nil {
		return e
	}
	if e := c.second.Cancel(); e != nil {
		return e
	}
	return c.first.Cancel()
}
func (c *tacticConcatModelConverter) Display(out io.Writer) error {
	if e := c.open(); e != nil {
		return e
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if _, e := io.WriteString(out, "(concat-model-converter\n"); e != nil {
		return e
	}
	if e := c.first.Display(out); e != nil {
		return e
	}
	if e := c.second.Display(out); e != nil {
		return e
	}
	_, e := io.WriteString(out, ")\n")
	return e
}
func (c *tacticConcatModelConverter) Translate(translator *ast.ASTTranslation) (_ ModelConverter, err error) {
	if e := c.open(); e != nil {
		return nil, e
	}
	if translator == nil || translator.To() == nil {
		return nil, ast.ErrASTHandle
	}
	first, err := c.first.Translate(translator)
	if err != nil {
		return nil, err
	}
	second, err := c.second.Translate(translator)
	if err != nil {
		return nil, errors.Join(err, first.Close())
	}
	result, err := ConcatModels(translator.To().Budget(), first, second)
	if err != nil {
		return nil, errors.Join(err, first.Close(), second.Close())
	}
	return result, nil
}
func (c *tacticConcatModelConverter) Close() error {
	if c == nil || c.self != &c.tacticConverterBase {
		return ErrModelConverterOwner
	}
	if c.closed {
		return nil
	}
	// Source reverse member destruction: second reference, then first.
	if c.second != nil {
		if e := closeTacticConverterRef(c.second, &c.secondCleanup); e != nil {
			return e
		}
		c.second = nil
		c.secondCleanup = false
	}
	if c.first != nil {
		if e := closeTacticConverterRef(c.first, &c.firstCleanup); e != nil {
			return e
		}
		c.first = nil
		c.firstCleanup = false
	}
	c.release()
	return nil
}

type tacticConcatStarModelConverter struct {
	tacticConverterBase
	first ModelConverter
	// Cold references to source virtual converter owners; no AST or solver
	// records are boxed. Scalar subgoal sizes retain their own source vector.
	seconds                                     []ModelConverter
	sizes                                       []uint32
	pending                                     ModelConverter
	closeIndex                                  uint32
	firstCleanup, secondCleanup, pendingCleanup bool
}

func (c *tacticConcatStarModelConverter) pushSecond(value ModelConverter) error {
	c.seconds = append(c.seconds, value)
	return nil
}
func (c *tacticConcatStarModelConverter) pushSize(size uint32) error {
	c.sizes = append(c.sizes, size)
	return nil
}
func concatTacticModelSubgoals(b *arena.Budget, first ModelConverter, seconds []ModelConverter, sizes []uint32) (result ModelConverter, err error) {
	if len(seconds) == 0 && tacticSourceDebug {
		return nil, errTacticModelConverterGoal
	}
	if len(seconds) == 1 {
		return ConcatModels(b, first, seconds[0])
	}
	allNull := true
	for _, second := range seconds {
		if second != nil {
			allNull = false
			break
		}
	}
	if allNull {
		return first, nil
	}
	// The original shortcuts never read szs. A table is needed only when the
	// actual star constructor will copy one size for each second converter.
	if len(sizes) < len(seconds) {
		return nil, errTacticModelConverterGoal
	}
	c := &tacticConcatStarModelConverter{}
	if err = c.init(c, b, uint64(reflect.TypeOf(*c).Size())); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, c.Close())
		}
	}()
	if first != nil {
		if err = first.IncRef(); err != nil {
			return nil, err
		}
		c.first = first
	}
	for i, second := range seconds {
		if second != nil {
			if err = second.IncRef(); err != nil {
				return nil, err
			}
			c.pending = second
		}
		if err = c.pushSecond(second); err != nil {
			return nil, err
		}
		c.pending = nil
		if err = c.pushSize(sizes[i]); err != nil {
			return nil, err
		}
	}
	return c, nil
}
func (c *tacticConcatStarModelConverter) branch(index uint32) (uint32, uint32, error) {
	if e := c.open(); e != nil {
		return 0, 0, e
	}
	for i, size := range c.sizes {
		if index < size {
			return uint32(i), index, nil
		}
		index -= size
	}
	return 0, 0, errTacticModelConverterGoal
}
func (c *tacticConcatStarModelConverter) ApplyModel(m *ModelRef, index uint32) error {
	if e := m.open(); e != nil {
		return e
	}
	i, local, e := c.branch(index)
	if e != nil {
		return e
	}
	if c.seconds[i] != nil {
		if e = c.seconds[i].ApplyModel(m, local); e != nil {
			return e
		}
	}
	if c.first != nil {
		return c.first.ApplyModel(m, i)
	}
	return nil
}
func (c *tacticConcatStarModelConverter) ApplyUnindexed(*ModelRef) error {
	if e := c.open(); e != nil {
		return e
	}
	return errTacticModelConverterGoal
}
func (c *tacticConcatStarModelConverter) ApplyLabels(labels *ModelLabels, index uint32) error {
	if labels == nil {
		return ErrModelConverterOwner
	}
	i, local, e := c.branch(index)
	if e != nil {
		return e
	}
	if c.seconds[i] != nil {
		if e = c.seconds[i].ApplyLabels(labels, local); e != nil {
			return e
		}
	}
	if c.first != nil {
		return c.first.ApplyLabels(labels, i)
	}
	return nil
}
func (c *tacticConcatStarModelConverter) Cancel() error {
	if e := c.open(); e != nil {
		return e
	}
	if c.first != nil {
		if e := c.first.Cancel(); e != nil {
			return e
		}
	}
	for _, second := range c.seconds {
		if second != nil {
			if e := second.Cancel(); e != nil {
				return e
			}
		}
	}
	return nil
}
func (c *tacticConcatStarModelConverter) Display(out io.Writer) error {
	if e := c.open(); e != nil {
		return e
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if _, e := io.WriteString(out, "(concat-star-model-converter\n"); e != nil {
		return e
	}
	if c.first != nil {
		if e := c.first.Display(out); e != nil {
			return e
		}
	}
	if _, e := io.WriteString(out, "(\n"); e != nil {
		return e
	}
	for _, second := range c.seconds {
		if second != nil {
			if e := second.Display(out); e != nil {
				return e
			}
		}
	}
	_, e := io.WriteString(out, "))\n")
	return e
}
func (c *tacticConcatStarModelConverter) Translate(translator *ast.ASTTranslation) (_ ModelConverter, err error) {
	if e := c.open(); e != nil {
		return nil, e
	}
	if translator == nil || translator.To() == nil {
		return nil, ast.ErrASTHandle
	}
	var first ModelConverter
	if c.first != nil {
		first, err = c.first.Translate(translator)
		if err != nil {
			return nil, err
		}
	}
	seconds := make([]ModelConverter, len(c.seconds))
	cleanup := func() error {
		var result error
		for _, value := range seconds {
			if value != nil {
				result = errors.Join(result, value.Close())
			}
		}
		if first != nil {
			result = errors.Join(result, first.Close())
		}
		return result
	}
	for i, second := range c.seconds {
		if second != nil {
			seconds[i], err = second.Translate(translator)
			if err != nil {
				return nil, errors.Join(err, cleanup())
			}
		}
	}
	result, err := concatTacticModelSubgoals(translator.To().Budget(), first, seconds, c.sizes)
	if err != nil {
		return nil, errors.Join(err, cleanup())
	}
	return result, nil
}
func (c *tacticConcatStarModelConverter) Close() error {
	if c == nil || c.self != &c.tacticConverterBase {
		return ErrModelConverterOwner
	}
	if c.closed {
		return nil
	}
	for uint64(c.closeIndex) < uint64(len(c.seconds)) {
		second := c.seconds[c.closeIndex]
		if second != nil {
			if e := closeTacticConverterRef(second, &c.secondCleanup); e != nil {
				return e
			}
			c.seconds[c.closeIndex] = nil
			c.secondCleanup = false
		}
		c.closeIndex++
	}
	if c.pending != nil {
		if e := closeTacticConverterRef(c.pending, &c.pendingCleanup); e != nil {
			return e
		}
		c.pending = nil
		c.pendingCleanup = false
	}
	c.sizes = nil
	c.seconds = nil
	if c.first != nil {
		if e := closeTacticConverterRef(c.first, &c.firstCleanup); e != nil {
			return e
		}
		c.first = nil
		c.firstCleanup = false
	}
	c.release()
	return nil
}

// Source apply(model_converter_ref&,model_ref&,unsigned) null shortcut.
func applyTacticModelConverter(c ModelConverter, m *ModelRef, index uint32) error {
	if c == nil {
		return nil
	}
	return c.ApplyModel(m, index)
}

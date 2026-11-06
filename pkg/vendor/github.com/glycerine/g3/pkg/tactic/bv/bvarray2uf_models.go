package bv

import (
	"errors"
	"github.com/glycerine/g3/pkg/tactic"
)

// prepareModels keeps the two mutable converters alive while the rewriter
// discovers eliminated arrays.  Publishing them before rewriting would leave
// the rewriter with pointers to converters whose initial ownership was already
// released.
func (t *BVArray2UFTactic) prepareModels(enabled bool) (err error) {
	t.rewriter.extension = nil
	t.rewriter.filter = nil
	if !enabled {
		return nil
	}
	extension, e := tactic.NewExtensionModel(t.manager, t.services)
	if e != nil {
		return e
	}
	if e = extension.IncRef(); e != nil {
		return errors.Join(e, extension.Close())
	}
	filter, e := tactic.NewFilterModel(t.manager)
	if e != nil {
		return errors.Join(e, extension.DecRef())
	}
	if e = filter.IncRef(); e != nil {
		return errors.Join(e, filter.Close(), extension.DecRef())
	}
	t.rewriter.extension = extension
	t.rewriter.filter = filter
	return nil
}

func (t *BVArray2UFTactic) discardPreparedModels() (err error) {
	filter, extension := t.rewriter.filter, t.rewriter.extension
	t.rewriter.filter = nil
	t.rewriter.extension = nil
	if filter != nil {
		err = errors.Join(err, filter.DecRef())
	}
	if extension != nil {
		err = errors.Join(err, extension.DecRef())
	}
	return err
}

func (t *BVArray2UFTactic) publishModels(out *tactic.TacticResult) (err error) {
	extension, filter := t.rewriter.extension, t.rewriter.filter
	if extension == nil && filter == nil {
		return nil
	}
	converter, e := tactic.ConcatModels(t.manager.Budget(), extension, filter)
	if e != nil {
		return errors.Join(e, t.discardPreparedModels())
	}
	if e = out.SetModelConverter(converter); e != nil {
		return errors.Join(e, converter.Close(), t.discardPreparedModels())
	}
	return t.discardPreparedModels()
}

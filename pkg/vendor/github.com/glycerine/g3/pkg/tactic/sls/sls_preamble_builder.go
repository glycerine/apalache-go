package sls

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// slsPreambleBuilder keeps only the unparented tactic roots. Wrapping a child
// transfers its reference into the wrapper, so constructor failure can retire
// each remaining root exactly once.
type slsPreambleBuilder struct {
	manager *ast.ASTManager
	context *uparams.ParamsContext
	roots   map[tactic.Tactic]struct{}
	params  []*uparams.ParamsRef
	err     error
}

func newSLSPreambleBuilder(manager *ast.ASTManager, context *uparams.ParamsContext) *slsPreambleBuilder {
	return &slsPreambleBuilder{manager: manager, context: context, roots: make(map[tactic.Tactic]struct{})}
}

func (b *slsPreambleBuilder) own(value tactic.Tactic, err error) tactic.Tactic {
	if b.err != nil {
		return nil
	}
	if err != nil || value == nil {
		if err == nil {
			err = ast.ErrASTHandle
		}
		b.err = errors.Join(err, func() error {
			if value != nil {
				return retireUnownedTactic(value)
			}
			return nil
		}())
		return nil
	}
	b.roots[value] = struct{}{}
	return value
}

func (b *slsPreambleBuilder) parameter(copyFrom *uparams.ParamsRef) *uparams.ParamsRef {
	if b.err != nil {
		return nil
	}
	value, err := b.context.NewValue()
	if err == nil && copyFrom != nil {
		err = b.context.Assign(&value, copyFrom)
	}
	if err != nil {
		b.err = errors.Join(b.err, err)
		return nil
	}
	b.params = append(b.params, &value)
	return &value
}

func (b *slsPreambleBuilder) boolean(p *uparams.ParamsRef, name string, value bool) {
	if b.err == nil {
		b.err = b.context.SetBoolName(p, &name, value)
	}
}

func (b *slsPreambleBuilder) unsigned(p *uparams.ParamsRef, name string, value uint32) {
	if b.err == nil {
		b.err = b.context.SetUIntName(p, &name, value)
	}
}

func (b *slsPreambleBuilder) using(child tactic.Tactic, p *uparams.ParamsRef) tactic.Tactic {
	if b.err != nil || child == nil || p == nil {
		return nil
	}
	wrapped, err := tactic.NewUsingParamsTactic(b.context, child, p)
	if err != nil {
		b.err = err
		return nil
	}
	delete(b.roots, child)
	b.roots[wrapped] = struct{}{}
	return wrapped
}

func (b *slsPreambleBuilder) then(left, right tactic.Tactic) tactic.Tactic {
	if b.err != nil || left == nil || right == nil {
		return nil
	}
	combined, err := tactic.NewAndThenTactic(left, right)
	if err != nil {
		b.err = err
		return nil
	}
	delete(b.roots, left)
	delete(b.roots, right)
	b.roots[combined] = struct{}{}
	return combined
}

func (b *slsPreambleBuilder) sequence(values ...tactic.Tactic) tactic.Tactic {
	if len(values) == 0 {
		b.err = ast.ErrASTDomain
		return nil
	}
	result := values[0]
	for _, value := range values[1:] {
		result = b.then(result, value)
	}
	return result
}

func (b *slsPreambleBuilder) finish(result tactic.Tactic) (tactic.Tactic, error) {
	if b.err == nil && result != nil {
		delete(b.roots, result)
	}
	cleanup := b.close()
	if b.err != nil || cleanup != nil {
		return nil, errors.Join(b.err, cleanup)
	}
	return result, nil
}

func (b *slsPreambleBuilder) close() (result error) {
	for root := range b.roots {
		result = errors.Join(result, retireUnownedTactic(root))
		delete(b.roots, root)
	}
	for i := len(b.params); i > 0; i-- {
		result = errors.Join(result, b.context.Del(b.params[i-1]))
	}
	b.params = nil
	return result
}

package rel

// Source: src/api/api_datalog.cpp, fixedpoint_context callback trail.

import "github.com/glycerine/g3/pkg/ast"

func (p *ExternalRelationPlugin) hold(value ast.ASTHandle) error {
	if value == (ast.ASTHandle{}) {
		return nil
	}
	ref, err := p.ast.NewRef(value)
	if err != nil {
		return err
	}
	p.trail = append(p.trail, ref)
	return nil
}

func (p *ExternalRelationPlugin) reduceAssign(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
	if p.callbacks.ReduceAssign == nil {
		return nil
	}
	if err := p.hold(declaration); err != nil {
		return err
	}
	for _, value := range arguments {
		if err := p.hold(value); err != nil {
			return err
		}
	}
	return p.callbacks.ReduceAssign(declaration, arguments, outputs)
}

func (p *ExternalRelationPlugin) reduce(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
	if p.callbacks.Reduce != nil {
		result, err := p.callbacks.Reduce(declaration, arguments)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if err = p.hold(declaration); err != nil {
			return ast.ASTHandle{}, err
		}
		for _, value := range arguments {
			if err = p.hold(value); err != nil {
				return ast.ASTHandle{}, err
			}
		}
		if err = p.hold(result); err != nil {
			return ast.ASTHandle{}, err
		}
		if result != (ast.ASTHandle{}) {
			return result, nil
		}
	}
	return p.ast.MkApp(declaration, arguments)
}

// Close releases callback trail references after all relations from the plugin
// have been closed. It is safe to retry after a release error.
func (p *ExternalRelationPlugin) Close() error {
	if p == nil || p.closed {
		return nil
	}
	if p.ast == nil {
		return ErrTableRelation
	}
	for predicate, relation := range p.predicates {
		if err := relation.Close(); err != nil {
			return err
		}
		consumed, err := p.ast.DecRefConsumed(predicate)
		if consumed {
			delete(p.predicates, predicate)
		}
		if err != nil {
			return err
		}
	}
	if p.users != 0 {
		return ErrTableRelation
	}
	for len(p.trail) != 0 {
		last := len(p.trail) - 1
		err := p.ast.DelRef(&p.trail[last])
		if p.trail[last] == (ast.ASTRef{}) {
			p.trail = p.trail[:last]
		}
		if err != nil {
			return err
		}
	}
	p.closed, p.manager, p.ast, p.predicates = true, nil, nil, nil
	p.callbacks = ExternalRelationCallbacks{}
	return nil
}

package smtcore

// Source: src/smt/smt_model_checker.cpp add_instance, restart_eh,
// assert_new_instances and reset_new_instances.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type smtModelCheckerInstance struct {
	quantifier ast.ASTHandle
	bindings   []ast.ASTHandle
	generation uint32
	pins       []ast.ASTRef
}

func (c *SMTModelChecker) enqueueCounterexample(quantifier ast.ASTHandle, values []ast.ASTHandle, fromInverse bool, generation uint32) (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	count, err := c.manager.GetNumDeclarations(quantifier)
	if err != nil {
		return false, err
	}
	if uint32(len(values)) < count {
		return false, ast.ErrASTDomain
	}
	bindings := append([]ast.ASTHandle(nil), values[:count]...)
	for i, value := range bindings {
		if term := c.valueTerms[value]; !fromInverse && !term.IsNull() {
			value = term
		}
		private, err := containsPrivateModelTerm(c.manager, value, true)
		if err != nil {
			return false, err
		}
		if private {
			c.exhausted = true
			return false, nil
		}
		kind, err := c.manager.GetKind(value)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp {
			return false, ast.ErrASTDomain
		}
		bindings[i] = value
	}
	quantifierGeneration, err := c.registry.Generation(quantifier)
	if err != nil {
		return false, err
	}
	if quantifierGeneration > generation {
		generation = quantifierGeneration
	}
	return c.pinInstance(quantifier, bindings, generation)
}

// The recursive-definition consumer supplies ground call arguments directly;
// the auxiliary-model consumer has already recovered and validated its terms.
func (c *SMTModelChecker) pinInstance(quantifier ast.ASTHandle, bindings []ast.ASTHandle, generation uint32) (bool, error) {
	instance := smtModelCheckerInstance{quantifier: quantifier, bindings: append([]ast.ASTHandle(nil), bindings...), generation: generation}
	// Pin all bindings before the quantifier, as the source owner does. Publish
	// only a complete instance; failed reference construction releases its prefix.
	for _, expression := range append(append([]ast.ASTHandle(nil), bindings...), quantifier) {
		ref, err := c.manager.NewRef(expression)
		if err != nil {
			return false, errors.Join(err, c.releaseInstancePins(&instance))
		}
		instance.pins = append(instance.pins, ref)
	}
	c.newInstances = append(c.newInstances, instance)
	return true, nil
}

func (c *SMTModelChecker) releaseInstancePins(instance *smtModelCheckerInstance) (result error) {
	for i := range instance.pins {
		result = errors.Join(result, c.manager.DelRef(&instance.pins[i]))
	}
	instance.pins = nil
	return result
}

func (c *SMTModelChecker) HasNewInstances() bool {
	return c != nil && !c.closed && len(c.newInstances) != 0
}

func (c *SMTModelChecker) ResetNewInstances() (result error) {
	if err := c.open(); err != nil {
		return err
	}
	for i := range c.newInstances {
		result = errors.Join(result, c.releaseInstancePins(&c.newInstances[i]))
	}
	c.newInstances = nil
	return result
}

// Restart publishes pinned instances after the context has returned to its
// search base. Quantifiers removed by backtracking are skipped, and bindings
// whose enodes disappeared are internalized again at the captured generation.
func (c *SMTModelChecker) Restart() (result error) {
	if err := c.open(); err != nil {
		return err
	}
	if err := c.reportRestart(); err != nil {
		return err
	}
	previousGeneration := c.state.generation
	defer func() { c.state.generation = previousGeneration }()
	for _, instance := range c.newInstances {
		if _, present := c.state.BoolVar(instance.quantifier); !present {
			continue
		}
		if err := c.state.SetGeneration(instance.generation); err != nil {
			return err
		}
		bindings := make([]*SMTEnode, len(instance.bindings))
		for i, expression := range instance.bindings {
			if c.state.context.enodes[expression] == nil {
				if err := c.state.Internalize(expression, false); err != nil {
					return err
				}
			}
			node := c.state.context.enodes[expression]
			if node == nil {
				return ast.ErrASTDomain
			}
			bindings[i] = node
		}
		if _, err := c.registry.AddInstance(instance.quantifier, ast.ASTHandle{}, bindings, instance.generation, instance.generation, instance.generation); err != nil {
			return err
		}
	}
	return c.ResetNewInstances()
}

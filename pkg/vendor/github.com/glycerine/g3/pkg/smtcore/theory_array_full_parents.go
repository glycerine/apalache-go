package smtcore

// Source: theory_array_full mk_var, parent maps, constructor merges,
// default initialization and delayed select-map propagation.

import "github.com/glycerine/g3/pkg/ast"

func (t *SMTArrayTheory) initializeFullArrayVariable(node *SMTEnode, data *smtArrayVarData, op int32) error {
	if op != ast.ASTArrayOpConst && op != ast.ASTArrayOpMap && op != ast.ASTArrayOpAsArray {
		return nil
	}
	if _, err := t.instantiateConstructorDefault(node.owner); err != nil {
		return err
	}
	switch op {
	case ast.ASTArrayOpConst:
		data.consts = append(data.consts, node)
	case ast.ASTArrayOpMap:
		data.maps = append(data.maps, node)
	case ast.ASTArrayOpAsArray:
		data.asArrays = append(data.asArrays, node)
	default:
		return nil
	}
	return nil
}

func (t *SMTArrayTheory) markArrayEqualityRelevant(literal SMTLiteral) error {
	if literal.Var() < SMTFirstBoolVar {
		return nil
	}
	s, err := t.State()
	if err != nil {
		return err
	}
	return s.markRelevant(s.boolExpr[literal.Var()])
}

func (t *SMTArrayTheory) addParentMap(variable SMTTheoryVar, mapped *SMTEnode) error {
	if variable < 0 || int(variable) >= len(t.arrayData) {
		return ast.ErrASTDomain
	}
	if mapped == nil {
		return ast.ErrASTDomain
	}
	op, array, err := t.arrayOperation(mapped.owner)
	if err != nil || !array || op != ast.ASTArrayOpMap {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	data := t.arrayData[t.arrayUnion.Find(uint32(variable))]
	data.parentMaps = append(data.parentMaps, mapped)
	return t.arrayTrail.PushUndo(func() { data.parentMaps = data.parentMaps[:len(data.parentMaps)-1] })
}

func (t *SMTArrayTheory) setMapInputsUpward(mapped *SMTEnode) error {
	if mapped == nil {
		return ast.ErrASTDomain
	}
	op, array, err := t.arrayOperation(mapped.owner)
	if err != nil || !array || op != ast.ASTArrayOpMap {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	if err := t.traceFullArrayExpression("set_prop_upward", 143, mapped.owner); err != nil {
		return err
	}
	for _, input := range mapped.arguments {
		variable := input.TheoryVar(t.FamilyID())
		if variable < 0 {
			return ast.ErrASTDomain
		}
		if err := t.setArrayUpward(uint32(variable)); err != nil {
			return err
		}
	}
	return nil
}

func (t *SMTArrayTheory) instantiateMapRead(observer, mapped *SMTEnode) (bool, error) {
	return t.instantiateConstructorRead(observer.owner, mapped.owner)
}

func (t *SMTArrayTheory) mergeFullArrayClasses(root uint32, second *smtArrayVarData) error {
	first := t.arrayData[root]
	for _, mapped := range second.maps {
		if err := t.setArrayUpward(root); err != nil {
			return err
		}
		first.maps = append(first.maps, mapped)
		if err := t.arrayTrail.PushUndo(func() { first.maps = first.maps[:len(first.maps)-1] }); err != nil {
			return err
		}
		for _, read := range first.parentSelects {
			if _, err := t.instantiateMapRead(read, mapped); err != nil {
				return err
			}
		}
		if err := t.setMapInputsUpward(mapped); err != nil {
			return err
		}
	}
	for _, mapped := range second.parentMaps {
		if err := t.addParentMap(SMTTheoryVar(root), mapped); err != nil {
			return err
		}
	}
	for _, node := range second.consts {
		if err := t.setArrayUpward(root); err != nil {
			return err
		}
		first.consts = append(first.consts, node)
		if err := t.arrayTrail.PushUndo(func() { first.consts = first.consts[:len(first.consts)-1] }); err != nil {
			return err
		}
		if _, err := t.instantiateConstructorDefault(node.owner); err != nil {
			return err
		}
		for _, read := range first.parentSelects {
			if err := t.assertSelectAxiom(read.owner, node.owner); err != nil {
				return err
			}
		}
	}
	for _, node := range second.asArrays {
		if err := t.setArrayUpward(root); err != nil {
			return err
		}
		first.asArrays = append(first.asArrays, node)
		if err := t.arrayTrail.PushUndo(func() { first.asArrays = first.asArrays[:len(first.asArrays)-1] }); err != nil {
			return err
		}
		if _, err := t.instantiateConstructorDefault(node.owner); err != nil {
			return err
		}
		for _, read := range first.parentSelects {
			if err := t.assertSelectAxiom(read.owner, node.owner); err != nil {
				return err
			}
		}
	}
	return nil
}

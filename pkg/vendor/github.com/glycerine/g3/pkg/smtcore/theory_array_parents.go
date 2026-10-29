package smtcore

// Source: theory_array var_data, private union-find, parent registration,
// merge_eh, set_prop_upward, relevant_eh and assert_delayed_axioms.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/backtrack"
)

type smtArrayVarData struct {
	stores, parentStores, parentSelects []*SMTEnode
	maps, parentMaps, consts, asArrays  []*SMTEnode
	propUpward                          bool
}

type smtArrayUnionContext struct{ theory *SMTArrayTheory }

func (c *smtArrayUnionContext) GetTrailStack() *backtrack.TrailStack      { return c.theory.arrayTrail }
func (*smtArrayUnionContext) UnmergeEH(uint32, uint32)                    {}
func (*smtArrayUnionContext) AfterMergeEH(uint32, uint32, uint32, uint32) {}
func (c *smtArrayUnionContext) MergeEH(root, child, _, _ uint32) {
	if c.theory.arrayMergeError == nil {
		c.theory.arrayMergeError = c.theory.mergeArrayClasses(root, child)
	}
}

func (t *SMTArrayTheory) Setup() error {
	if t.schedulingReady {
		return nil
	}
	s, err := t.State()
	if err != nil {
		return err
	}
	if s.relevancy == nil || !s.relevancy.enabled {
		t.arrayConfig.Laziness = 0
		if t.parameterOwner != nil {
			t.parameterOwner.state.Laziness = 0
		}
	}
	t.schedulingReady = true
	return nil
}

func (t *SMTArrayTheory) CreateVariable(node *SMTEnode) (SMTTheoryVar, error) {
	variable, err := t.SMTTheoryBase.CreateVariable(node)
	if err != nil {
		return variable, err
	}
	unionVariable := t.arrayUnion.MkVar()
	if err = arraySourceVerify(73, "r == static_cast<theory_var>(m_find.mk_var())", func() bool { return unionVariable == uint32(variable) }); err != nil {
		return SMTNullTheoryVar, err
	}
	if err = arraySourceAssert(74, "r == static_cast<int>(m_var_data.size())", func() bool { return int(variable) == len(t.arrayData) }); err != nil {
		return SMTNullTheoryVar, err
	}
	data := &smtArrayVarData{}
	t.arrayData = append(t.arrayData, data)
	sort, err := t.state.manager.GetSort(node.owner)
	if err != nil {
		return variable, err
	}
	family, err := t.state.manager.GetFamilyID(sort)
	if err != nil {
		return variable, err
	}
	if family == t.FamilyID() {
		if err = t.registerArraySort(sort); err != nil {
			return variable, err
		}
	}
	op, array, err := t.arrayOperation(node.owner)
	if err != nil {
		return variable, err
	}
	if err = t.emitSimpleArrayTrace("array", "mk_var", 77, nil, func(out io.Writer) error {
		if err := displayArrayTraceExpression(t.state.manager, out, node.owner); err != nil {
			return err
		}
		_, err := fmt.Fprintf(out, "\nis_array: %t, is_select: %t, is_store: %t\n", family == t.FamilyID(), array && op == ast.ASTArrayOpSelect, array && op == ast.ASTArrayOpStore)
		return err
	}); err != nil {
		return variable, err
	}
	if array && op == ast.ASTArrayOpStore {
		data.stores = append(data.stores, node)
		if err = t.assertStoreAxiom(node.owner); err != nil {
			return variable, err
		}
	}
	if !t.simple && array {
		if err := t.initializeFullArrayVariable(node, data, op); err != nil {
			return variable, err
		}
	}
	return variable, nil
}

func (t *SMTArrayTheory) addParentSelect(variable SMTTheoryVar, read *SMTEnode) error {
	if read == nil {
		return ast.ErrASTDomain
	}
	op, array, err := t.arrayOperation(read.owner)
	if err != nil {
		return err
	}
	if err = arraySourceAssert(94, "is_select(s)", func() bool { return array && op == ast.ASTArrayOpSelect }); err != nil {
		return err
	}
	if !array || op != ast.ASTArrayOpSelect || variable < 0 || int(variable) >= len(t.arrayData) {
		return ast.ErrASTDomain
	}
	root := t.arrayUnion.Find(uint32(variable))
	if err := t.emitSimpleArrayTrace("array", "add_parent_select", 98, nil, func(out io.Writer) error {
		if err := displayArrayTraceExpression(t.state.manager, out, read.owner); err != nil {
			return err
		}
		if _, err := io.WriteString(out, " "); err != nil {
			return err
		}
		if err := displayArrayTraceExpression(t.state.manager, out, t.variables[root].owner); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	}); err != nil {
		return err
	}
	if !t.simple {
		if err := t.emitFullArrayTrace("array", "add_parent_select", 394, nil, func(out io.Writer) error {
			if _, err := fmt.Fprintf(out, "%d select parent: ", variable); err != nil {
				return err
			}
			if err := displayArrayTraceExpression(t.state.manager, out, read.owner); err != nil {
				return err
			}
			if _, err := io.WriteString(out, "\n"); err != nil {
				return err
			}
			return t.DisplayVariable(out, SMTTheoryVar(root))
		}); err != nil {
			return err
		}
	}
	data := t.arrayData[root]
	data.parentSelects = append(data.parentSelects, read)
	if err := t.arrayTrail.PushUndo(func() { data.parentSelects = data.parentSelects[:len(data.parentSelects)-1] }); err != nil {
		return err
	}
	for _, store := range data.stores {
		if err := t.enqueueStoreReadAxiom(read.owner, store.owner); err != nil {
			return err
		}
	}
	if !t.simple {
		for _, node := range data.consts {
			if err := t.assertSelectAxiom(read.owner, node.owner); err != nil {
				return err
			}
		}
		for _, node := range data.maps {
			if _, err := t.instantiateMapRead(read, node); err != nil {
				return err
			}
		}
		// Native add_parent_select visits consts and maps. As-array reads
		// are instantiated by add_as_array when its constructor joins a
		// class with existing reads, retaining the source incompleteness.
	}
	return nil
}

func (t *SMTArrayTheory) addParentStore(variable SMTTheoryVar, store *SMTEnode) error {
	if store == nil {
		return ast.ErrASTDomain
	}
	op, array, err := t.arrayOperation(store.owner)
	if err != nil {
		return err
	}
	if err = arraySourceAssert(116, "is_store(s)", func() bool { return array && op == ast.ASTArrayOpStore }); err != nil {
		return err
	}
	if !array || op != ast.ASTArrayOpStore || variable < 0 || int(variable) >= len(t.arrayData) {
		return ast.ErrASTDomain
	}
	data := t.arrayData[t.arrayUnion.Find(uint32(variable))]
	data.parentStores = append(data.parentStores, store)
	return t.arrayTrail.PushUndo(func() { data.parentStores = data.parentStores[:len(data.parentStores)-1] })
}

func (t *SMTArrayTheory) setStoreBaseUpward(store *SMTEnode) error {
	if !t.simple {
		if err := t.traceFullArrayExpression("set_prop_upward", 143, store.owner); err != nil {
			return err
		}
	}
	variable := store.arguments[0].TheoryVar(t.FamilyID())
	if variable < 0 {
		return ast.ErrASTDomain
	}
	return t.setArrayUpward(uint32(variable))
}

func (t *SMTArrayTheory) setArrayUpward(variable uint32) error {
	if t.arrayConfig.Weak {
		return nil
	}
	root := t.arrayUnion.Find(variable)
	data := t.arrayData[root]
	if data.propUpward {
		return nil
	}
	data.propUpward = true
	if err := t.arrayTrail.PushUndo(func() { data.propUpward = false }); err != nil {
		return err
	}
	if err := t.emitSimpleArrayTrace("array", "set_prop_upward", 146, nil, func(out io.Writer) error {
		_, err := fmt.Fprintf(out, "#%d\n", root)
		return err
	}); err != nil {
		return err
	}
	if !t.simple {
		if err := t.emitFullArrayTrace("array", "set_prop_upward", 115, nil, func(out io.Writer) error {
			_, err := fmt.Fprintf(out, "#%d\n", root)
			return err
		}); err != nil {
			return err
		}
	}
	for _, store := range data.stores {
		if err := t.setStoreBaseUpward(store); err != nil {
			return err
		}
	}
	if !t.simple {
		for _, mapped := range data.maps {
			if err := t.setMapInputsUpward(mapped); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *SMTArrayTheory) addArrayStore(variable uint32, store *SMTEnode) error {
	if store == nil {
		return ast.ErrASTDomain
	}
	op, array, err := t.arrayOperation(store.owner)
	if err != nil {
		return err
	}
	if err = arraySourceAssert(182, "is_store(s)", func() bool { return array && op == ast.ASTArrayOpStore }); err != nil {
		return err
	}
	if !array || op != ast.ASTArrayOpStore || int(variable) >= len(t.arrayData) {
		return ast.ErrASTDomain
	}
	data := t.arrayData[t.arrayUnion.Find(variable)]
	// Retained AlwaysPropUpward is true: add_store marks the class before
	// inserting the new constructor, then propagates through its new input.
	if err := t.setArrayUpward(variable); err != nil {
		return err
	}
	data.stores = append(data.stores, store)
	if err := t.arrayTrail.PushUndo(func() { data.stores = data.stores[:len(data.stores)-1] }); err != nil {
		return err
	}
	for _, read := range data.parentSelects {
		readOp, readArray, readErr := t.arrayOperation(read.owner)
		if readErr != nil {
			return readErr
		}
		if err = arraySourceAssert(192, "is_select(n)", func() bool { return readArray && readOp == ast.ASTArrayOpSelect }); err != nil {
			return err
		}
		if !readArray || readOp != ast.ASTArrayOpSelect {
			return ast.ErrASTDomain
		}
		if err := t.enqueueStoreReadAxiom(read.owner, store.owner); err != nil {
			return err
		}
	}
	return t.setStoreBaseUpward(store)
}

func (t *SMTArrayTheory) mergeArrayClasses(root, child uint32) error {
	if err := arraySourceAssert(51, "v1 == find(v1)", func() bool { return root < uint32(len(t.arrayData)) && root == t.arrayUnion.Find(root) }); err != nil {
		return err
	}
	if root >= uint32(len(t.arrayData)) || child >= uint32(len(t.arrayData)) {
		return ast.ErrASTDomain
	}
	if err := t.emitSimpleArrayTrace("array", "merge_eh", 48, nil, func(out io.Writer) error {
		if _, err := fmt.Fprintf(out, "merging v%d v%d\n", root, child); err != nil {
			return err
		}
		if err := t.DisplayVariable(out, SMTTheoryVar(root)); err != nil {
			return err
		}
		if err := displayArrayTraceExpression(t.state.manager, out, t.variables[root].owner); err != nil {
			return err
		}
		if _, err := io.WriteString(out, " <- "); err != nil {
			return err
		}
		if err := displayArrayTraceExpression(t.state.manager, out, t.variables[child].owner); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	}); err != nil {
		return err
	}
	first, second := t.arrayData[root], t.arrayData[child]
	if !first.propUpward && second.propUpward {
		if err := t.setArrayUpward(root); err != nil {
			return err
		}
	}
	for _, store := range second.stores {
		if err := t.addArrayStore(root, store); err != nil {
			return err
		}
	}
	for _, store := range second.parentStores {
		if err := t.addParentStore(SMTTheoryVar(root), store); err != nil {
			return err
		}
	}
	for _, read := range second.parentSelects {
		if err := t.addParentSelect(SMTTheoryVar(root), read); err != nil {
			return err
		}
	}
	if !t.simple {
		if err := t.mergeFullArrayClasses(root, second); err != nil {
			return err
		}
		if err := t.emitFullArrayTrace("array", "merge_eh", 359, nil, func(out io.Writer) error {
			if int(root) >= len(t.variables) || int(child) >= len(t.variables) {
				return ast.ErrASTDomain
			}
			if err := displayArrayTraceExpression(t.state.manager, out, t.variables[root].owner); err != nil {
				return err
			}
			if _, err := io.WriteString(out, "\n"); err != nil {
				return err
			}
			if err := displayArrayTraceExpression(t.state.manager, out, t.variables[child].owner); err != nil {
				return err
			}
			if _, err := io.WriteString(out, "\nmerge in\n"); err != nil {
				return err
			}
			if err := t.DisplayVariable(out, SMTTheoryVar(child)); err != nil {
				return err
			}
			if _, err := io.WriteString(out, "after merge\n"); err != nil {
				return err
			}
			return t.DisplayVariable(out, SMTTheoryVar(root))
		}); err != nil {
			return err
		}
	}
	return t.emitSimpleArrayTrace("array", "merge_eh", 62, nil, func(out io.Writer) error {
		if _, err := io.WriteString(out, "after merge\n"); err != nil {
			return err
		}
		return t.DisplayVariable(out, SMTTheoryVar(root))
	})
}

func (t *SMTArrayTheory) Relevant(expression ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	if basic, basicErr := state.manager.GetFamilyID(expression); basicErr == nil && basic == state.manager.BasicFamilyID() {
		if op, opErr := state.manager.GetDeclKind(expression); opErr == nil && op == ast.ASTOpIte {
			return t.traceSimpleArrayExpression("array", "relevant_eh", 318, "relevant ite ", expression)
		}
	}
	op, array, err := t.arrayOperation(expression)
	if err != nil || !array {
		return err
	}
	s, err := t.State()
	if err != nil {
		return err
	}
	if !t.simple {
		if err = t.traceFullArrayExpression("relevant_eh", 428, expression); err != nil {
			return err
		}
	}
	node := s.context.enodes[expression]
	if node == nil {
		return nil
	}
	if t.arrayConfig.Laziness != 0 && (op == ast.ASTArrayOpStore || op == ast.ASTArrayOpSelect) {
		variable := node.arguments[0].TheoryVar(t.FamilyID())
		if err = arraySourceAssert(325, "v_arg != null_theory_var", func() bool { return variable != SMTNullTheoryVar }); err != nil {
			return err
		}
		if variable == SMTNullTheoryVar {
			return ast.ErrASTDomain
		}
		if op == ast.ASTArrayOpStore {
			if err = arraySourceAssert(330, "is_store(n)", func() bool { return true }); err != nil {
				return err
			}
			err = t.addParentStore(variable, node)
		} else {
			err = t.addParentSelect(variable, node)
		}
		if err != nil {
			return err
		}
	}
	if !t.simple && op == ast.ASTArrayOpSelect {
		return t.addParentSelect(node.arguments[0].TheoryVar(t.FamilyID()), node)
	}
	if !t.simple && op == ast.ASTArrayOpMap {
		for _, input := range node.arguments {
			variable := input.TheoryVar(t.FamilyID())
			if err := t.addParentMap(variable, node); err != nil {
				return err
			}
			if err := t.setArrayUpward(uint32(variable)); err != nil {
				return err
			}
		}
	}
	if !t.simple {
		if op == ast.ASTArrayOpDefault {
			return t.assertDefaultAxiom(expression)
		}
		if op == ast.ASTArrayOpConst || op == ast.ASTArrayOpMap || op == ast.ASTArrayOpAsArray {
			_, err := t.instantiateConstructorDefault(expression)
			return err
		}
	}
	return nil
}

func (t *SMTArrayTheory) assertDelayedStoreAxioms() (bool, error) {
	if !t.arrayConfig.DelayExpAxiom {
		return false, nil
	}
	progress := false
	for _, data := range t.arrayData {
		if !data.propUpward {
			continue
		}
		for _, store := range data.parentStores {
			for _, read := range data.parentSelects {
				added, err := t.queueStoreReadAxiom(read.owner, store.owner, true)
				if err != nil {
					return false, err
				}
				progress = progress || added
			}
		}
		if !t.simple {
			for _, mapped := range data.parentMaps {
				for _, read := range data.parentSelects {
					added, err := t.instantiateMapRead(read, mapped)
					if err != nil {
						return false, err
					}
					progress = progress || added
				}
			}
		}
	}
	return progress, nil
}

package smtcore

// Source: theory_datatype var_data, merge_eh and add_recognizer.

import (
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/backtrack"
	"github.com/glycerine/g3/pkg/util/lbool"
	"io"
)

type smtDatatypeVarData struct {
	constructor *SMTEnode
	recognizers []*SMTEnode
}

type smtDatatypeUnionContext struct{ theory *SMTDatatypeTheory }

func (c *smtDatatypeUnionContext) GetTrailStack() *backtrack.TrailStack      { return c.theory.findTrail }
func (*smtDatatypeUnionContext) UnmergeEH(uint32, uint32)                    {}
func (*smtDatatypeUnionContext) AfterMergeEH(uint32, uint32, uint32, uint32) {}
func (c *smtDatatypeUnionContext) MergeEH(root, child, _, _ uint32) {
	if c.theory.mergeError == nil {
		c.theory.mergeError = c.theory.mergeDatatypeClasses(root, child)
	}
}

func (t *SMTDatatypeTheory) rootData(variable SMTTheoryVar) (uint32, *smtDatatypeVarData, error) {
	if variable < 0 || uint32(variable) >= t.find.GetNumVars() {
		return 0, nil, ast.ErrASTDomain
	}
	root := t.find.Find(uint32(variable))
	if int(root) >= len(t.data) {
		return 0, nil, ast.ErrASTDomain
	}
	return root, t.data[root], nil
}

func (t *SMTDatatypeTheory) mergeDatatypeClasses(root, child uint32) error {
	s, err := t.State()
	if err != nil {
		return err
	}
	if err = t.emitDatatype("datatype", "merge_eh", 663, func(out io.Writer) error {
		_, err := fmt.Fprintf(out, "merging v%d v%d\n", root, child)
		return err
	}); err != nil {
		return err
	}
	first, second := t.data[root], t.data[child]
	if second.constructor != nil {
		if first.constructor != nil && first.constructor.declaration != second.constructor.declaration {
			if err = t.setDatatypeConflict(nil, []SMTEnodePair{{first.constructor, second.constructor}}); err != nil {
				return err
			}
		}
		if first.constructor == nil {
			_ = t.findTrail.PushUndo(func() { first.constructor = nil })
			index, matched, err := s.manager.DatatypeConstructorIndex(second.constructor.declaration)
			if err != nil {
				return err
			}
			if !matched {
				return ast.ErrASTDomain
			}
			if int(index) < len(first.recognizers) && first.recognizers[index] != nil {
				literal, err := s.Literal(first.recognizers[index].owner)
				if err != nil {
					return err
				}
				value, err := s.Assignment(literal)
				if err != nil {
					return err
				}
				if value == lbool.False {
					return t.signRecognizerConflict(second.constructor, first.recognizers[index])
				}
			}
			first.constructor = second.constructor
		}
	}
	for _, recognizer := range second.recognizers {
		if recognizer != nil {
			if err = t.addRecognizer(SMTTheoryVar(root), recognizer); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *SMTDatatypeTheory) addRecognizer(variable SMTTheoryVar, recognizer *SMTEnode) error {
	s, err := t.State()
	if err != nil {
		return err
	}
	root, data, err := t.rootData(variable)
	if err != nil {
		return err
	}
	if recognizer == nil || len(recognizer.arguments) != 1 {
		return ast.ErrASTDomain
	}
	sort, err := s.manager.GetSort(recognizer.arguments[0].owner)
	if err != nil {
		return err
	}
	constructors, err := s.manager.DatatypeConstructors(sort)
	if err != nil {
		return err
	}
	if len(data.recognizers) == 0 {
		data.recognizers = make([]*SMTEnode, len(constructors))
	}
	index, matched, err := s.manager.DatatypeRecognizerConstructorIndex(recognizer.declaration)
	if err != nil {
		return err
	}
	if !matched || int(index) >= len(data.recognizers) {
		return ast.ErrASTDomain
	}
	if data.recognizers[index] != nil {
		return nil
	}
	literal, err := s.Literal(recognizer.owner)
	if err != nil {
		return err
	}
	value, err := s.Assignment(literal)
	if err != nil {
		return err
	}
	if err = t.emitDatatype("datatype", "add_recognizer", 712, func(out io.Writer) error {
		_, _ = fmt.Fprintf(out, "adding recognizer to v%d rec: #", root)
		id, err := recognizer.OwnerID()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "%d val: %s\n", id, value.String())
		return err
	}); err != nil {
		return err
	}
	if value == lbool.True {
		return nil
	}
	if value == lbool.False && data.constructor != nil {
		if data.constructor.declaration == constructors[index] {
			return t.signRecognizerConflict(data.constructor, recognizer)
		}
		return nil
	}
	data.recognizers[index] = recognizer
	_ = t.findTrail.PushUndo(func() { data.recognizers[index] = nil })
	if value == lbool.False {
		return t.propagateRecognizer(SMTTheoryVar(root), recognizer)
	}
	return nil
}

func (t *SMTDatatypeTheory) Relevant(expression ast.ASTHandle) error {
	s, err := t.State()
	if err != nil {
		return err
	}
	if err = t.emitDatatype("datatype", "relevant_eh", 396, func(out io.Writer) error {
		_, _ = io.WriteString(out, "relevant_eh: ")
		if err := t.displayDatatypeBounded(out, expression); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	}); err != nil {
		return err
	}
	op, own, err := t.operation(expression)
	if err != nil || !own || op != ast.ASTDatatypeRecognizer && op != ast.ASTDatatypeIs {
		return err
	}
	if err = t.emitDatatype("datatype", "relevant_eh", 400, func(out io.Writer) error {
		_, _ = io.WriteString(out, "relevant_eh: #")
		id, err := s.manager.GetASTID(expression)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "%d\n", id)
		if err = t.displayDatatypeBounded(out, expression); err != nil {
			return err
		}
		_, err = io.WriteString(out, "\n")
		return err
	}); err != nil {
		return err
	}
	node := s.context.enodes[expression]
	if node == nil || len(node.arguments) != 1 {
		return ast.ErrASTDomain
	}
	return t.addRecognizer(node.arguments[0].TheoryVar(t.FamilyID()), node)
}

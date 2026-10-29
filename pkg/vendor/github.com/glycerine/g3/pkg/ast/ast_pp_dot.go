package ast

// Source: src/ast/ast_pp_dot.cpp; src/ast/ast_pp_dot.h.

import (
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func escapeDOT(s string) string { return strings.ReplaceAll(s, "\n", `\l`) }

func writeDOTString(out io.Writer, s string) error {
	n, err := io.WriteString(out, s)
	if err == nil && n != len(s) {
		return io.ErrShortWrite
	}
	return err
}

func (m *ASTManager) proofDOTLabel(h ASTHandle) (value string, result error) {
	format, err := m.SMT2PPExprTree(h, nil)
	if err != nil {
		return "", err
	}
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.BudgetField})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	if err = m.PrintFormat(out, format, nil, nil, nil); err != nil {
		return "", err
	}
	value, err = out.String()
	return escapeDOT(value), err
}

func (m *ASTManager) proofDOTTraceStep(p ASTHandle) error {
	tags := m.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	return tags.EmitConditional(
		"pp_ast_dot_step", "pp_ast_dot_step", "src/ast/ast_pp_dot.cpp", 73,
		func() bool { return true }, func(w io.Writer) error {
			kind, err := m.GetKind(p)
			if err != nil {
				return err
			}
			n, err := m.GetNumArgs(p)
			if err != nil {
				return err
			}
			return writeDOTString(w, " :kind "+strconv.FormatUint(uint64(kind), 10)+" :num-args "+strconv.FormatUint(uint64(n), 10)+"\n")
		})
}

// WriteProofDOT writes proof in the source Graphviz representation. The proof
// and manager are borrowed for the duration of the call.
func (m *ASTManager) WriteProofDOT(out io.Writer, proof ASTHandle) error {
	if out == nil {
		return ErrASTDomain
	}
	isProof, err := m.IsProof(proof)
	if err != nil {
		return err
	}
	if !isProof {
		return ErrASTDomain
	}
	if err = writeDOTString(out, "digraph proof { \n"); err != nil {
		return err
	}

	nextID := uint32(0)
	ids := make(map[ASTHandle]uint32)
	printed := make(map[ASTHandle]struct{})
	stack := []ASTHandle{proof}
	first := true
	getID := func(h ASTHandle) uint32 {
		if id, ok := ids[h]; ok {
			return id
		}
		id := nextID
		nextID++
		ids[h] = id
		return id
	}
	atomicStep := func(h ASTHandle) error {
		label, e := m.proofDOTLabel(h)
		if e != nil {
			return e
		}
		return writeDOTString(out, "node_"+strconv.FormatUint(uint64(getID(h)), 10)+" [shape=box,color=\"yellow\",style=\"filled\",label=\""+label+"\"] ;\n")
	}

	for len(stack) != 0 {
		i := len(stack) - 1
		h := stack[i]
		stack = stack[:i]
		if _, ok := printed[h]; ok {
			continue
		}
		printed[h] = struct{}{}
		isProof, err = m.IsProof(h)
		if err != nil {
			return err
		}
		if !isProof {
			if err = atomicStep(h); err != nil {
				return err
			}
			continue
		}
		if err = m.proofDOTTraceStep(h); err != nil {
			return err
		}
		hasFact, e := m.ProofHasFact(h)
		if e != nil {
			return e
		}
		if !hasFact {
			if err = atomicStep(h); err != nil {
				return err
			}
			continue
		}
		fact, e := m.GetProofFact(h)
		if e != nil {
			return e
		}
		label, e := m.proofDOTLabel(fact)
		if e != nil {
			return e
		}
		n, e := m.GetProofNumParents(h)
		if e != nil {
			return e
		}
		color := ""
		if first {
			first = false
			color = `color="red"`
		} else if n == 0 {
			color = `color="yellow"`
		}
		id := getID(h)
		if err = writeDOTString(out, "node_"+strconv.FormatUint(uint64(id), 10)+" [shape=box,style=\"filled\",label=\""+label+"\""+color+"]\n"); err != nil {
			return err
		}
		decl, e := m.GetDecl(h)
		if e != nil {
			return e
		}
		name, e := m.GetASTName(decl)
		if e != nil {
			return e
		}
		rule, e := m.Symbols.String(name)
		if e != nil {
			return e
		}
		for parentIndex := uint32(0); parentIndex < n; parentIndex++ {
			parent, parentErr := m.GetProofParent(h, parentIndex)
			if parentErr != nil {
				return parentErr
			}
			stack = append(stack, parent)
			if err = writeDOTString(out, "node_"+strconv.FormatUint(uint64(id), 10)+" -> node_"+strconv.FormatUint(uint64(getID(parent)), 10)+"[label=\""+rule+"\"];\n"); err != nil {
				return err
			}
		}
	}
	if err = writeDOTString(out, "\n } \n"); err != nil {
		return err
	}
	return nil
}

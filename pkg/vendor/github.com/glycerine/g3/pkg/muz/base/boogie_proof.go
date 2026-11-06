package base

// Source: src/muz/base/dl_boogie_proof.h;
// src/muz/base/dl_boogie_proof.cpp.

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/model"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

var ErrBoogieProofOwner = errors.New("muz/base: invalid or closed Boogie proof owner")

type BoogieProofConfig struct {
	// Context and Globals are borrowed and are needed only when a model is
	// printed. They must belong to Manager's symbol service.
	Context *uparams.ParamsContext
	Globals *uparams.GlobalParamsService
	// Diagnostic receives the two literal set_proof messages. Nil preserves
	// the source's process stdout destination.
	Diagnostic io.Writer
}

type boogieSubstitution struct {
	name usymbol.Symbol
	term ast.ASTHandle
}

type boogieStep struct {
	ruleName usymbol.Symbol
	fact     ast.ASTHandle
	subst    []boogieSubstitution
	labels   []usymbol.Symbol
	refs     []uint32
}

// BoogieProof owns its proof reference and an independent model copy. The
// latter is the concrete Go replacement for source model_ref retention.
type BoogieProof struct {
	manager    *ast.ASTManager
	proof      ast.ASTRef
	model      *model.Model
	context    *uparams.ParamsContext
	globals    *uparams.GlobalParamsService
	diagnostic io.Writer
	closed     bool
}

func NewBoogieProof(manager *ast.ASTManager, config BoogieProofConfig) (*BoogieProof, error) {
	if manager == nil {
		return nil, ErrBoogieProofOwner
	}
	if config.Context != nil || config.Globals != nil {
		if config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || config.Context.SymbolsField != manager.Symbols {
			return nil, ErrBoogieProofOwner
		}
	}
	if config.Diagnostic == nil {
		config.Diagnostic = os.Stdout
	}
	proof, err := manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	return &BoogieProof{manager: manager, proof: proof, context: config.Context, globals: config.Globals, diagnostic: config.Diagnostic}, nil
}

func (b *BoogieProof) open() error {
	if b == nil || b.closed || b.manager == nil {
		return ErrBoogieProofOwner
	}
	return nil
}

// SetProof retains p, then applies the source proof normalization passes.
func (b *BoogieProof) SetProof(p ast.ASTHandle) error {
	if err := b.open(); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(b.diagnostic, "set proof"); err != nil {
		return err
	}
	if err := b.manager.SetRef(b.proof, p); err != nil {
		return err
	}
	if err := proofs.PushInstantiationsUp(b.manager, b.proof); err != nil {
		return err
	}
	normalized, err := b.manager.RefGet(b.proof)
	if err != nil {
		return err
	}
	normalized, err = boogieInputResolution(b.manager, normalized)
	if err != nil {
		return err
	}
	if err = b.manager.SetRef(b.proof, normalized); err != nil {
		return err
	}
	_, err = fmt.Fprintln(b.diagnostic, "proof set")
	return err
}

// SetModel replaces the retained source model_ref with a concrete owned copy.
func (b *BoogieProof) SetModel(in *model.Model) error {
	if err := b.open(); err != nil {
		return err
	}
	if in == nil {
		if b.model == nil {
			return nil
		}
		err := b.model.Close()
		if err == nil {
			b.model = nil
		}
		return err
	}
	if in.Core().Manager() != b.manager {
		return ErrBoogieProofOwner
	}
	copy, err := in.Copy()
	if err != nil {
		if copy != nil {
			err = errors.Join(err, copy.Close())
		}
		return err
	}
	if b.model != nil {
		if err = b.model.Close(); err != nil {
			return errors.Join(err, copy.Close())
		}
	}
	b.model = copy
	return nil
}

func boogieMatchHyper(manager *ast.ASTManager, proof ast.ASTHandle) (matched bool, premises []ast.ASTHandle, conclusion ast.ASTHandle, positions []ast.ASTProofPosition, substitutions [][]ast.ASTHandle, resultErr error) {
	conclusionRef, err := manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, nil, ast.ASTHandle{}, nil, nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&conclusionRef)) }()
	out, err := ast.NewASTProofHyperOutput(manager)
	if err != nil {
		return false, nil, ast.ASTHandle{}, nil, nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, out.Close()) }()
	defer func() {
		for _, premise := range premises {
			resultErr = errors.Join(resultErr, manager.DecRef(premise))
		}
	}()
	matched, err = manager.MatchProofHyperResolve(proof, &premises, conclusionRef, out)
	if err != nil || !matched {
		return matched, premises, ast.ASTHandle{}, nil, nil, err
	}
	conclusion, err = manager.RefGet(conclusionRef)
	if err != nil {
		return false, premises, ast.ASTHandle{}, nil, nil, err
	}
	positions = make([]ast.ASTProofPosition, out.PositionsLen())
	for i := range positions {
		positions[i], err = out.Position(uint32(i))
		if err != nil {
			return false, premises, ast.ASTHandle{}, nil, nil, err
		}
	}
	substitutions = make([][]ast.ASTHandle, out.SubstitutionsLen())
	for i := range substitutions {
		v, e := out.Substitution(uint32(i))
		if e != nil {
			return false, premises, ast.ASTHandle{}, nil, nil, e
		}
		substitutions[i] = append([]ast.ASTHandle(nil), v...)
	}
	return true, append([]ast.ASTHandle(nil), premises...), conclusion, positions, substitutions, nil
}

func boogieIsImplies(manager *ast.ASTManager, expression ast.ASTHandle) (left, right ast.ASTHandle, matched bool, err error) {
	kind, err := manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	op, err := manager.GetDeclKind(expression)
	if err != nil || op != ast.ASTOpImplies {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	left, err = manager.GetArg(expression, 0)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	right, err = manager.GetArg(expression, 1)
	return left, right, err == nil, err
}

// boogieInputResolution implements mk_input_resolution's nested first-premise
// rewrite. It returns a borrowed handle retained by either the input proof or
// the newly constructed proof.
func boogieInputResolution(manager *ast.ASTManager, proof ast.ASTHandle) (ast.ASTHandle, error) {
	outer, outerPremises, outerConclusion, _, _, err := boogieMatchHyper(manager, proof)
	if err != nil || !outer || len(outerPremises) == 0 {
		return proof, err
	}
	inner, innerPremises, _, innerPositions, innerSubstitutions, err := boogieMatchHyper(manager, outerPremises[0])
	if err != nil || !inner {
		return proof, err
	}
	for i := 1; i < len(outerPremises); i++ {
		outerPremises[i], err = boogieInputResolution(manager, outerPremises[i])
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	for i := range innerPremises {
		innerPremises[i], err = boogieInputResolution(manager, innerPremises[i])
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	for i := 1; i < len(innerPremises); i++ {
		fact, e := manager.GetProofFact(innerPremises[i])
		if e != nil {
			return ast.ASTHandle{}, e
		}
		antecedent, consequent, implies, e := boogieIsImplies(manager, fact)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if !implies {
			continue
		}
		literals, e := manager.FlattenAnd([]ast.ASTHandle{antecedent})
		if e != nil {
			return ast.ASTHandle{}, e
		}
		newPremises := []ast.ASTHandle{innerPremises[i]}
		newPositions := make([]ast.ASTProofPosition, 0, len(literals))
		newSubstitutions := [][]ast.ASTHandle{{}}
		for j, literal := range literals {
			for k := 1; k < len(outerPremises); k++ {
				outerFact, e := manager.GetProofFact(outerPremises[k])
				if e != nil {
					return ast.ASTHandle{}, e
				}
				if outerFact == literal {
					newPremises = append(newPremises, outerPremises[k])
					newPositions = append(newPositions, ast.ASTProofPosition{First: uint32(j + 1)})
					newSubstitutions = append(newSubstitutions, nil)
					break
				}
			}
		}
		innerPremises[i], e = manager.MkProofHyperResolve(newPremises, consequent, newPositions, newSubstitutions)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	return manager.MkProofHyperResolve(innerPremises, outerConclusion, innerPositions, innerSubstitutions)
}

func (b *BoogieProof) getSubstitution(proof ast.ASTHandle) ([]boogieSubstitution, error) {
	todo := []ast.ASTHandle{proof}
	visited := make(map[ast.ASTHandle]struct{})
	for len(todo) != 0 {
		last := len(todo) - 1
		current := todo[last]
		todo = todo[:last]
		if _, found := visited[current]; found {
			continue
		}
		visited[current] = struct{}{}
		matched, premises, _, _, substitutions, err := boogieMatchHyper(b.manager, current)
		if err != nil {
			return nil, err
		}
		if matched && len(substitutions) != 0 && len(substitutions[0]) != 0 {
			if len(premises) == 0 {
				return nil, ast.ErrASTDomain
			}
			quantifier, e := b.manager.GetProofFact(premises[0])
			if e != nil {
				return nil, e
			}
			count, e := b.manager.GetNumDeclarations(quantifier)
			if e != nil {
				return nil, e
			}
			if ast.ASTSourceDebug && int(count) != len(substitutions[0]) {
				return nil, ast.ErrASTDomain
			}
			result := make([]boogieSubstitution, len(substitutions[0]))
			for i, term := range substitutions[0] {
				name, e := b.manager.GetDeclarationName(quantifier, count-1-uint32(i))
				if e != nil {
					return nil, e
				}
				result[i] = boogieSubstitution{name: name, term: term}
			}
			return result, nil
		}
		parents, err := b.manager.GetProofNumParents(current)
		if err != nil {
			return nil, err
		}
		for i := uint32(0); i < parents; i++ {
			parent, e := b.manager.GetProofParent(current, i)
			if e != nil {
				return nil, e
			}
			todo = append(todo, parent)
		}
	}
	return nil, nil
}

func (b *BoogieProof) render(out io.Writer, expression ast.ASTHandle) error {
	printer, err := ast.NewSimpleASTPrinterContext(b.manager, out, nil)
	if err != nil {
		return err
	}
	return printer.DisplayExpr(out, expression, 0)
}

func (b *BoogieProof) ppProof(out io.Writer) error {
	root, err := b.manager.RefGet(b.proof)
	if err != nil {
		return err
	}
	rules := []ast.ASTHandle{root}
	steps := []boogieStep{{}}
	index := map[ast.ASTHandle]uint32{root: 0}
	for j := 0; j < len(rules); j++ {
		fact, e := b.manager.GetProofFact(rules[j])
		if e != nil {
			return e
		}
		_, consequent, implies, e := boogieIsImplies(b.manager, fact)
		if e != nil {
			return e
		}
		if implies {
			fact = consequent
		}
		steps[j].fact = fact
		steps[j].subst, e = b.getSubstitution(rules[j])
		if e != nil {
			return e
		}
		matched, premises, _, _, _, e := boogieMatchHyper(b.manager, rules[j])
		if e != nil {
			return e
		}
		if matched {
			for i := 1; i < len(premises); i++ {
				position, found := index[premises[i]]
				if !found {
					position = uint32(len(rules))
					index[premises[i]] = position
					rules = append(rules, premises[i])
					steps = append(steps, boogieStep{})
				}
				steps[j].refs = append(steps[j].refs, position)
			}
			// get_rule_name and get_labels are empty in the source.
		}
	}
	for i := range steps {
		for j, ref := range steps[i].refs {
			steps[i].refs[j] = uint32(len(rules)-1) - ref
		}
	}
	for left, right := 0, len(steps)-1; left < right; left, right = left+1, right-1 {
		steps[left], steps[right] = steps[right], steps[left]
	}
	if _, err = io.WriteString(out, "(derivation\n"); err != nil {
		return err
	}
	for i := range steps {
		if err = b.ppStep(out, uint32(i), &steps[i]); err != nil {
			return err
		}
	}
	_, err = io.WriteString(out, ")\n")
	return err
}

func (b *BoogieProof) ppStep(out io.Writer, id uint32, step *boogieStep) error {
	if _, err := fmt.Fprintf(out, "(step\n s!%d ", id); err != nil {
		return err
	}
	if err := b.render(out, step.fact); err != nil {
		return err
	}
	ruleName, err := b.manager.Symbols.String(step.ruleName)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(out, "\n %s\n (subst", ruleName); err != nil {
		return err
	}
	for _, assignment := range step.subst {
		name, e := b.manager.Symbols.String(assignment.name)
		if e != nil {
			return e
		}
		if _, e = fmt.Fprintf(out, "\n  (= %s ", name); e != nil {
			return e
		}
		if e = b.render(out, assignment.term); e != nil {
			return e
		}
		if _, e = io.WriteString(out, ")"); e != nil {
			return e
		}
	}
	if _, err = io.WriteString(out, ")\n (labels"); err != nil {
		return err
	}
	for _, label := range step.labels {
		text, e := b.manager.Symbols.String(label)
		if e != nil {
			return e
		}
		if _, e = fmt.Fprintf(out, " %s", text); e != nil {
			return e
		}
	}
	if _, err = io.WriteString(out, ")\n (ref"); err != nil {
		return err
	}
	for _, ref := range step.refs {
		if _, err = fmt.Fprintf(out, " s!%d", ref); err != nil {
			return err
		}
	}
	_, err = io.WriteString(out, ")\n)\n")
	return err
}

// PP writes the derivation and then the source old-format model.
func (b *BoogieProof) PP(out io.Writer) error {
	if err := b.open(); err != nil {
		return err
	}
	if out == nil {
		return ErrBoogieProofOwner
	}
	proof, err := b.manager.RefGet(b.proof)
	if err != nil {
		return err
	}
	if !proof.IsNull() {
		if err = b.ppProof(out); err != nil {
			return err
		}
	}
	if b.model != nil {
		if b.context == nil || b.globals == nil {
			return ErrBoogieProofOwner
		}
		return b.model.Core().WriteDebug(out, b.context, b.globals)
	}
	return nil
}

func (b *BoogieProof) Close() (err error) {
	if b == nil || b.manager == nil {
		return ErrBoogieProofOwner
	}
	if b.closed {
		return nil
	}
	if b.model != nil {
		err = b.model.Close()
		if err == nil {
			b.model = nil
		}
	}
	if b.proof != (ast.ASTRef{}) {
		err = errors.Join(err, b.manager.DelRef(&b.proof))
	}
	if err == nil {
		b.closed = true
	}
	return err
}

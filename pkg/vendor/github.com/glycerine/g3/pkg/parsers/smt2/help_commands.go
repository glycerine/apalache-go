package smt2

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/cmd_context"
	mathpoly "github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/escaped"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type commandHelpDescriptor struct {
	name                  string
	usage, description    string
	usagePresent, hasDesc bool
	dynamicDescription    func(*cmd_context.Context) (string, error)
}

func helpCommand(name, usage, description string) commandHelpDescriptor {
	return commandHelpDescriptor{name: name, usage: usage, description: description, usagePresent: true, hasDesc: true}
}

func helpCommandUsage(name, usage string) commandHelpDescriptor {
	return commandHelpDescriptor{name: name, usage: usage, usagePresent: true}
}

func helpCommandDescription(name, description string) commandHelpDescriptor {
	return commandHelpDescriptor{name: name, description: description, hasDesc: true}
}

func helpParametric(name, usage, main string, collect func(*cmd_context.Context, *uparams.ParamDescrs) error) commandHelpDescriptor {
	d := commandHelpDescriptor{name: name, usage: usage, usagePresent: true, hasDesc: true}
	d.dynamicDescription = func(context *cmd_context.Context) (result string, resultErr error) {
		manager, err := context.Manager()
		if err != nil {
			return "", err
		}
		descriptions, err := uparams.NewParamDescrs(manager.Symbols, uparams.ParamDescrsConfig{Budget: manager.Budget()})
		if err != nil {
			return "", err
		}
		defer func() { resultErr = errors.Join(resultErr, descriptions.Close()) }()
		if err = collect(context, descriptions); err != nil {
			return "", err
		}
		options, err := descriptions.Display(2, false, true)
		if err != nil {
			return "", err
		}
		return main + "\nThe following options are available:\n" + options, nil
	}
	return d
}

func helpParametricDescriptors(context *cmd_context.Context, name string, out *uparams.ParamDescrs) error {
	insert := func(key string, kind uparams.ParamKind, description string, defaultValue *string) error {
		return out.InsertName(key, kind, &description, defaultValue, nil)
	}
	switch name {
	case "check-sat-using":
		if err := uparams.InsertTimeout(out); err != nil {
			return err
		}
		if err := uparams.InsertMaxMemory(out); err != nil {
			return err
		}
		if err := insert("print_statistics", uparams.ParamBool, "(default: false) print statistics.", nil); err != nil {
			return err
		}
		for _, item := range []struct{ name, description string }{
			{"print_unsat_core", "(default: false) print unsatisfiable core."},
			{"print_proof", "(default: false) print proof."},
			{"print_model", "(default: false) print model."},
		} {
			if err := insert(item.name, uparams.ParamBool, item.description, nil); err != nil {
				return err
			}
		}
		return nil
	case "apply":
		if err := insert("print", uparams.ParamBool, "(default: true) print resultant goals.", nil); err != nil {
			return err
		}
		if err := insert("print_proof", uparams.ParamBool, "(default: false) print proof associated with each assertion.", nil); err != nil {
			return err
		}
		for _, item := range []struct{ name, description string }{
			{"print_model_converter", "(default: false) print model converter."},
			{"print_benchmark", "(default: false) display resultant goals as a SMT2 benchmark."},
			{"print_dependencies", "(default: false) print dependencies when displaying the resultant set of goals."},
		} {
			if err := insert(item.name, uparams.ParamBool, item.description, nil); err != nil {
				return err
			}
		}
		if err := uparams.InsertTimeout(out); err != nil {
			return err
		}
		if err := uparams.InsertMaxMemory(out); err != nil {
			return err
		}
		return insert("print_statistics", uparams.ParamBool, "(default: false) print statistics.", nil)
	case "assert-soft":
		if err := insert("weight", uparams.ParamNumeral, "(default: 1) penalty of not satisfying constraint.", nil); err != nil {
			return err
		}
		return insert("id", uparams.ParamSymbol, "(default: null) partition identifier for soft constraints.", nil)
	case "eval":
		if err := model.ModelEvaluatorParameterDescriptions(out); err != nil {
			return err
		}
		if err := uparams.InsertTimeout(out); err != nil {
			return err
		}
		return insert("model_index", uparams.ParamUInt, "(default: 0) index of model from box optimization objective", nil)
	case "simplify", "dbg-th-rewriter":
		if err := rewriter.TheoryRewriterParameterDescriptions(out); err != nil {
			return err
		}
		if err := uparams.InsertTimeout(out); err != nil {
			return err
		}
		if err := insert("print", uparams.ParamBool, "(default: true)  print the simplified term.", nil); err != nil {
			return err
		}
		if err := insert("print_proofs", uparams.ParamBool, "(default: false) print a proof showing the original term is equal to the resultant one.", nil); err != nil {
			return err
		}
		return insert("print_statistics", uparams.ParamBool, "(default: false) print statistics.", nil)
	case "poly/factor":
		return mathpoly.CollectPolynomialFactorParamDescriptions(out)
	case "query":
		globals, err := context.GlobalParameters()
		if err != nil {
			return err
		}
		fixedpoint, err := globals.SourceModuleDescriptorsName("fixedpoint")
		if err != nil {
			return err
		}
		return out.Copy(fixedpoint)
	case "get-interpolant", "compute-interpolant":
		return nil
	default:
		return nil
	}
}

func commandHelpCatalog() []commandHelpDescriptor {
	entries := []commandHelpDescriptor{
		helpCommandDescription("exit", "exit."),
		helpCommandDescription("get-assignment", "retrieve assignment"),
		helpCommandDescription("get-assertions", "retrieve asserted terms when in interactive mode"),
		helpCommandDescription("get-proof", "retrieve proof"),
		helpCommandDescription("get-proof-graph", "retrieve proof and print it in graphviz"),
		helpCommandDescription("get-unsat-core", "retrieve unsat core"),
		helpCommandDescription("get-unsat-assumptions", "retrieve subset of assumptions sufficient for unsatisfiability"),
		helpCommandDescription("reset-assertions", "reset all asserted formulas (but retain definitions and declarations)"),
		helpCommandDescription("labels", "retrieve Simplify-like labels"),
		helpCommand("assert", "<term>", "assert term."),
		helpCommand("check-sat", "<boolean-constants>*", "check if the current context is satisfiable. If a list of boolean constants B is provided, then check if the current context is consistent with assigning every constant in B to true."),
		helpCommand("push", "<number>?", "push 1 (or <number>) scopes."),
		helpCommand("pop", "<number>?", "pop 1 (or <number>) scopes."),
		helpCommand("get-value", "(<term>+)", "evaluate the given terms in the current model."),
		helpCommand("declare-sort", "<symbol> <numeral>?", "declare a new uninterpreted sort of arity <numeral>, if arity is not provided, then it is assumed to be 0."),
		helpCommand("define-sort", "<symbol> (<symbol>*) <sort>", "define a new sort."),
		helpCommand("declare-fun", "<symbol> (<sort>*) <sort>", "declare a new function/constant."),
		helpCommand("declare-const", "<symbol> <sort>", "declare a new constant."),
		helpCommand("declare-datatypes", "(<symbol>*) (<datatype-declaration>+)", "declare mutually recursive datatypes.\n<datatype-declaration> ::= (<symbol> <constructor-decl>+)\n<constructor-decl> ::= (<symbol> <accessor-decl>*)\n<accessor-decl> ::= (<symbol> <sort>)\nexample: (declare-datatypes (T) ((BinTree (leaf (value T)) (node (left BinTree) (right BinTree)))))"),
		helpCommand("check-sat-assuming", "( hprop_literali* )", "check sat assuming a collection of literals"),
		helpCommand("help", "<symbol>*", "print this help."),
		helpCommand("get-model", "[<index of box objective>]", "retrieve model for the last check-sat command.\nSupply optional index if retrieving a model corresponding to a box optimization objective"),
		helpCommand("set-logic", "<symbol>", "set the background logic."),
		helpCommand("display", "<term>", "display the given term."),
		helpCommand("echo", "<string>", "display the given string"),
		helpCommand("set-option", "<keyword> <value>", "set configuration option."),
		helpCommand("get-option", "<keyword>", "get configuration option."),
		helpCommand("get-info", "<keyword>", "get information."),
		helpCommand("set-info", "<keyword> <value>", "set information."),
		helpCommand("declare-map", "<symbol> (<sort>+) <func-decl-ref>", "declare a new array map operator with name <symbol> using the given function declaration.\n<func-decl-ref> ::= <symbol>\n                  | (<symbol> (<sort>*) <sort>)\n                  | ((_ <symbol> <numeral>+) (<sort>*) <sort>)\nThe last two cases are used to disumbiguate between declarations with the same name and/or select (indexed) builtin declarations.\nFor more details about the array map operator, see 'Generalized and Efficient Array Decision Procedures' (FMCAD 2009).\nExample: (declare-map set-union (Int) (or (Bool Bool) Bool))\nDeclares a new function (declare-fun set-union ((Array Int Bool) (Array Int Bool)) (Array Int Bool)).\nThe instance of the map axiom for this new declaration is:\n(forall ((a1 (Array Int Bool)) (a2 (Array Int Bool)) (i Int)) (= (select (set-union a1 a2) i) (or (select a1 i) (select a2 i))))"),
		helpCommand("get-consequences", "(<boolean-variable>*) (<variable>*)", "retrieve consequences that fix values for supplied variables"),
		helpCommandDescription("get-user-tactics", "display tactics defined using the declare-tactic command."),
		helpCommandDescription("help-tactic", "display the tactic combinators and primitives."),
		helpCommand("declare-tactic", "<symbol> <tactic>", "declare a new tactic, use (help-tactic) for the tactic language syntax."),
		helpParametric("check-sat-using", "<tactic> (<keyword> <value>)*", "check if the current context is satisfiable using the given tactic, use (help-tactic) for the tactic language syntax.", func(c *cmd_context.Context, d *uparams.ParamDescrs) error {
			return helpParametricDescriptors(c, "check-sat-using", d)
		}),
		helpParametric("apply", "<tactic> (<keyword> <value>)*", "apply the given tactic to the current context, and print the resultant set of goals.", func(c *cmd_context.Context, d *uparams.ParamDescrs) error {
			return helpParametricDescriptors(c, "apply", d)
		}),
		helpParametric("get-interpolant", "<fmla>+", "get interpolant for formulas", func(_ *cmd_context.Context, _ *uparams.ParamDescrs) error { return nil }),
		helpParametric("compute-interpolant", "<fmla>+", "get interpolant for formulas", func(_ *cmd_context.Context, _ *uparams.ParamDescrs) error { return nil }),
		helpParametric("eval", "<term> (<keyword> <value>)*", "evaluate the given term in the current model.", func(c *cmd_context.Context, d *uparams.ParamDescrs) error {
			return helpParametricDescriptors(c, "eval", d)
		}),
		helpParametric("simplify", "<term> (<keyword> <value>)*", "simplify the given term using builtin theory simplification rules.", func(c *cmd_context.Context, d *uparams.ParamDescrs) error {
			return helpParametricDescriptors(c, "simplify", d)
		}),
		helpParametric("dbg-th-rewriter", "<term> (<keyword> <value>)*", "simplify the given term using builtin theory simplification rules.", func(c *cmd_context.Context, d *uparams.ParamDescrs) error {
			return helpParametricDescriptors(c, "dbg-th-rewriter", d)
		}),
		helpCommandDescription("reset", "reset the shell (all declarations and assertions will be erased)"),
		helpParametric("assert-soft", "<formula> [:weight <rational-weight>] [:id <symbol>]", "assert soft constraint with optional weight and identifier", func(c *cmd_context.Context, d *uparams.ParamDescrs) error {
			return helpParametricDescriptors(c, "assert-soft", d)
		}),
		helpCommand("maximize", "<term>", "check sat modulo objective function"),
		helpCommand("minimize", "<term>", "check sat modulo objective function"),
		helpCommand("get-objectives", "(get-objectives)", "retrieve the objective values (after optimization)"),
		helpParametric("query", "predicate", "pose a query to a predicate based on the Horn rules.", func(c *cmd_context.Context, d *uparams.ParamDescrs) error {
			return helpParametricDescriptors(c, "query", d)
		}),
		helpCommand("rule", "(forall (q) (=> (and body) head)) :optional-name :optional-recursion-bound", "add a Horn rule."),
		helpCommand("declare-rel", "<symbol> (<arg1 sort> ...) <representation>*", "declare new relation"),
		helpCommand("declare-var", "<symbol> <sort>", "declare constant as variable"),
		helpCommand("fixedpoint-push", "", "push the fixedpoint context"),
		helpCommand("fixedpoint-pop", "", "pop the fixedpoint context"),
		helpCommand("include", "<string>", "include a file"),
		helpCommand("to-poly", "<term>", "convert expression into sum-of-monomials form"),
		helpCommand("poly/isolate-roots", "<term> (<term> <value>)*", "isolate the roots a multivariate polynomial modulo an assignment"),
		helpParametric("poly/factor", "<term> (<keyword> <value>)*", "factor a polynomial", func(c *cmd_context.Context, d *uparams.ParamDescrs) error {
			return helpParametricDescriptors(c, "poly/factor", d)
		}),
		helpCommand("to-subpaving", "<expr>", "internalize expression into subpaving module"),
		helpCommand("dbg-get-qbody", "<symbol> <quantifier>", "store the body of the quantifier in the global variable <symbol>"),
		helpCommand("dbg-set", "<symbol> <term>", "store <term> in the global variable <symbol>"),
		helpCommand("dbg-pp-var", "<symbol>", "pretty print a global variable that references an AST"),
		helpCommand("dbg-shift-vars", "<symbol> <uint>", "shift the free variables by <uint> in the term referenced by the global variable <symbol>, the result is stored in <symbol>"),
		helpCommand("dbg-pp-shared", "<term>", "display shared subterms of the given term"),
		helpCommand("dbg-num-shared", "<term>", "return the number of shared subterms"),
		helpCommand("dbg-size", "<term>", "return the size of the given term"),
		helpCommand("dbg-subst", "<symbol> (<symbol>*) <symbol>", "substitute the free variables in the AST referenced by <symbol> using the ASTs referenced by <symbol>*"),
		helpCommand("dbg-bool-rewriter", "<term>", "apply the Boolean rewriter to the given term"),
		helpCommand("dbg-bool-flat-rewriter", "<term>", "apply the Boolean (flattening) rewriter to the given term"),
		helpCommand("dbg-elim-and", "<term>", "apply the Boolean rewriter (eliminating AND operator and flattening) to the given term"),
		helpCommand("dbg-lt", "<term> <term>", "return true if the first term is smaller than the second one in the internal Z3 total order on terms."),
		helpCommand("dbg-some-value", "<sort>", "retrieve some value of the given sort"),
		helpCommandDescription("dbg-params", "test parameters"),
		helpCommand("dbg-translator", "<term>", "test AST translator"),
		helpCommand("dbg-sexpr", "<sexpr>", "display an s-expr"),
		helpCommand("dbg-set-next-id", "<unsigned>", "set the next expression id to be at least the given value"),
		helpCommand("dbg-used-vars", "<expr>", "test used_vars functor"),
		helpCommand("dbg-elim-unused-vars", "<expr>", "eliminate unused vars from a quantifier"),
		helpCommand("dbg-instantiate", "<quantifier> (<symbol>*)", "instantiate the quantifier using the given expressions."),
		helpCommand("dbg-instantiate-nested", "<quantifier> (<symbol>*)", "instantiate the quantifier nested in the outermost quantifier, this command is used to test the instantiation procedure with quantifiers that contain free variables."),
		helpCommand("display-dimacs", "", "print benchmark in DIMACS format"),
	}
	return entries
}

func (p *commandParser) helpCommands() error {
	entries := commandHelpCatalog()
	byName := make(map[string]commandHelpDescriptor, len(entries))
	for _, entry := range entries {
		byName[entry.name] = entry
	}
	selected := make([]commandHelpDescriptor, 0)
	for p.token != RightParenToken {
		name, err := p.symbol()
		if err != nil {
			return err
		}
		entry, found := byName[name]
		if !found {
			return fmt.Errorf("unknown command '%s'", name)
		}
		selected = append(selected, entry)
	}
	if len(selected) == 0 {
		for _, entry := range byName {
			selected = append(selected, entry)
		}
		sort.Slice(selected, func(i, j int) bool { return selected[i].name < selected[j].name })
	}
	var body strings.Builder
	for _, entry := range selected {
		fmt.Fprintf(&body, " (%s", entry.name)
		if entry.usagePresent {
			usage := entry.usage
			quoted, err := escaped.String(&usage, true, 0)
			if err != nil {
				return err
			}
			fmt.Fprintf(&body, " %s", quoted)
		}
		body.WriteString(")\n")
		if entry.hasDesc {
			description := entry.description
			if entry.dynamicDescription != nil {
				var err error
				description, err = entry.dynamicDescription(p.context)
				if err != nil {
					return err
				}
			}
			quoted, err := escaped.String(&description, true, 4)
			if err != nil {
				return err
			}
			fmt.Fprintf(&body, "    %s\n", quoted)
		}
	}
	text := body.String()
	quoted, err := escaped.String(&text, true, 0)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(p.out, "\"%s\"\n", quoted)
	return err
}

// helpTactic emits tactic_cmds.cpp help_tactic's registry listing and
// combinator guide. The probe metadata is shared with the public probe API.
func (p *commandParser) helpTactic() (resultErr error) {
	if err := p.requireEnd(); err != nil {
		return err
	}
	var text strings.Builder
	text.WriteString("combinators:\n")
	text.WriteString("- (and-then <tactic>+) executes the given tactics sequencially.\n")
	text.WriteString("- (or-else <tactic>+) tries the given tactics in sequence until one of them succeeds (i.e., the first that doesn't fail).\n")
	text.WriteString("- (par-or <tactic>+) executes the given tactics in parallel until one of them succeeds (i.e., the first that doesn't fail).\n")
	text.WriteString("- (par-then <tactic1> <tactic2>) executes tactic1 and then tactic2 to every subgoal produced by tactic1. All subgoals are processed in parallel.\n")
	text.WriteString("- (try-for <tactic> <num>) executes the given tactic for at most <num> milliseconds, it fails if the execution takes more than <num> milliseconds.\n")
	text.WriteString("- (if <probe> <tactic> <tactic>) if <probe> evaluates to true, then execute the first tactic. Otherwise execute the second tactic.\n")
	text.WriteString("- (when <probe> <tactic>) shorthand for (if <probe> <tactic> skip).\n")
	text.WriteString("- (fail-if <probe>) fail if <probe> evaluates to true.\n")
	text.WriteString("- (using-params <tactic> <attribute>*) executes the given tactic using the given attributes, where <attribute> ::= <keyword> <value>. ! is a syntax sugar for using-params.\n")
	text.WriteString("builtin tactics:\n")

	m, err := p.context.Manager()
	if err != nil {
		return err
	}
	for _, name := range cmd_context.BuiltinTacticNames() {
		description, err := cmd_context.BuiltinTacticDescription(name)
		if err != nil {
			return fmt.Errorf("help-tactic %s: %w", name, err)
		}
		fmt.Fprintf(&text, "- %s %s\n", name, description)
		selected, err := p.context.NewBuiltinTacticWithOutput(name, p.out)
		if err != nil {
			return fmt.Errorf("help-tactic %s: %w", name, err)
		}
		if err = selected.IncRef(); err != nil {
			return errors.Join(err, selected.Cleanup())
		}
		func() {
			defer func() {
				if cause := selected.DecRef(); cause != nil {
					resultErr = errors.Join(resultErr, fmt.Errorf("help-tactic %s release: %w", name, cause))
				}
			}()
			descriptors, cause := uparams.NewParamDescrs(m.Symbols, uparams.ParamDescrsConfig{Budget: m.Budget()})
			if cause != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("help-tactic %s: %w", name, cause))
				return
			}
			defer func() { resultErr = errors.Join(resultErr, descriptors.Close()) }()
			if cause = selected.CollectParamDescrs(descriptors); cause != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("help-tactic %s: %w", name, cause))
				return
			}
			size, cause := descriptors.Size()
			if cause != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("help-tactic %s: %w", name, cause))
				return
			}
			if size == 0 {
				return
			}
			options, cause := descriptors.Display(4, false, true)
			if cause != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("help-tactic %s: %w", name, cause))
				return
			}
			text.WriteString(options)
		}()
		if resultErr != nil {
			return resultErr
		}
	}
	text.WriteString("builtin probes:\n")
	for _, probe := range tactic.BuiltinProbeDescriptors() {
		fmt.Fprintf(&text, "- %s %s\n", probe.Name, probe.Description)
	}
	contents := text.String()
	quoted, err := escaped.String(&contents, true, 0)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(p.out, "\"%s\"\n", quoted)
	return err
}

package apalache

import (
	"fmt"
	"sort"
	"strings"
)

// ModulePass is one ordered transformation in a PassChainExecutor.
type ModulePass interface {
	Name() string
	Dependencies() []ModuleProperty
	Transformations() []ModuleProperty
	Execute(*Module) (*Module, error)
}

// ModulePassFunc adapts a function and its property metadata to ModulePass.
type ModulePassFunc struct {
	PassName  string
	Requires  []ModuleProperty
	Adds      []ModuleProperty
	Transform func(*Module) (*Module, error)
}

func (pass ModulePassFunc) Name() string { return pass.PassName }
func (pass ModulePassFunc) Dependencies() []ModuleProperty {
	return append([]ModuleProperty(nil), pass.Requires...)
}
func (pass ModulePassFunc) Transformations() []ModuleProperty {
	return append([]ModuleProperty(nil), pass.Adds...)
}
func (pass ModulePassFunc) Execute(module *Module) (*Module, error) {
	if pass.Transform == nil {
		return nil, fmt.Errorf("pass %s has no transformation", pass.PassName)
	}
	return pass.Transform(module)
}

// PassChainExecutor runs an ordered pass list once, checking module-property
// dependencies before each pass and preserving accumulated properties.
type PassChainExecutor struct {
	passes   []ModulePass
	adapter  ExceptionAdapter
	executed bool
}

func NewPassChainExecutor(passes ...ModulePass) *PassChainExecutor {
	return &PassChainExecutor{passes: append([]ModulePass(nil), passes...)}
}

func NewPassChainExecutorWithAdapter(adapter ExceptionAdapter, passes ...ModulePass) *PassChainExecutor {
	return &PassChainExecutor{passes: append([]ModulePass(nil), passes...), adapter: adapter}
}

func (executor *PassChainExecutor) Run() (*Module, error) {
	if executor == nil {
		return nil, fmt.Errorf("cannot run a nil pass-chain executor")
	}
	if executor.executed {
		return nil, fmt.Errorf("PassChainExecutor can only be executed once")
	}
	executor.executed = true
	module, err := executor.run()
	if err != nil && executor.adapter != nil {
		if message, matched := executor.adapter.Adapt(err); matched {
			if message == nil {
				return nil, fmt.Errorf("exception adapter matched %T but returned a nil message", err)
			}
			return nil, &AdaptedError{Message: message}
		}
	}
	return module, err
}

func (executor *PassChainExecutor) run() (*Module, error) {
	module := NewModule("empty", nil)
	for _, pass := range executor.passes {
		if pass == nil {
			return nil, fmt.Errorf("pass chain contains a nil pass")
		}
		missing := missingPassProperties(module, pass.Dependencies())
		if len(missing) != 0 {
			return nil, fmt.Errorf("%s cannot run for a module without the properties: %s", pass.Name(), strings.Join(missing, ", "))
		}
		transformed, err := pass.Execute(module)
		if err != nil {
			return nil, err
		}
		if transformed == nil {
			return nil, fmt.Errorf("pass %s returned a nil module", pass.Name())
		}
		next := NewModule(transformed.Name(), transformed.Declarations())
		next.properties = cloneMap(module.properties)
		for _, property := range pass.Transformations() {
			next.AddProperty(property)
		}
		module = next
	}
	return module, nil
}

func missingPassProperties(module *Module, dependencies []ModuleProperty) []string {
	missing := make([]string, 0)
	for _, property := range dependencies {
		if !module.HasProperty(property) {
			missing = append(missing, modulePropertyName(property))
		}
	}
	sort.Strings(missing)
	return missing
}

func modulePropertyName(property ModuleProperty) string {
	switch property {
	case PropertyTemporalEncoded:
		return "TemporalEncoded"
	case PropertyDesugared:
		return "Desugared"
	case PropertyTypeChecked:
		return "TypeChecked"
	case PropertyConfigured:
		return "Configured"
	case PropertyInlined:
		return "Inlined"
	case PropertyPrimed:
		return "Primed"
	case PropertyVCGenerated:
		return "VCGenerated"
	case PropertyTransitionsFound:
		return "TransitionsFound"
	case PropertyPreprocessed:
		return "Preprocessed"
	case PropertyOptimized:
		return "Optimized"
	case PropertyAnalyzed:
		return "Analyzed"
	default:
		return fmt.Sprintf("ModuleProperty(%d)", property)
	}
}

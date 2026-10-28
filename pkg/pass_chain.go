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

// ToolModule is the mode-specific, ordered pass catalog corresponding to the
// Scala ToolModule boundary. Concrete Go modules construct their dependencies
// normally and expose only the resulting execution order.
type ToolModule interface {
	Passes() []ModulePass
}

type StaticToolModule struct{ passes []ModulePass }

func NewToolModule(passes ...ModulePass) StaticToolModule {
	return StaticToolModule{passes: append([]ModulePass(nil), passes...)}
}

func (module StaticToolModule) Passes() []ModulePass {
	return append([]ModulePass(nil), module.passes...)
}

func NewToolModuleExecutor(module ToolModule, adapter ExceptionAdapter, logger PassLogger) (*PassChainExecutor, error) {
	if module == nil {
		return nil, fmt.Errorf("cannot execute a nil tool module")
	}
	return NewPassChainExecutorWithRuntime(adapter, logger, module.Passes()...), nil
}

// ModulePassFunc adapts a function and its property metadata to ModulePass.
type ModulePassFunc struct {
	PassName  string
	Requires  []ModuleProperty
	Adds      []ModuleProperty
	Transform func(*Module) (*Module, error)
}

// PassLogger receives the lifecycle messages emitted around pass execution.
type PassLogger interface {
	Info(message string)
	Debug(message string)
}

type PassLoggerFunc func(level, message string)

func (logger PassLoggerFunc) Info(message string)  { logger("info", message) }
func (logger PassLoggerFunc) Debug(message string) { logger("debug", message) }

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
	logger   PassLogger
	executed bool
}

func NewPassChainExecutor(passes ...ModulePass) *PassChainExecutor {
	return &PassChainExecutor{passes: append([]ModulePass(nil), passes...)}
}

func NewPassChainExecutorWithAdapter(adapter ExceptionAdapter, passes ...ModulePass) *PassChainExecutor {
	return &PassChainExecutor{passes: append([]ModulePass(nil), passes...), adapter: adapter}
}

func NewPassChainExecutorWithRuntime(adapter ExceptionAdapter, logger PassLogger, passes ...ModulePass) *PassChainExecutor {
	return &PassChainExecutor{passes: append([]ModulePass(nil), passes...), adapter: adapter, logger: logger}
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
	for index, pass := range executor.passes {
		if pass == nil {
			return nil, fmt.Errorf("pass chain contains a nil pass")
		}
		missing := missingPassProperties(module, pass.Dependencies())
		if len(missing) != 0 {
			return nil, fmt.Errorf("%s cannot run for a module without the properties: %s", pass.Name(), strings.Join(missing, ", "))
		}
		if executor.logger != nil {
			executor.logger.Info(fmt.Sprintf("PASS #%d: %s", index, pass.Name()))
		}
		transformed, err := pass.Execute(module)
		if executor.logger != nil {
			outcome := "[OK]"
			if err != nil {
				outcome = "[FAIL]"
			}
			executor.logger.Debug(fmt.Sprintf("PASS #%d: %s %s", index, pass.Name(), outcome))
		}
		if err != nil {
			return nil, err
		}
		if transformed == nil {
			return nil, fmt.Errorf("pass %s returned a nil module", pass.Name())
		}
		next := NewModule(transformed.Name(), transformed.Declarations())
		next.copyMetadataFrom(module)
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

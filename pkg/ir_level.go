package apalache

import "fmt"

// Level is the semantic TLA+ level from Specifying Systems, chapter 17.
type Level uint8

const (
	ConstantLevel Level = iota
	StateLevel
	ActionLevel
	TemporalLevel
)

func LevelFromInt(number int) (Level, error) {
	if number < int(ConstantLevel) || number > int(TemporalLevel) {
		return ConstantLevel, fmt.Errorf("unexpected TLA+ level %d", number)
	}
	return Level(number), nil
}
func (level Level) Number() int { return int(level) }
func (level Level) String() string {
	switch level {
	case ConstantLevel:
		return "Constant"
	case StateLevel:
		return "State"
	case ActionLevel:
		return "Action"
	case TemporalLevel:
		return "Temporal"
	default:
		return fmt.Sprintf("Level(%d)", level)
	}
}
func (level Level) Join(other Level) Level {
	if other > level {
		return other
	}
	return level
}

type LevelFinder struct {
	constants map[string]struct{}
	variables map[string]struct{}
	operators map[string]*OperDecl
	cache     map[string]Level
}

func NewLevelFinder(module *Module) *LevelFinder {
	if module == nil {
		panic("NewLevelFinder requires a module")
	}
	finder := &LevelFinder{
		constants: make(map[string]struct{}), variables: make(map[string]struct{}),
		operators: make(map[string]*OperDecl), cache: make(map[string]Level),
	}
	for _, declaration := range module.ConstDeclarations() {
		finder.constants[declaration.Name()] = struct{}{}
	}
	for _, declaration := range module.VarDeclarations() {
		finder.variables[declaration.Name()] = struct{}{}
	}
	for _, declaration := range module.OperDeclarations() {
		finder.operators[declaration.Name()] = declaration
	}
	return finder
}

func (finder *LevelFinder) LevelOfName(name string) Level {
	return finder.levelOfName(make(map[string]struct{}), name)
}

func (finder *LevelFinder) levelOfName(callers map[string]struct{}, name string) Level {
	if level, ok := finder.cache[name]; ok {
		return level
	}
	level := ConstantLevel
	if _, ok := finder.constants[name]; ok {
		level = ConstantLevel
	} else if _, ok := finder.variables[name]; ok {
		level = StateLevel
	} else if declaration, ok := finder.operators[name]; ok {
		if _, recursive := callers[name]; !recursive {
			nextCallers := cloneStringSet(callers)
			nextCallers[name] = struct{}{}
			level = finder.levelOfExpression(nextCallers, declaration.Body())
		}
	}
	finder.cache[name] = level
	return level
}

func (finder *LevelFinder) LevelOfExpression(expr Expr) Level {
	return finder.levelOfExpression(make(map[string]struct{}), expr)
}

func (finder *LevelFinder) levelOfExpression(callers map[string]struct{}, expr Expr) Level {
	switch typed := expr.(type) {
	case *NameExpr:
		return finder.levelOfName(callers, typed.Name())
	case *OperExpr:
		level := operatorLevel(typed.Operator())
		for _, argument := range typed.Arguments() {
			level = level.Join(finder.levelOfExpression(callers, argument))
		}
		return level
	case *LetInExpr:
		level := finder.levelOfExpression(callers, typed.Body())
		for _, declaration := range typed.Declarations() {
			level = level.Join(finder.levelOfExpression(callers, declaration.Body()))
		}
		return level
	default:
		return ConstantLevel
	}
}

func (finder *LevelFinder) LevelOfDeclaration(declaration Decl) Level {
	if level, ok := finder.cache[declaration.Name()]; ok {
		return level
	}
	return finder.LevelOfDeclarationWithoutCache(declaration)
}

func (finder *LevelFinder) LevelOfDeclarationWithoutCache(declaration Decl) Level {
	switch typed := declaration.(type) {
	case *ConstDecl:
		return ConstantLevel
	case *VarDecl:
		return StateLevel
	case *OperDecl:
		callers := map[string]struct{}{typed.Name(): {}}
		level := finder.levelOfExpression(callers, typed.Body())
		finder.cache[typed.Name()] = level
		return level
	case *AssumeDecl, *TheoremDecl:
		return ConstantLevel
	default:
		return ConstantLevel
	}
}

func operatorLevel(operator Operator) Level {
	switch operator.Name() {
	case "PRIME", "ENABLED", "UNCHANGED", "STUTTER", "NO_STUTTER", "COMPOSE":
		return ActionLevel
	case "GLOBALLY", "EVENTUALLY", "LEADS_TO", "GUARANTEES", "WEAK_FAIRNESS", "STRONG_FAIRNESS", "TEMPORAL_EXISTS", "TEMPORAL_FORALL":
		return TemporalLevel
	default:
		return ConstantLevel
	}
}

func cloneStringSet(source map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{}, len(source)+1)
	for value := range source {
		result[value] = struct{}{}
	}
	return result
}

package ast

// Source: src/ast/ast.cpp:1520-1561.

import (
	"github.com/glycerine/g3/pkg/util/symbol"
)

type astFamilyPluginSnapshot struct {
	name   symbol.Symbol
	text   string
	kind   astPluginKind
	plugin bool
}

// CopyFamiliesPlugins preserves the source manager's family IDs and installs
// fresh concrete declaration plugins in this manager. Plugin-owned AST state is
// manager-local. ASTTranslation follows this registry copy with datatype
// definition inheritance because those definitions contain translatable ASTs.
func (m *ASTManager) CopyFamiliesPlugins(from *ASTManager) error {
	if from == nil {
		return ErrASTHandle
	}
	if err := from.lock(); err != nil {
		return err
	}
	n := from.families.Stats().NextID
	families := make([]astFamilyPluginSnapshot, 0, n)
	for id := FamilyID(0); id < n; id++ {
		name, err := from.families.GetName(id)
		if err != nil {
			from.mu.Unlock()
			return err
		}
		entry := astFamilyPluginSnapshot{name: name}
		if !name.IsNumerical() {
			entry.text, err = from.Symbols.String(name)
			if err != nil {
				from.mu.Unlock()
				return err
			}
		}
		binding := from.pluginAt(id)
		entry.kind, entry.plugin = binding.kind, binding.present
		families = append(families, entry)
	}
	from.mu.Unlock()

	// Reserve the entire family sequence before plugin constructors run. Some
	// constructors reach other families, so this first pass is what keeps IDs
	// identical to the source manager.
	for id, entry := range families {
		if err := m.lock(); err != nil {
			return err
		}
		name := entry.name
		var err error
		if !name.IsNumerical() {
			name, err = m.Symbols.Intern(entry.text)
		}
		if err == nil {
			var got FamilyID
			got, err = m.families.MkID(name)
			if err == nil && got != FamilyID(id) {
				err = ErrASTDomain
			}
		}
		m.mu.Unlock()
		if err != nil {
			return err
		}
	}

	for id, entry := range families {
		if !entry.plugin {
			continue
		}
		present, err := m.hasPlugin(FamilyID(id))
		if err != nil {
			return err
		}
		if !present {
			if err = m.registerTranslatedPlugin(entry.kind); err != nil {
				return err
			}
		}
		present, err = m.hasPluginKind(FamilyID(id), entry.kind)
		if err != nil {
			return err
		}
		if !present {
			return ErrASTDomain
		}
	}
	return nil
}

func (m *ASTManager) hasPlugin(id FamilyID) (bool, error) {
	if err := m.lock(); err != nil {
		return false, err
	}
	defer m.mu.Unlock()
	return m.pluginAt(id).present, nil
}

func (m *ASTManager) hasPluginKind(id FamilyID, kind astPluginKind) (bool, error) {
	if err := m.lock(); err != nil {
		return false, err
	}
	defer m.mu.Unlock()
	binding := m.pluginAt(id)
	return binding.present && binding.kind == kind, nil
}

func (m *ASTManager) registerTranslatedPlugin(kind astPluginKind) error {
	var err error
	switch kind {
	case astArithmeticPlugin:
		_, err = m.RegisterArithmeticPlugin()
	case astBitVectorPlugin:
		_, err = m.RegisterBitVectorPlugin()
	case astArrayPlugin:
		_, err = m.RegisterArrayPlugin()
	case astSequencePlugin:
		_, err = m.RegisterSequencePlugin()
	case astFloatingPointPlugin:
		_, err = m.RegisterFloatingPointPlugin()
	case astDatatypePlugin:
		_, err = m.RegisterDatatypePlugin()
	case astDatalogPlugin:
		_, err = m.RegisterDatalogPlugin()
	case astPseudoBooleanPlugin:
		_, err = m.RegisterPseudoBooleanPlugin()
	case astFormatPlugin:
		if !m.IsFormatManager() {
			return ErrASTDomain
		}
		if err = m.lock(); err == nil {
			m.formatExecuting = true
			_, err = m.formatFamilyID()
			m.formatExecuting = false
			m.mu.Unlock()
		}
	case astBasicPlugin, astLabelPlugin, astPatternPlugin, astModelValuePlugin, astUserSortPlugin:
		return ErrASTDomain
	default:
		return ErrASTDomain
	}
	return err
}

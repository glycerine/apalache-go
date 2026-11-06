package model

// Source: src/model/model.h; src/model/model_core.h.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

func (m *Model) Core() *ModelCore             { return &m.CoreField }
func (m *ModelCore) Manager() *ast.ASTManager { return m.manager }
func (m *ModelCore) Check() error             { return m.open() }

func (m *ModelCore) SetValueProvider(provider ModelValueProvider) error {
	if err := m.open(); err != nil {
		return err
	}
	if provider == nil {
		m.valueProvider = m.Derived
	} else {
		m.valueProvider = provider
	}
	return nil
}

func (m *ModelCore) DerivedModel() *Model { return m.Derived }
func (m *ModelCore) CheckFunctionStorageEmpty() error {
	for _, entry := range m.Functions.All() {
		if entry.function != nil {
			return errors.New("model: consumer requires a constant-only interpretation")
		}
	}
	return nil
}
func (m *ModelCore) NumFunctionDeclarations() uint32 {
	return uint32(len(m.functionDeclarations))
}

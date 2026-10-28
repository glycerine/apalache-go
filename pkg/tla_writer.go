package apalache

import (
	"encoding/json"
	"fmt"
	"io"
)

// TLAWriter is the Go spelling of the Scala writer contract. Implementations
// write modules, declarations, and expressions to one caller-owned stream.
type TLAWriter interface {
	WriteModule(*Module, []string) error
	WriteDeclaration(Decl) error
	WriteExpression(Expr) error
}

type PrettyTLAWriter struct {
	output      io.Writer
	annotations *AnnotationStore
}

func NewPrettyTLAWriter(output io.Writer, annotations *AnnotationStore) *PrettyTLAWriter {
	return &PrettyTLAWriter{output: output, annotations: annotations}
}

func (writer *PrettyTLAWriter) WriteModule(module *Module, extendedModules []string) error {
	if writer == nil || writer.output == nil {
		return fmt.Errorf("TLA+ writer has no output")
	}
	formatted, err := FormatTLAModule(module, TLAFormatOptions{
		ExtendedModules: append([]string(nil), extendedModules...), Annotations: writer.annotations,
	})
	if err != nil {
		return err
	}
	_, err = io.WriteString(writer.output, formatted)
	return err
}

func (writer *PrettyTLAWriter) WriteDeclaration(declaration Decl) error {
	if writer == nil || writer.output == nil {
		return fmt.Errorf("TLA+ writer has no output")
	}
	formatted, err := FormatTLADeclaration(declaration, writer.annotations)
	if err != nil {
		return err
	}
	_, err = io.WriteString(writer.output, formatted)
	return err
}

func (writer *PrettyTLAWriter) WriteExpression(expression Expr) error {
	if writer == nil || writer.output == nil {
		return fmt.Errorf("TLA+ writer has no output")
	}
	formatted, err := FormatTLAExpression(expression)
	if err != nil {
		return err
	}
	_, err = io.WriteString(writer.output, formatted)
	return err
}

type JSONTLAWriter struct {
	output io.Writer
	store  *SourceStore
}

func NewJSONTLAWriter(output io.Writer, store *SourceStore) *JSONTLAWriter {
	return &JSONTLAWriter{output: output, store: store}
}

func (writer *JSONTLAWriter) writeRepresentation(representation JSONRepresentation, err error) error {
	if err != nil {
		return err
	}
	if writer == nil || writer.output == nil {
		return fmt.Errorf("JSON writer has no output")
	}
	encoded, err := json.Marshal(representation.Value())
	if err != nil {
		return err
	}
	_, err = writer.output.Write(encoded)
	return err
}

func (writer *JSONTLAWriter) WriteModule(module *Module, _ []string) error {
	if writer == nil {
		return fmt.Errorf("JSON writer has no output")
	}
	return writer.writeRepresentation((IRJSONEncoder{Options: IRJSONOptions{SourceStore: writer.store}}).Root([]*Module{module}))
}

func (writer *JSONTLAWriter) WriteDeclaration(declaration Decl) error {
	if writer == nil {
		return fmt.Errorf("JSON writer has no output")
	}
	return writer.writeRepresentation((IRJSONEncoder{Options: IRJSONOptions{SourceStore: writer.store}}).Declaration(declaration))
}

func (writer *JSONTLAWriter) WriteExpression(expression Expr) error {
	if writer == nil {
		return fmt.Errorf("JSON writer has no output")
	}
	return writer.writeRepresentation((IRJSONEncoder{Options: IRJSONOptions{SourceStore: writer.store}}).Expression(expression))
}

type TLAWriterFactory struct {
	Annotations *AnnotationStore
	SourceStore *SourceStore
}

func (factory TLAWriterFactory) NewPrettyWriter(output io.Writer) TLAWriter {
	return NewPrettyTLAWriter(output, factory.Annotations)
}

func (factory TLAWriterFactory) NewJSONWriter(output io.Writer) TLAWriter {
	return NewJSONTLAWriter(output, factory.SourceStore)
}

func (factory TLAWriterFactory) WriteModuleAllFormats(
	manager *OutputManager,
	module *Module,
	extendedModules []string,
) (bool, error) {
	tlaWritten, err := factory.writeModuleInIntermediate(manager, module, extendedModules, ".tla", factory.NewPrettyWriter)
	if err != nil {
		return tlaWritten, err
	}
	jsonWritten, err := factory.writeModuleInIntermediate(manager, module, extendedModules, ".json", factory.NewJSONWriter)
	return tlaWritten || jsonWritten, err
}

func (factory TLAWriterFactory) writeModuleInIntermediate(
	manager *OutputManager,
	module *Module,
	extendedModules []string,
	extension string,
	create func(io.Writer) TLAWriter,
) (bool, error) {
	filename := extension
	if module != nil {
		filename = module.Name() + extension
	}
	return manager.WithWriterInIntermediateDir([]string{filename}, func(output io.Writer) error {
		return create(output).WriteModule(module, extendedModules)
	})
}

func (factory TLAWriterFactory) WriteModuleTLAFile(path string, module *Module, extendedModules []string) error {
	return withOutputFile(path, func(output io.Writer) error {
		return factory.NewPrettyWriter(output).WriteModule(module, extendedModules)
	})
}

func (factory TLAWriterFactory) WriteModuleJSONFile(path string, module *Module) error {
	return withOutputFile(path, func(output io.Writer) error {
		return factory.NewJSONWriter(output).WriteModule(module, nil)
	})
}

var _ TLAWriter = (*PrettyTLAWriter)(nil)
var _ TLAWriter = (*JSONTLAWriter)(nil)

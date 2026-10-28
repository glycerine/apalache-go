package apalache

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// TLAWriter is the Go spelling of the Scala writer contract. Implementations
// write modules, declarations, and expressions to one caller-owned stream.
type TLAWriter interface {
	WriteModule(*Module, []string) error
	WriteDeclaration(Decl) error
	WriteExpression(Expr) error
}

// TLADeclarationAnnotator supplies already-rendered annotation lines for a
// declaration. The bool distinguishes the default no-decoration result.
type TLADeclarationAnnotator interface {
	Annotate(TextLayout, Decl) ([]string, bool)
}

type TLADeclarationAnnotatorFunc func(TextLayout, Decl) ([]string, bool)

func (annotator TLADeclarationAnnotatorFunc) Annotate(layout TextLayout, declaration Decl) ([]string, bool) {
	return annotator(layout, declaration)
}

type NoTLADeclarationAnnotations struct{}

func (NoTLADeclarationAnnotations) Annotate(TextLayout, Decl) ([]string, bool) { return nil, false }

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

// PrettyTLAWriterWithAnnotations reproduces Scala's declaration annotator: a
// typed declaration receives a synthesized @type before its stored annotations.
type PrettyTLAWriterWithAnnotations struct {
	output io.Writer
	store  *AnnotationStore
}

func NewPrettyTLAWriterWithAnnotations(output io.Writer, store *AnnotationStore) *PrettyTLAWriterWithAnnotations {
	return &PrettyTLAWriterWithAnnotations{output: output, store: store}
}

func (writer *PrettyTLAWriterWithAnnotations) annotations(declaration Decl) []Annotation {
	annotations := make([]Annotation, 0, 1)
	if declaration != nil && declaration.Tag().IsTyped() {
		annotations = append(annotations, NewTypeAnnotation((TLAType1TagPrinter{}).PrintTypeTag(declaration.Tag())))
	}
	if writer != nil && writer.store != nil && declaration != nil {
		annotations = append(annotations, writer.store.Get(declaration.ID())...)
	}
	return annotations
}

func formatAnnotationBlock(annotations []string, outerIndent, annotationIndent string) string {
	var output strings.Builder
	output.WriteString(outerIndent + "(*\n")
	for _, annotation := range annotations {
		output.WriteString(annotationIndent + annotation + "\n")
	}
	output.WriteString(outerIndent + "*)")
	return output.String()
}

func (writer *PrettyTLAWriterWithAnnotations) formatDeclaration(declaration Decl) (string, error) {
	annotations := writer.annotations(declaration)
	lines := make([]string, len(annotations))
	for index, annotation := range annotations {
		lines[index] = annotation.PrettyString()
	}
	return formatDecoratedTLADeclaration(declaration, lines)
}

func formatDecoratedTLADeclaration(declaration Decl, annotations []string) (string, error) {
	base, err := FormatTLADeclaration(declaration, nil)
	if err != nil {
		return "", err
	}
	if len(annotations) == 0 {
		return base, nil
	}
	switch typed := declaration.(type) {
	case *VarDecl:
		return "VARIABLE\n" + formatAnnotationBlock(annotations, "  ", "    ") + "\n  " + ParseableTLAName(typed.Name()), nil
	case *ConstDecl:
		return "CONSTANT\n" + formatAnnotationBlock(annotations, "  ", "    ") + "\n  " + ParseableTLAName(typed.Name()), nil
	case *AssumeDecl:
		if _, named := typed.DefinedName(); !named {
			base = strings.Replace(base, "ASSUME (", "ASSUME(", 1)
		}
	}
	block := formatAnnotationBlock(annotations, "", "  ")
	if recursive, ok := declaration.(*OperDecl); ok && recursive.IsRecursive() {
		parts := strings.SplitN(base, "\n", 2)
		if len(parts) == 2 {
			return parts[0] + "\n" + block + "\n" + parts[1], nil
		}
	}
	return block + "\n" + base, nil
}

type PrettyTLAWriterWithAnnotator struct {
	output    io.Writer
	layout    TextLayout
	annotator TLADeclarationAnnotator
}

func NewPrettyTLAWriterWithAnnotator(
	output io.Writer,
	layout TextLayout,
	annotator TLADeclarationAnnotator,
) *PrettyTLAWriterWithAnnotator {
	if annotator == nil {
		annotator = NoTLADeclarationAnnotations{}
	}
	return &PrettyTLAWriterWithAnnotator{output: output, layout: layout, annotator: annotator}
}

func (writer *PrettyTLAWriterWithAnnotator) formatDeclaration(declaration Decl) (string, error) {
	annotations, found := writer.annotator.Annotate(writer.layout, declaration)
	if !found {
		annotations = nil
	}
	return formatDecoratedTLADeclaration(declaration, append([]string(nil), annotations...))
}

func (writer *PrettyTLAWriterWithAnnotator) WriteModule(module *Module, extendedModules []string) error {
	if writer == nil || writer.output == nil {
		return fmt.Errorf("decorated TLA+ writer has no output")
	}
	formatted, err := formatTLAModuleWith(module, TLAFormatOptions{
		ExtendedModules: append([]string(nil), extendedModules...),
	}, writer.formatDeclaration)
	if err != nil {
		return err
	}
	_, err = io.WriteString(writer.output, formatted)
	return err
}

func (writer *PrettyTLAWriterWithAnnotator) WriteDeclaration(declaration Decl) error {
	if writer == nil || writer.output == nil {
		return fmt.Errorf("decorated TLA+ writer has no output")
	}
	formatted, err := writer.formatDeclaration(declaration)
	if err != nil {
		return err
	}
	_, err = io.WriteString(writer.output, formatted+"\n\n")
	return err
}

func (writer *PrettyTLAWriterWithAnnotator) WriteExpression(expression Expr) error {
	if writer == nil || writer.output == nil {
		return fmt.Errorf("decorated TLA+ writer has no output")
	}
	formatted, err := FormatTLAExpression(expression)
	if err != nil {
		return err
	}
	_, err = io.WriteString(writer.output, formatted)
	return err
}

func (writer *PrettyTLAWriterWithAnnotations) WriteModule(module *Module, extendedModules []string) error {
	if writer == nil || writer.output == nil {
		return fmt.Errorf("annotated TLA+ writer has no output")
	}
	formatted, err := formatTLAModuleWith(module, TLAFormatOptions{
		ExtendedModules: append([]string(nil), extendedModules...),
	}, writer.formatDeclaration)
	if err != nil {
		return err
	}
	_, err = io.WriteString(writer.output, formatted)
	return err
}

func (writer *PrettyTLAWriterWithAnnotations) WriteDeclaration(declaration Decl) error {
	if writer == nil || writer.output == nil {
		return fmt.Errorf("annotated TLA+ writer has no output")
	}
	formatted, err := writer.formatDeclaration(declaration)
	if err != nil {
		return err
	}
	_, err = io.WriteString(writer.output, formatted+"\n\n")
	return err
}

func (writer *PrettyTLAWriterWithAnnotations) WriteExpression(expression Expr) error {
	if writer == nil || writer.output == nil {
		return fmt.Errorf("annotated TLA+ writer has no output")
	}
	formatted, err := FormatTLAExpression(expression)
	if err != nil {
		return err
	}
	_, err = io.WriteString(writer.output, formatted)
	return err
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
	return NewPrettyTLAWriterWithAnnotations(output, factory.Annotations)
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
var _ TLAWriter = (*PrettyTLAWriterWithAnnotations)(nil)
var _ TLAWriter = (*PrettyTLAWriterWithAnnotator)(nil)
var _ TLAWriter = (*JSONTLAWriter)(nil)

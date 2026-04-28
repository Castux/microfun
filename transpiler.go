package main

import (
	"fmt"
	"strings"
)

const MFType = "MFRuntimeValue"

type Transpiler struct {
	Analyzer *Analyzer
	Lines    []string
}

func (t *Transpiler) Emit(line string, values ...any) {
	str := fmt.Sprintf(line, values...)
	t.Lines = append(t.Lines, str)
}

func (t *Transpiler) WriteHeader() {
	t.Emit("package main")
	t.Emit("type " + MFType + " any")
}

func Mangle(name string) string {
	return "mf_" + name
}

func MangleQualified(module string, name string) string {
	return "mf_" + module + "_" + name
}

func (t *Transpiler) DeclareModuleExports(modName string, module *Module) {
	for _, binding := range module.PublicBindings {
		t.Emit("var %s %s", MangleQualified(modName, binding.Name.Value), MFType)
	}
}
func (t *Transpiler) TranspileModule(modName string, module *Module) {

	t.Emit("func mf_init_%s() {", modName)
	defer t.Emit("}")

	for _, binding := range module.PublicBindings {
		t.Emit("%s = %s", MangleQualified(modName, binding.Name.Value), t.TranspileExpression(binding.Expression))
	}
}

func (t *Transpiler) TranspileExpression(expr Expression) string {

	switch e := expr.(type) {
	case *NumberLiteral:
		return fmt.Sprintf("%f", e.Value)
	case *Name:
		definition := t.Analyzer.Definitions[e]
		if binding, ok := definition.(*Binding); ok {
			if module := t.Analyzer.ExportedFrom[binding]; module != nil {
				return MangleQualified(module.Name, e.Value)
			}
		}
		Mangle(e.Value)
	case *QualifiedName:
		return MangleQualified(e.Module, e.Value)
	}

	return "0"

}

func Transpile(program *Program, modules map[string]*Module, analyzer *Analyzer) string {

	transpiler := &Transpiler{
		Analyzer: analyzer,
	}

	transpiler.WriteHeader()

	for _, modName := range program.Imports {
		module := modules[modName.Value]
		transpiler.DeclareModuleExports(modName.Value, module)
	}

	for _, modName := range program.Imports {
		module := modules[modName.Value]
		transpiler.TranspileModule(modName.Value, module)
	}

	return strings.Join(transpiler.Lines, "\n")
}

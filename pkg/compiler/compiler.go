package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/qbradq/m3/pkg/asm"
	"github.com/qbradq/m3/pkg/data"
	"github.com/qbradq/m3/pkg/linker"
	"github.com/qbradq/m3/pkg/obj"
)

// SourceUnit represents an accumulated .m3 source file ready for compilation.
type SourceUnit struct {
	Path    string
	Content string
	AST     *SourceFile
}

// ResolveImport searches for an imported file content and its canonical path.
// If importPath is relative (starts with "./" or "../"), it is resolved relative
// to currentFile's directory.
// Otherwise, it searches the standard library directory (pkg/data/lib).
func ResolveImport(currentFile, importPath string) ([]byte, string, error) {
	normPath := filepath.FromSlash(importPath)

	if strings.HasPrefix(importPath, "./") || strings.HasPrefix(importPath, "../") {
		// Relative import: resolved relative to current file's directory
		baseDir := filepath.Dir(currentFile)
		targetPath := filepath.Clean(filepath.Join(baseDir, normPath))
		content, err := os.ReadFile(targetPath)
		if err == nil {
			return content, targetPath, nil
		}

		// Also check embedded filesystem if resolving within embedded pkg/data/lib path
		normTarget := filepath.ToSlash(targetPath)
		if strings.HasPrefix(normTarget, "pkg/data/lib/") {
			fsPath := strings.TrimPrefix(normTarget, "pkg/data/")
			if c, errFs := data.FS.ReadFile(fsPath); errFs == nil {
				return c, targetPath, nil
			}
		}

		return nil, "", fmt.Errorf("failed to read relative import %q (resolved to %q): %w", importPath, targetPath, err)
	}

	// Non-relative import: search standard library (pkg/data/lib)
	// 1. Check embedded filesystem data.FS
	fsPath := "lib/" + strings.TrimPrefix(filepath.ToSlash(importPath), "/")
	if content, err := data.FS.ReadFile(fsPath); err == nil {
		return content, "pkg/data/" + fsPath, nil
	}

	// 2. Check disk search paths relative to workspace
	searchPaths := []string{
		filepath.Join("pkg", "data", "lib", normPath),
		filepath.Join("lib", normPath),
	}
	for _, sp := range searchPaths {
		if content, err := os.ReadFile(sp); err == nil {
			return content, sp, nil
		}
	}

	// 3. Fallback: check relative to currentFile directory
	baseDir := filepath.Dir(currentFile)
	fallbackPath := filepath.Clean(filepath.Join(baseDir, normPath))
	if content, err := os.ReadFile(fallbackPath); err == nil {
		return content, fallbackPath, nil
	}

	return nil, "", fmt.Errorf("cannot find imported library %q in pkg/data/lib or relative paths", importPath)
}

func resolveAndMergeImports(file *SourceFile, currentFile string, visited map[string]bool) error {
	for _, imp := range file.Imports {
		content, resolvedPath, err := ResolveImport(currentFile, imp.Path)
		if err != nil {
			return err
		}
		if visited[resolvedPath] {
			continue
		}
		visited[resolvedPath] = true

		impLexer := NewLexer(resolvedPath, string(content))
		impParser := NewParser(impLexer)
		impAST, err := impParser.ParseSourceFile()
		if err != nil {
			return fmt.Errorf("failed to parse import %q (%s): %w", imp.Path, resolvedPath, err)
		}

		if err := resolveAndMergeImports(impAST, resolvedPath, visited); err != nil {
			return err
		}

		// Merge compile-time definitions and types from the imported AST into the current file
		for _, decl := range impAST.Decls {
			switch d := decl.(type) {
			case *DefineDecl:
				file.Decls = append(file.Decls, d)
			case *TypeDecl:
				file.Decls = append(file.Decls, d)
			}
		}
	}
	return nil
}

// AccumulateSourceFiles recursively traverses all input .m3 files and their imports,
// safely handling circular references and loops, and returns a list of unique SourceUnits.
func AccumulateSourceFiles(entryFiles []string) ([]*SourceUnit, error) {
	if len(entryFiles) == 0 {
		return nil, fmt.Errorf("no input files provided")
	}

	var units []*SourceUnit
	visited := make(map[string]bool)

	var accumulate func(filePath string, content []byte) error
	accumulate = func(filePath string, content []byte) error {
		cleanPath := filepath.Clean(filePath)
		if visited[cleanPath] {
			return nil
		}
		visited[cleanPath] = true

		if content == nil {
			dataBytes, err := os.ReadFile(cleanPath)
			if err != nil {
				return fmt.Errorf("failed to read source file %q: %w", cleanPath, err)
			}
			content = dataBytes
		}

		srcStr := string(content)
		lexer := NewLexer(cleanPath, srcStr)
		parser := NewParser(lexer)
		astFile, err := parser.ParseSourceFile()
		if err != nil {
			return fmt.Errorf("failed to parse source file %q: %w", cleanPath, err)
		}

		units = append(units, &SourceUnit{
			Path:    cleanPath,
			Content: srcStr,
			AST:     astFile,
		})

		for _, imp := range astFile.Imports {
			impContent, resolvedPath, err := ResolveImport(cleanPath, imp.Path)
			if err != nil {
				return err
			}
			if err := accumulate(resolvedPath, impContent); err != nil {
				return err
			}
		}

		return nil
	}

	for _, file := range entryFiles {
		if err := accumulate(file, nil); err != nil {
			return nil, err
		}
	}

	return units, nil
}

// Build compiles one or more .m3 source files by recursively accumulating imported .m3 files,
// compiling all of them into object files in memory, and linking all object files into the final NES ROM bytes.
func Build(entryFiles []string) ([]byte, error) {
	units, err := AccumulateSourceFiles(entryFiles)
	if err != nil {
		return nil, err
	}

	var objects []*obj.ObjectFile
	for _, unit := range units {
		_, asmCode, err := Compile(unit.Path, unit.Content)
		if err != nil {
			return nil, fmt.Errorf("compilation failed for %q: %w", unit.Path, err)
		}

		objFile, err := asm.Assemble(unit.Path, asmCode)
		if err != nil {
			return nil, fmt.Errorf("assembly failed for %q:\n%s\nerror: %w", unit.Path, asmCode, err)
		}

		objects = append(objects, objFile)
	}

	linkerInst := linker.NewLinker(objects...)
	romData, err := linkerInst.Link()
	if err != nil {
		return nil, fmt.Errorf("linking failed: %w", err)
	}

	return romData, nil
}

// BuildFiles compiles the given input .m3 files and writes the linked NES ROM to outputFile.
func BuildFiles(inputFiles []string, outputFile string) error {
	romData, err := Build(inputFiles)
	if err != nil {
		return err
	}

	if err := os.WriteFile(outputFile, romData, 0644); err != nil {
		return fmt.Errorf("failed to write NES ROM %q: %w", outputFile, err)
	}

	return nil
}

// Compile parses an m3 source string into an AST, resolves imports, and generates the assembly output.
func Compile(filename, source string) (*SourceFile, string, error) {
	lexer := NewLexer(filename, source)
	parser := NewParser(lexer)

	astFile, err := parser.ParseSourceFile()
	if err != nil {
		return nil, "", err
	}

	visited := make(map[string]bool)
	if filename != "" {
		visited[filename] = true
	}

	if err := resolveAndMergeImports(astFile, filename, visited); err != nil {
		return astFile, "", err
	}

	asmCode, err := generateAssembly(astFile)
	if err != nil {
		return astFile, "", err
	}

	return astFile, asmCode, nil
}

// CompileFile reads an input .m3 file, compiles it, and writes the assembly output to outputFile.
func CompileFile(inputFile, outputFile string) error {
	content, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read source file %q: %w", inputFile, err)
	}

	_, asmCode, err := Compile(inputFile, string(content))
	if err != nil {
		return err
	}

	if err := os.WriteFile(outputFile, []byte(asmCode), 0644); err != nil {
		return fmt.Errorf("failed to write assembly file %q: %w", outputFile, err)
	}

	return nil
}

// generateAssembly generates assembly source from the AST.
func generateAssembly(file *SourceFile) (string, error) {
	var sb strings.Builder

	sb.WriteString("; =============================================================================\n")
	sb.WriteString(fmt.Sprintf("; Generated by m3 compiler from %s\n", file.Pos().Filename))
	if file.PackageName != "" {
		sb.WriteString(fmt.Sprintf("; Package: %s\n", file.PackageName))
	}
	sb.WriteString("; =============================================================================\n\n")

	// Imports
	if len(file.Imports) > 0 {
		sb.WriteString("; Imports\n")
		for _, imp := range file.Imports {
			sb.WriteString(fmt.Sprintf("; import %q\n", imp.Path))
		}
		sb.WriteString("\n")
	}

	// Group declarations by category
	var zpVars []*VarDecl
	var ramVars []*VarDecl
	var wramVars []*VarDecl
	var defineDecls []*DefineDecl
	var constDecls []*ConstDecl
	var funcDecls []*FuncDecl

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *VarDecl:
			switch d.Storage {
			case StorageZP:
				zpVars = append(zpVars, d)
			case StorageWRAM:
				wramVars = append(wramVars, d)
			default:
				ramVars = append(ramVars, d)
			}
		case *DefineDecl:
			defineDecls = append(defineDecls, d)
		case *ConstDecl:
			constDecls = append(constDecls, d)
		case *FuncDecl:
			funcDecls = append(funcDecls, d)
		}
	}

	// Compile-time Definitions (.define)
	if len(defineDecls) > 0 {
		sb.WriteString("; Compile-time Definitions\n")
		for _, d := range defineDecls {
			pkg := d.Package
			if pkg == "" {
				pkg = file.PackageName
			}
			name := mangleSymbol(pkg, d.Name)
			if isExported(d.Name) && (d.Pos().Filename == "" || d.Pos().Filename == file.Pos().Filename) {
				sb.WriteString(fmt.Sprintf(".export %s\n", name))
			}
			sb.WriteString(fmt.Sprintf(".define %s %s\n\n", name, formatConstExpr(d.Value, pkg)))
		}
	}

	// Zero page segment
	if len(zpVars) > 0 {
		sb.WriteString(".zp\n")
		for _, v := range zpVars {
			pkg := v.Package
			if pkg == "" {
				pkg = file.PackageName
			}
			name := mangleSymbol(pkg, v.Name)
			if isExported(v.Name) {
				sb.WriteString(fmt.Sprintf(".export %s\n", name))
			}
			sb.WriteString(fmt.Sprintf("%s:\n", name))
			sb.WriteString(fmt.Sprintf("  .res %d\n", varSize(v)))
		}
		sb.WriteString("\n")
	}

	// RAM segment
	if len(ramVars) > 0 {
		sb.WriteString(".ram\n")
		for _, v := range ramVars {
			pkg := v.Package
			if pkg == "" {
				pkg = file.PackageName
			}
			name := mangleSymbol(pkg, v.Name)
			if isExported(v.Name) {
				sb.WriteString(fmt.Sprintf(".export %s\n", name))
			}
			sb.WriteString(fmt.Sprintf("%s:\n", name))
			sb.WriteString(fmt.Sprintf("  .res %d\n", varSize(v)))
		}
		sb.WriteString("\n")
	}

	// WRAM segment
	if len(wramVars) > 0 {
		sb.WriteString(".wram\n")
		for _, v := range wramVars {
			pkg := v.Package
			if pkg == "" {
				pkg = file.PackageName
			}
			name := mangleSymbol(pkg, v.Name)
			if isExported(v.Name) {
				sb.WriteString(fmt.Sprintf(".export %s\n", name))
			}
			sb.WriteString(fmt.Sprintf("%s:\n", name))
			sb.WriteString(fmt.Sprintf("  .res %d\n", varSize(v)))
		}
		sb.WriteString("\n")
	}

	// Constants / PRG-ROM
	if len(constDecls) > 0 {
		sb.WriteString("; Constants and ROM Data\n")
		for _, c := range constDecls {
			if c.Bank != nil {
				if c.Bank.IsAuto {
					sb.WriteString(".bank auto\n")
				} else if num, ok := c.Bank.Value.(*NumberLit); ok {
					sb.WriteString(fmt.Sprintf(".bank %d\n", num.Value))
				}
			} else {
				sb.WriteString(".bank auto\n")
			}
			pkg := c.Package
			if pkg == "" {
				pkg = file.PackageName
			}
			name := mangleSymbol(pkg, c.Name)
			if isExported(c.Name) {
				sb.WriteString(fmt.Sprintf(".export %s\n", name))
			}
			sb.WriteString(fmt.Sprintf("%s:\n", name))
			if strLit, ok := c.Value.(*StringLit); ok {
				sb.WriteString(fmt.Sprintf("  .asciiz %q\n", strLit.Value))
			} else if arrLit, ok := c.Value.(*ArrayLit); ok {
				var items []string
				for _, elem := range arrLit.Elements {
					if n, ok := elem.(*NumberLit); ok {
						items = append(items, fmt.Sprintf("$%02X", n.Value&0xFF))
					} else {
						items = append(items, "0")
					}
				}
				if len(items) > 0 {
					sb.WriteString(fmt.Sprintf("  .byte %s\n", strings.Join(items, ", ")))
				}
			} else if numLit, ok := c.Value.(*NumberLit); ok {
				sb.WriteString(fmt.Sprintf("  .word $%04X\n", numLit.Value))
			} else if incbin, ok := c.Value.(*IncbinExpr); ok {
				sb.WriteString(fmt.Sprintf("  .incbin %q\n", incbin.Path))
			} else if incchr, ok := c.Value.(*IncchrExpr); ok {
				sb.WriteString(fmt.Sprintf("  .incchr %q\n", incchr.Path))
			} else if incpal, ok := c.Value.(*IncpalExpr); ok {
				if incpal.Count != nil {
					sb.WriteString(fmt.Sprintf("  .incpal %q, %s\n", incpal.Path, formatConstExpr(incpal.Count, pkg)))
				} else {
					sb.WriteString(fmt.Sprintf("  .incpal %q\n", incpal.Path))
				}
			}
			sb.WriteString("\n")
		}
	}

	// Functions / Procedures
	if len(funcDecls) > 0 {
		sb.WriteString("; Functions\n")
		for _, f := range funcDecls {
			if f.Bank != nil {
				if f.Bank.IsAuto {
					sb.WriteString(".bank auto\n")
				} else if num, ok := f.Bank.Value.(*NumberLit); ok {
					sb.WriteString(fmt.Sprintf(".bank %d\n", num.Value))
				}
			} else {
				sb.WriteString(".bank auto\n")
			}

			pkg := f.Package
			if pkg == "" {
				pkg = file.PackageName
			}
			name := mangleSymbol(pkg, f.Name)
			if isExported(f.Name) {
				sb.WriteString(fmt.Sprintf(".export %s\n", name))
			}
			sb.WriteString(fmt.Sprintf(".proc %s\n", name))

			// Emit any inline assembly inside the function
			if f.Body != nil {
				emitBodyStmts(&sb, f.Body.Stmts)
			}

			sb.WriteString("  RTS\n")
			sb.WriteString(".endproc\n\n")
		}
	}

	return sb.String(), nil
}

func emitBodyStmts(sb *strings.Builder, stmts []Stmt) {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *AsmStmt:
			sb.WriteString("  ; Inline assembly\n")
			lines := strings.Split(s.Body, "\n")
			for _, line := range lines {
				trimmed := strings.TrimRight(line, " \t\r")
				if trimmed != "" {
					sb.WriteString(fmt.Sprintf("  %s\n", trimmed))
				}
			}
		case *BlockStmt:
			emitBodyStmts(sb, s.Stmts)
		case *IfStmt:
			if s.Then != nil {
				emitBodyStmts(sb, s.Then.Stmts)
			}
			if s.Else != nil {
				if elseBlock, ok := s.Else.(*BlockStmt); ok {
					emitBodyStmts(sb, elseBlock.Stmts)
				}
			}
		case *ForStmt:
			if s.Body != nil {
				emitBodyStmts(sb, s.Body.Stmts)
			}
		}
	}
}

func mangleSymbol(pkg, name string) string {
	if pkg != "" {
		return fmt.Sprintf("_%s_%s", pkg, name)
	}
	return "_" + name
}

func isExported(name string) bool {
	if len(name) == 0 {
		return false
	}
	first := name[0]
	return first >= 'A' && first <= 'Z'
}

func formatConstExpr(expr Expr, defaultPkg string) string {
	if expr == nil {
		return "0"
	}
	switch e := expr.(type) {
	case *NumberLit:
		if e.Raw != "" {
			if strings.HasPrefix(e.Raw, "0x") || strings.HasPrefix(e.Raw, "0X") {
				return "$" + e.Raw[2:]
			}
			if strings.HasPrefix(e.Raw, "0b") || strings.HasPrefix(e.Raw, "0B") {
				return "%" + e.Raw[2:]
			}
			return e.Raw
		}
		return fmt.Sprintf("%d", e.Value)
	case *CharLit:
		return fmt.Sprintf("'%c'", e.Value)
	case *BoolLit:
		if e.Value {
			return "1"
		}
		return "0"
	case *StringLit:
		return fmt.Sprintf("%q", e.Value)
	case *Ident:
		return mangleSymbol(defaultPkg, e.Name)
	case *MemberExpr:
		if targetIdent, ok := e.Target.(*Ident); ok {
			return mangleSymbol(targetIdent.Name, e.Member)
		}
		return mangleSymbol(defaultPkg, e.Member)
	case *ParenExpr:
		return fmt.Sprintf("(%s)", formatConstExpr(e.Expr, defaultPkg))
	case *UnaryExpr:
		opStr := e.Op.String()
		switch e.Op {
		case TokenTilde, TokenCaret:
			opStr = "~"
		case TokenBang:
			opStr = "!"
		case TokenLt:
			opStr = "<"
		case TokenGt:
			opStr = ">"
		case TokenAmp:
			opStr = "^"
		}
		return opStr + formatConstExpr(e.Operand, defaultPkg)
	case *BinaryExpr:
		opStr := e.Op.String()
		switch e.Op {
		case TokenEqEq:
			opStr = "=="
		case TokenBangEq:
			opStr = "!="
		}
		return fmt.Sprintf("%s %s %s", formatConstExpr(e.Left, defaultPkg), opStr, formatConstExpr(e.Right, defaultPkg))
	case *CallExpr:
		if len(e.Args) == 1 {
			return formatConstExpr(e.Args[0], defaultPkg)
		}
		return "0"
	case *IncbinExpr:
		return fmt.Sprintf("incbin(%q)", e.Path)
	case *IncchrExpr:
		return fmt.Sprintf("incchr(%q)", e.Path)
	case *IncpalExpr:
		if e.Count != nil {
			return fmt.Sprintf("incpal(%q, %s)", e.Path, formatConstExpr(e.Count, defaultPkg))
		}
		return fmt.Sprintf("incpal(%q)", e.Path)
	default:
		return "0"
	}
}

func varSize(v *VarDecl) int {
	if v == nil {
		return 1
	}
	if arr, ok := v.Type.(*ArrayType); ok && arr.Length == nil && v.Init != nil {
		return exprSize(v.Init, typeSize(arr.Elem))
	}
	return typeSize(v.Type)
}

func exprSize(expr Expr, elemSize int) int {
	if expr == nil {
		return elemSize
	}
	switch e := expr.(type) {
	case *IncpalExpr:
		if e.Count != nil {
			if num, ok := e.Count.(*NumberLit); ok && num.Value > 0 {
				return int(num.Value)
			}
		}
		return 4
	case *ArrayLit:
		return len(e.Elements) * elemSize
	case *StringLit:
		return len(e.Value)
	default:
		return elemSize
	}
}

func typeSize(t TypeSpec) int {
	if t == nil {
		return 1
	}
	switch typ := t.(type) {
	case *PointerType:
		return 2
	case *NamedType:
		switch typ.Name {
		case "uint8", "int8", "bool", "byte":
			return 1
		case "uint16", "int16":
			return 2
		case "uint32", "int32":
			return 4
		default:
			return 1
		}
	case *ArrayType:
		elemSize := typeSize(typ.Elem)
		if typ.Length != nil {
			if num, ok := typ.Length.(*NumberLit); ok && num.Value > 0 {
				return elemSize * int(num.Value)
			}
		}
		return elemSize
	default:
		return 1
	}
}

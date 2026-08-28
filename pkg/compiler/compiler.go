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

func resolveAndMergeImports(file *SourceFile, currentFile string, visited map[string]bool, funcMap map[string]*FuncDecl, declMap map[string]Decl) error {
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

		if err := resolveAndMergeImports(impAST, resolvedPath, visited, funcMap, declMap); err != nil {
			return err
		}

		// Merge compile-time definitions, types, and function signatures from the imported AST into the current file
		for _, decl := range impAST.Decls {
			switch d := decl.(type) {
			case *DefineDecl:
				file.Decls = append(file.Decls, d)
				pkg := d.Package
				if pkg == "" {
					pkg = impAST.PackageName
				}
				if pkg != "" && declMap != nil {
					declMap[pkg+"."+d.Name] = d
				}
				if pkg == file.PackageName && declMap != nil {
					declMap[d.Name] = d
				}
			case *TypeDecl:
				file.Decls = append(file.Decls, d)
			case *DataDecl:
				pkg := d.Package
				if pkg == "" {
					pkg = impAST.PackageName
				}
				if pkg != "" && declMap != nil {
					declMap[pkg+"."+d.Name] = d
				}
				if pkg == file.PackageName && declMap != nil {
					declMap[d.Name] = d
				}
			case *ConstDecl:
				pkg := d.Package
				if pkg == "" {
					pkg = impAST.PackageName
				}
				if pkg != "" && declMap != nil {
					declMap[pkg+"."+d.Name] = d
				}
				if pkg == file.PackageName && declMap != nil {
					declMap[d.Name] = d
				}
			case *VarDecl:
				pkg := d.Package
				if pkg == "" {
					pkg = impAST.PackageName
				}
				if pkg != "" && declMap != nil {
					declMap[pkg+"."+d.Name] = d
				}
				if pkg == file.PackageName && declMap != nil {
					declMap[d.Name] = d
				}
			case *FuncDecl:
				pkg := d.Package
				if pkg == "" {
					pkg = impAST.PackageName
				}
				if pkg != "" {
					funcMap[pkg+"."+d.Name] = d
				}
				if pkg == file.PackageName {
					funcMap[d.Name] = d
				}
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

// BuildOptions provides configuration options for BuildWithOptions and BuildFilesWithOptions.
type BuildOptions struct {
	Debug bool // When true, generates Mesen-compatible .mlb debug symbols.
}

// BuildWithOptions compiles entryFiles and resolves imports, links in memory, and returns ROM bytes and Mesen debug symbols.
func BuildWithOptions(entryFiles []string, opts BuildOptions) ([]byte, string, error) {
	units, err := AccumulateSourceFiles(entryFiles)
	if err != nil {
		return nil, "", err
	}

	var objects []*obj.ObjectFile
	for _, unit := range units {
		_, asmCode, err := Compile(unit.Path, unit.Content)
		if err != nil {
			return nil, "", fmt.Errorf("compilation failed for %q: %w", unit.Path, err)
		}

		objFile, err := asm.Assemble(unit.Path, asmCode)
		if err != nil {
			return nil, "", fmt.Errorf("assembly failed for %q:\n%s\nerror: %w", unit.Path, asmCode, err)
		}

		objects = append(objects, objFile)
	}

	linkerInst := linker.NewLinker(objects...)
	romData, err := linkerInst.Link()
	if err != nil {
		return nil, "", fmt.Errorf("linking failed: %w", err)
	}

	var symbols string
	if opts.Debug {
		symbols = linkerInst.GenerateMesenSymbols()
	}

	return romData, symbols, nil
}

// Build compiles one or more .m3 source files by recursively accumulating imported .m3 files,
// compiling all of them into object files in memory, and linking all object files into the final NES ROM bytes.
func Build(entryFiles []string) ([]byte, error) {
	romData, _, err := BuildWithOptions(entryFiles, BuildOptions{})
	return romData, err
}

// BuildFilesWithOptions compiles inputFiles, writes the linked NES ROM to outputFile, and optionally writes debug symbols to <outputFile_base>.mlb.
func BuildFilesWithOptions(inputFiles []string, outputFile string, opts BuildOptions) error {
	romData, symbols, err := BuildWithOptions(inputFiles, opts)
	if err != nil {
		return err
	}

	if err := os.WriteFile(outputFile, romData, 0644); err != nil {
		return fmt.Errorf("failed to write NES ROM %q: %w", outputFile, err)
	}

	if opts.Debug {
		debugPath := strings.TrimSuffix(outputFile, filepath.Ext(outputFile)) + ".mlb"
		if err := os.WriteFile(debugPath, []byte(symbols), 0644); err != nil {
			return fmt.Errorf("failed to write debug symbol file %q: %w", debugPath, err)
		}
	}

	return nil
}

// BuildFiles compiles the given input .m3 files and writes the linked NES ROM to outputFile.
func BuildFiles(inputFiles []string, outputFile string) error {
	return BuildFilesWithOptions(inputFiles, outputFile, BuildOptions{})
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

	funcMap := make(map[string]*FuncDecl)
	declMap := make(map[string]Decl)
	if err := resolveAndMergeImports(astFile, filename, visited, funcMap, declMap); err != nil {
		return astFile, "", err
	}

	for _, decl := range astFile.Decls {
		switch d := decl.(type) {
		case *FuncDecl:
			pkg := d.Package
			if pkg == "" {
				pkg = astFile.PackageName
			}
			if pkg != "" {
				funcMap[pkg+"."+d.Name] = d
			}
			funcMap[d.Name] = d
		case *DataDecl:
			declMap[d.Name] = d
			if astFile.PackageName != "" {
				declMap[astFile.PackageName+"."+d.Name] = d
			}
		case *ConstDecl:
			declMap[d.Name] = d
			if astFile.PackageName != "" {
				declMap[astFile.PackageName+"."+d.Name] = d
			}
		case *VarDecl:
			declMap[d.Name] = d
			if astFile.PackageName != "" {
				declMap[astFile.PackageName+"."+d.Name] = d
			}
		case *DefineDecl:
			declMap[d.Name] = d
			if astFile.PackageName != "" {
				declMap[astFile.PackageName+"."+d.Name] = d
			}
		}
	}

	if err := validateAST(astFile, funcMap); err != nil {
		return astFile, "", err
	}

	asmCode, err := generateAssembly(astFile, funcMap, declMap)
	if err != nil {
		return astFile, "", err
	}

	return astFile, asmCode, nil
}

func isPrimitiveType(name string) bool {
	switch name {
	case "uint8", "uint16", "uint32", "int8", "int16", "int32", "bool", "byte", "word":
		return true
	default:
		return false
	}
}

func validateAST(file *SourceFile, funcMap map[string]*FuncDecl) error {
	var curFunc *FuncDecl
	var walkExpr func(e Expr) error
	var walkStmt func(s Stmt) error

	walkExpr = func(e Expr) error {
		if e == nil {
			return nil
		}
		switch node := e.(type) {
		case *CallExpr:
			if err := checkCallExpr(node, file.PackageName, funcMap); err != nil {
				return err
			}
			if err := walkExpr(node.Func); err != nil {
				return err
			}
			for _, arg := range node.Args {
				if err := walkExpr(arg); err != nil {
					return err
				}
			}
		case *ArrayLit:
			if err := walkExpr(node.Length); err != nil {
				return err
			}
			for _, el := range node.Elements {
				if err := walkExpr(el); err != nil {
					return err
				}
			}
		case *UnaryExpr:
			return walkExpr(node.Operand)
		case *BinaryExpr:
			if err := walkExpr(node.Left); err != nil {
				return err
			}
			return walkExpr(node.Right)
		case *ParenExpr:
			return walkExpr(node.Expr)
		case *IndexExpr:
			if err := walkExpr(node.Array); err != nil {
				return err
			}
			return walkExpr(node.Index)
		case *MemberExpr:
			return walkExpr(node.Target)
		case *IncpalExpr:
			return walkExpr(node.Count)
		case *StructLit:
			for _, f := range node.Fields {
				if err := walkExpr(f.Value); err != nil {
					return err
				}
			}
		}
		return nil
	}

	walkStmt = func(s Stmt) error {
		if s == nil {
			return nil
		}
		switch node := s.(type) {
		case *BlockStmt:
			for _, inner := range node.Stmts {
				if err := walkStmt(inner); err != nil {
					return err
				}
			}
		case *VarDeclStmt:
			if err := walkExpr(node.Decl.Length); err != nil {
				return err
			}
			return walkExpr(node.Decl.Init)
		case *ConstDeclStmt:
			if err := walkExpr(node.Decl.Length); err != nil {
				return err
			}
			return walkExpr(node.Decl.Value)
		case *DataDeclStmt:
			if err := walkExpr(node.Decl.Length); err != nil {
				return err
			}
			return walkExpr(node.Decl.Value)
		case *DefineDeclStmt:
			return walkExpr(node.Decl.Value)
		case *AssignStmt:
			if ident, ok := node.Left.(*Ident); ok && curFunc != nil {
				for _, p := range curFunc.Params {
					if p.Name == ident.Name {
						return fmt.Errorf("%s:%d:%d: cannot assign to parameter %q (function parameters are read-only)",
							node.Pos().Filename, node.Pos().Line, node.Pos().Column, ident.Name)
					}
				}
			}
			if err := walkExpr(node.Left); err != nil {
				return err
			}
			return walkExpr(node.Right)
		case *IncDecStmt:
			if ident, ok := node.Expr.(*Ident); ok && curFunc != nil {
				for _, p := range curFunc.Params {
					if p.Name == ident.Name {
						return fmt.Errorf("%s:%d:%d: cannot assign to parameter %q (function parameters are read-only)",
							node.Pos().Filename, node.Pos().Line, node.Pos().Column, ident.Name)
					}
				}
			}
			return walkExpr(node.Expr)
		case *ShortVarDeclStmt:
			return walkExpr(node.Value)
		case *IfStmt:
			if err := walkStmt(node.Init); err != nil {
				return err
			}
			if err := walkExpr(node.Cond); err != nil {
				return err
			}
			if err := walkStmt(node.Then); err != nil {
				return err
			}
			return walkStmt(node.Else)
		case *ForStmt:
			if err := walkStmt(node.Init); err != nil {
				return err
			}
			if err := walkExpr(node.Cond); err != nil {
				return err
			}
			if err := walkStmt(node.Post); err != nil {
				return err
			}
			return walkStmt(node.Body)
		case *SwitchStmt:
			if err := walkExpr(node.Expr); err != nil {
				return err
			}
			for _, cc := range node.Cases {
				for _, val := range cc.Values {
					if err := walkExpr(val); err != nil {
						return err
					}
				}
				for _, inner := range cc.Body {
					if err := walkStmt(inner); err != nil {
						return err
					}
				}
			}
		case *ReturnStmt:
			return walkExpr(node.Value)
		case *ExprStmt:
			return walkExpr(node.Expr)
		}
		return nil
	}

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *VarDecl:
			if err := walkExpr(d.Length); err != nil {
				return err
			}
			if err := walkExpr(d.Init); err != nil {
				return err
			}
		case *ConstDecl:
			if err := walkExpr(d.Length); err != nil {
				return err
			}
			if err := walkExpr(d.Value); err != nil {
				return err
			}
		case *DataDecl:
			if err := walkExpr(d.Length); err != nil {
				return err
			}
			if err := walkExpr(d.Value); err != nil {
				return err
			}
		case *DefineDecl:
			if err := walkExpr(d.Value); err != nil {
				return err
			}
		case *FuncDecl:
			curFunc = d
			if d.Body != nil {
				if err := walkStmt(d.Body); err != nil {
					return err
				}
			}
			curFunc = nil
		}
	}

	return nil
}

func checkCallExpr(call *CallExpr, defaultPkg string, funcMap map[string]*FuncDecl) error {
	var funcPkg string
	var funcName string
	var fullName string

	if ident, ok := call.Func.(*Ident); ok {
		if isPrimitiveType(ident.Name) {
			if len(call.Args) != 1 {
				return fmt.Errorf("%s:%d:%d: type conversion to %s requires exactly 1 argument, got %d",
					call.Pos().Filename, call.Pos().Line, call.Pos().Column, ident.Name, len(call.Args))
			}
			return nil
		}
		funcPkg = defaultPkg
		funcName = ident.Name
		fullName = ident.Name
	} else if mem, ok := call.Func.(*MemberExpr); ok {
		if targetIdent, ok := mem.Target.(*Ident); ok {
			funcPkg = targetIdent.Name
			funcName = mem.Member
			fullName = fmt.Sprintf("%s.%s", funcPkg, funcName)
		}
	}

	if funcName != "" {
		var targetFn *FuncDecl
		if funcPkg != "" {
			targetFn = funcMap[funcPkg+"."+funcName]
		}
		if targetFn == nil {
			targetFn = funcMap[funcName]
		}

		if targetFn != nil {
			expected := len(targetFn.Params)
			actual := len(call.Args)
			if actual < expected {
				return fmt.Errorf("%s:%d:%d: too few arguments in call to %s (expected %d, got %d)",
					call.Pos().Filename, call.Pos().Line, call.Pos().Column, fullName, expected, actual)
			}
			if actual > expected {
				return fmt.Errorf("%s:%d:%d: too many arguments in call to %s (expected %d, got %d)",
					call.Pos().Filename, call.Pos().Line, call.Pos().Column, fullName, expected, actual)
			}
		}
	}

	return nil
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

func isLeafFunction(f *FuncDecl) bool {
	if f == nil || f.Body == nil {
		return true
	}
	return !containsCallExpr(f.Body)
}

func containsCallExpr(node Node) bool {
	if node == nil {
		return false
	}
	switch n := node.(type) {
	case *CallExpr:
		if ident, ok := n.Func.(*Ident); ok && isPrimitiveType(ident.Name) {
			for _, arg := range n.Args {
				if containsCallExpr(arg) {
					return true
				}
			}
			return false
		}
		return true
	case *BlockStmt:
		for _, s := range n.Stmts {
			if containsCallExpr(s) {
				return true
			}
		}
	case *AssignStmt:
		return containsCallExpr(n.Left) || containsCallExpr(n.Right)
	case *IncDecStmt:
		return containsCallExpr(n.Expr)
	case *ShortVarDeclStmt:
		return containsCallExpr(n.Value)
	case *IfStmt:
		if containsCallExpr(n.Init) || containsCallExpr(n.Cond) || containsCallExpr(n.Then) || containsCallExpr(n.Else) {
			return true
		}
	case *ForStmt:
		if containsCallExpr(n.Init) || containsCallExpr(n.Cond) || containsCallExpr(n.Post) || containsCallExpr(n.Body) {
			return true
		}
	case *SwitchStmt:
		if containsCallExpr(n.Expr) {
			return true
		}
		for _, cc := range n.Cases {
			for _, v := range cc.Values {
				if containsCallExpr(v) {
					return true
				}
			}
			for _, s := range cc.Body {
				if containsCallExpr(s) {
					return true
				}
			}
		}
	case *ReturnStmt:
		return containsCallExpr(n.Value)
	case *ExprStmt:
		return containsCallExpr(n.Expr)
	case *UnaryExpr:
		return containsCallExpr(n.Operand)
	case *BinaryExpr:
		return containsCallExpr(n.Left) || containsCallExpr(n.Right)
	case *ParenExpr:
		return containsCallExpr(n.Expr)
	case *IndexExpr:
		return containsCallExpr(n.Array) || containsCallExpr(n.Index)
	case *MemberExpr:
		return containsCallExpr(n.Target)
	case *ArrayLit:
		for _, el := range n.Elements {
			if containsCallExpr(el) {
				return true
			}
		}
	case *StructLit:
		for _, f := range n.Fields {
			if containsCallExpr(f.Value) {
				return true
			}
		}
	}
	return false
}

func isAsmOnly(body *BlockStmt) bool {
	if body == nil || len(body.Stmts) == 0 {
		return false
	}
	for _, s := range body.Stmts {
		if _, ok := s.(*AsmStmt); !ok {
			return false
		}
	}
	return true
}

type ParamLocType int

const (
	ParamLocRegA ParamLocType = iota
	ParamLocRegX
	ParamLocRegY
	ParamLocRegAX
	ParamLocRegXY
	ParamLocMemory
)

type ParamLocation struct {
	Param   *Param
	LocType ParamLocType
	MemSym  string
	Offset  int
	Size    int
}

func computeParamLocations(f *FuncDecl, defaultPkg string) []ParamLocation {
	pkg := f.Package
	if pkg == "" {
		pkg = defaultPkg
	}
	isLeaf := isLeafFunction(f)
	var locs []ParamLocation
	usedSlots := 0
	excessOffset := 0

	for _, p := range f.Params {
		sz := typeSize(p.Type)
		assignedReg := false

		if sz == 1 && usedSlots < 3 {
			switch usedSlots {
			case 0:
				locs = append(locs, ParamLocation{Param: p, LocType: ParamLocRegA, Size: 1})
				usedSlots = 1
				assignedReg = true
			case 1:
				locs = append(locs, ParamLocation{Param: p, LocType: ParamLocRegX, Size: 1})
				usedSlots = 2
				assignedReg = true
			case 2:
				locs = append(locs, ParamLocation{Param: p, LocType: ParamLocRegY, Size: 1})
				usedSlots = 3
				assignedReg = true
			}
		} else if sz == 2 {
			if usedSlots == 0 {
				locs = append(locs, ParamLocation{Param: p, LocType: ParamLocRegAX, Size: 2})
				usedSlots = 2
				assignedReg = true
			} else if usedSlots == 1 {
				locs = append(locs, ParamLocation{Param: p, LocType: ParamLocRegXY, Size: 2})
				usedSlots = 3
				assignedReg = true
			}
		}

		if !assignedReg {
			usedSlots = 3
			var memSym string
			if isLeaf {
				memSym = fmt.Sprintf("__leaf_param%d", excessOffset)
			} else {
				memSym = fmt.Sprintf("_%s_%s_%s", pkg, f.Name, p.Name)
			}
			locs = append(locs, ParamLocation{
				Param:   p,
				LocType: ParamLocMemory,
				MemSym:  memSym,
				Offset:  excessOffset,
				Size:    sz,
			})
			excessOffset += sz
		}
	}
	return locs
}

// generateAssembly generates assembly source from the AST.
func generateAssembly(file *SourceFile, funcMap map[string]*FuncDecl, declMap map[string]Decl) (string, error) {
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

	// Collect structs
	structMap := make(map[string]*StructType)
	for _, decl := range file.Decls {
		if td, ok := decl.(*TypeDecl); ok {
			if st, ok := td.Type.(*StructType); ok {
				structMap[td.Name] = st
			}
		}
	}

	// Group declarations by category
	var zpVars []*VarDecl
	var ramVars []*VarDecl
	var wramVars []*VarDecl
	var defineDecls []*DefineDecl
	var dataDecls []*DataDecl
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
		case *DataDecl:
			dataDecls = append(dataDecls, d)
		case *ConstDecl:
			constDecls = append(constDecls, d)
		case *FuncDecl:
			funcDecls = append(funcDecls, d)
		}
	}

	// Allocate dedicated RAM for excess parameters in non-leaf functions
	for _, f := range funcDecls {
		if !isLeafFunction(f) {
			locs := computeParamLocations(f, file.PackageName)
			for _, loc := range locs {
				if loc.LocType == ParamLocMemory {
					pkg := f.Package
					if pkg == "" {
						pkg = file.PackageName
					}
					ramVars = append(ramVars, &VarDecl{
						Package: pkg,
						Name:    fmt.Sprintf("%s_%s", f.Name, loc.Param.Name),
						Type:    loc.Param.Type,
						Storage: StorageRAM,
					})
				}
			}
		}
	}

	// Collect local variables from ShortVarDeclStmt across functions
	localVars := make(map[string]*VarDecl)
	for _, f := range funcDecls {
		if f.Body != nil {
			collectLocalVars(f.Body.Stmts, f, file.PackageName, localVars)
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
	if len(zpVars) > 0 || len(localVars) > 0 {
		sb.WriteString(".zp\n")
		for _, v := range zpVars {
			emitVarDecl(&sb, v, file.PackageName, structMap)
		}
		for _, lv := range localVars {
			emitVarDecl(&sb, lv, file.PackageName, structMap)
		}
		sb.WriteString("\n")
	}

	// RAM segment
	if len(ramVars) > 0 {
		sb.WriteString(".ram\n")
		for _, v := range ramVars {
			emitVarDecl(&sb, v, file.PackageName, structMap)
		}
		sb.WriteString("\n")
	}

	// WRAM segment
	if len(wramVars) > 0 {
		sb.WriteString(".wram\n")
		for _, v := range wramVars {
			emitVarDecl(&sb, v, file.PackageName, structMap)
		}
		sb.WriteString("\n")
	}

	// Banked Data (Relocated to $8000-$9FFF)
	if len(dataDecls) > 0 {
		sb.WriteString("; Banked Data (Relocated to $8000-$9FFF)\n")
		for _, d := range dataDecls {
			pkg := d.Package
			if pkg == "" {
				pkg = file.PackageName
			}
			emitBank(&sb, d.Bank, pkg)
			sb.WriteString(".data\n")
			if is2DDecl(d.Type, d.Value) {
				if err := emit2DDataOrConstDecl(&sb, d.Name, pkg, isExported(d.Name), d.Type, d.Length, d.Value, structMap); err != nil {
					return "", err
				}
			} else {
				name := mangleSymbol(pkg, d.Name)
				if isExported(d.Name) {
					sb.WriteString(fmt.Sprintf(".export %s\n", name))
				}
				sb.WriteString(fmt.Sprintf("%s:\n", name))
				if err := emitConstDataDeclValue(&sb, d.Value, d.Type, structMap, pkg); err != nil {
					return "", err
				}
			}
			sb.WriteString("\n")
		}
	}

	// Constants / PRG-ROM (Relocated to $A000-$BFFF)
	if len(constDecls) > 0 {
		sb.WriteString("; Constants and ROM Data\n")
		for _, c := range constDecls {
			pkg := c.Package
			if pkg == "" {
				pkg = file.PackageName
			}
			emitBank(&sb, c.Bank, pkg)
			sb.WriteString(".code\n")
			if is2DDecl(c.Type, c.Value) {
				if err := emit2DDataOrConstDecl(&sb, c.Name, pkg, isExported(c.Name), c.Type, c.Length, c.Value, structMap); err != nil {
					return "", err
				}
			} else {
				name := mangleSymbol(pkg, c.Name)
				if isExported(c.Name) {
					sb.WriteString(fmt.Sprintf(".export %s\n", name))
				}
				sb.WriteString(fmt.Sprintf("%s:\n", name))
				if err := emitConstDataDeclValue(&sb, c.Value, c.Type, structMap, pkg); err != nil {
					return "", err
				}
			}
			sb.WriteString("\n")
		}
	}

	// Functions / Procedures (Relocated to $A000-$BFFF)
	if len(funcDecls) > 0 {
		sb.WriteString("; Functions\n")
		cg := newCodeGenerator(file, structMap, localVars, funcMap, declMap)

		for _, f := range funcDecls {
			pkg := f.Package
			if pkg == "" {
				pkg = file.PackageName
			}
			emitBank(&sb, f.Bank, pkg)
			sb.WriteString(".code\n")
			name := mangleSymbol(pkg, f.Name)
			if isExported(f.Name) {
				sb.WriteString(fmt.Sprintf(".export %s\n", name))
			}
			sb.WriteString(fmt.Sprintf(".proc %s\n", name))

			cg.curFunc = f
			cg.paramLocs = make(map[string]ParamLocation)
			locs := computeParamLocations(f, file.PackageName)
			isLeaf := isLeafFunction(f)
			excessOffset := 0
			for _, loc := range locs {
				if loc.LocType == ParamLocMemory {
					excessOffset += loc.Size
				}
			}
			saveOffset := excessOffset
			for _, loc := range locs {
				if loc.LocType != ParamLocMemory {
					if isLeaf {
						loc.MemSym = fmt.Sprintf("__leaf_param%d", saveOffset)
						saveOffset += loc.Size
					} else {
						loc.MemSym = fmt.Sprintf("_%s_%s_%s", pkg, f.Name, loc.Param.Name)
					}
				}
				cg.paramLocs[loc.Param.Name] = loc
			}

			// Emit register parameter save prologue for M3 function bodies (not asm-only)
			if f.Body != nil && len(f.Params) > 0 && !isAsmOnly(f.Body) {
				for _, loc := range locs {
					switch loc.LocType {
					case ParamLocRegA:
						sb.WriteString(fmt.Sprintf("  STA %s\n", loc.MemSym))
					case ParamLocRegX:
						sb.WriteString(fmt.Sprintf("  STX %s\n", loc.MemSym))
					case ParamLocRegY:
						sb.WriteString(fmt.Sprintf("  STY %s\n", loc.MemSym))
					case ParamLocRegAX:
						sb.WriteString(fmt.Sprintf("  STA %s\n  STX %s+1\n", loc.MemSym, loc.MemSym))
					case ParamLocRegXY:
						sb.WriteString(fmt.Sprintf("  STX %s\n  STY %s+1\n", loc.MemSym, loc.MemSym))
					}
				}
			}

			if f.Body != nil {
				for _, stmt := range f.Body.Stmts {
					cg.compileStmt(&sb, stmt)
				}
			}

			sb.WriteString("  RTS\n")
			sb.WriteString(".endproc\n\n")
		}
	}

	return sb.String(), nil
}

func collectLocalVars(stmts []Stmt, f *FuncDecl, defaultPkg string, localVars map[string]*VarDecl) {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ShortVarDeclStmt:
			mangled := fmt.Sprintf("%s_%s", f.Name, s.Name)
			if _, exists := localVars[mangled]; !exists {
				localVars[mangled] = &VarDecl{
					Package: defaultPkg,
					Name:    mangled,
					Type:    &NamedType{Name: "uint8"},
					Storage: StorageZP,
				}
			}
		case *BlockStmt:
			collectLocalVars(s.Stmts, f, defaultPkg, localVars)
		case *IfStmt:
			if s.Init != nil {
				collectLocalVars([]Stmt{s.Init}, f, defaultPkg, localVars)
			}
			if s.Then != nil {
				collectLocalVars(s.Then.Stmts, f, defaultPkg, localVars)
			}
			if s.Else != nil {
				if eb, ok := s.Else.(*BlockStmt); ok {
					collectLocalVars(eb.Stmts, f, defaultPkg, localVars)
				} else if ei, ok := s.Else.(*IfStmt); ok {
					collectLocalVars([]Stmt{ei}, f, defaultPkg, localVars)
				}
			}
		case *ForStmt:
			if s.Init != nil {
				collectLocalVars([]Stmt{s.Init}, f, defaultPkg, localVars)
			}
			if s.Body != nil {
				collectLocalVars(s.Body.Stmts, f, defaultPkg, localVars)
			}
			if s.Post != nil {
				collectLocalVars([]Stmt{s.Post}, f, defaultPkg, localVars)
			}
		}
	}
}

func emitVarDecl(sb *strings.Builder, v *VarDecl, defaultPkg string, structMap map[string]*StructType) {
	pkg := v.Package
	if pkg == "" {
		pkg = defaultPkg
	}

	// Check if this variable is a Struct or Array of Structs (SoA Striping)
	var structName string
	var arrayCount int = 1
	var isStruct bool

	if nt, ok := v.Type.(*NamedType); ok {
		if _, ok := structMap[nt.Name]; ok {
			structName = nt.Name
			isStruct = true
		}
	} else if arr, ok := v.Type.(*ArrayType); ok {
		if nt, ok := arr.Elem.(*NamedType); ok {
			if _, ok := structMap[nt.Name]; ok {
				structName = nt.Name
				isStruct = true
				if num, ok := arr.Length.(*NumberLit); ok {
					arrayCount = int(num.Value)
				}
			}
		}
	}

	if isStruct {
		st := structMap[structName]
		for _, field := range st.Fields {
			fieldName := field.Name
			fieldSize := typeSize(field.Type) * arrayCount
			mangled := mangleSymbol(pkg, fmt.Sprintf("%s_%s", v.Name, fieldName))
			if isExported(v.Name) {
				sb.WriteString(fmt.Sprintf(".export %s\n", mangled))
			}
			sb.WriteString(fmt.Sprintf("%s:\n", mangled))
			sb.WriteString(fmt.Sprintf("  .res %d\n", fieldSize))
		}
		return
	}

	name := mangleSymbol(pkg, v.Name)
	if isExported(v.Name) {
		sb.WriteString(fmt.Sprintf(".export %s\n", name))
	}
	sb.WriteString(fmt.Sprintf("%s:\n", name))
	sb.WriteString(fmt.Sprintf("  .res %d\n", varSize(v)))
}

type loopInfo struct {
	headLabel string
	postLabel string
	exitLabel string
}

type codeGenerator struct {
	file       *SourceFile
	structMap  map[string]*StructType
	localVars  map[string]*VarDecl
	funcMap    map[string]*FuncDecl
	declMap    map[string]Decl
	curFunc    *FuncDecl
	paramLocs  map[string]ParamLocation
	labelCount int
	loopStack  []loopInfo
}

func newCodeGenerator(file *SourceFile, structMap map[string]*StructType, localVars map[string]*VarDecl, funcMap map[string]*FuncDecl, declMap map[string]Decl) *codeGenerator {
	return &codeGenerator{
		file:      file,
		structMap: structMap,
		localVars: localVars,
		funcMap:   funcMap,
		declMap:   declMap,
		paramLocs: make(map[string]ParamLocation),
	}
}

func (cg *codeGenerator) newLabel(prefix string) string {
	cg.labelCount++
	return fmt.Sprintf("@%s_%d", prefix, cg.labelCount)
}

func (cg *codeGenerator) compileStmt(sb *strings.Builder, stmt Stmt) {
	if stmt == nil {
		return
	}

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
		for _, inner := range s.Stmts {
			cg.compileStmt(sb, inner)
		}

	case *AssignStmt:
		cg.compileAssignStmt(sb, s)

	case *IncDecStmt:
		cg.compileIncDecStmt(sb, s)

	case *ShortVarDeclStmt:
		cg.compileShortVarDeclStmt(sb, s)

	case *IfStmt:
		cg.compileIfStmt(sb, s)

	case *ForStmt:
		cg.compileForStmt(sb, s)

	case *SwitchStmt:
		cg.compileSwitchStmt(sb, s)

	case *ReturnStmt:
		cg.compileReturnStmt(sb, s)

	case *BreakStmt:
		if len(cg.loopStack) > 0 {
			top := cg.loopStack[len(cg.loopStack)-1]
			sb.WriteString(fmt.Sprintf("  JMP %s\n", top.exitLabel))
		}

	case *ContinueStmt:
		if len(cg.loopStack) > 0 {
			top := cg.loopStack[len(cg.loopStack)-1]
			sb.WriteString(fmt.Sprintf("  JMP %s\n", top.postLabel))
		}

	case *ExprStmt:
		if call, ok := s.Expr.(*CallExpr); ok {
			cg.compileCallExpr(sb, call)
		}
	}
}

func (cg *codeGenerator) compileAssignStmt(sb *strings.Builder, s *AssignStmt) {
	// 1. Array-of-Struct member access: enemies[i].x = expr
	if mem, ok := s.Left.(*MemberExpr); ok {
		if idx, ok := mem.Target.(*IndexExpr); ok {
			if ident, ok := idx.Array.(*Ident); ok {
				// Index is in idx.Index
				cg.evalExprIntoX(sb, idx.Index)
				targetSym := mangleSymbol(cg.file.PackageName, fmt.Sprintf("%s_%s", ident.Name, mem.Member))

				switch s.Op {
				case TokenEq:
					cg.evalExprIntoA(sb, s.Right)
					sb.WriteString(fmt.Sprintf("  STA %s, X\n", targetSym))
				case TokenPlusEq:
					sb.WriteString(fmt.Sprintf("  LDA %s, X\n", targetSym))
					sb.WriteString("  CLC\n")
					cg.emitAddA(sb, s.Right)
					sb.WriteString(fmt.Sprintf("  STA %s, X\n", targetSym))
				case TokenMinusEq:
					sb.WriteString(fmt.Sprintf("  LDA %s, X\n", targetSym))
					sb.WriteString("  SEC\n")
					cg.emitSubA(sb, s.Right)
					sb.WriteString(fmt.Sprintf("  STA %s, X\n", targetSym))
				}
				return
			}
		}
	}

	// 2. Simple array element access: arr[i] = expr
	if idx, ok := s.Left.(*IndexExpr); ok {
		if ident, ok := idx.Array.(*Ident); ok {
			cg.evalExprIntoX(sb, idx.Index)
			targetSym := cg.resolveSymbolName(ident.Name)
			switch s.Op {
			case TokenEq:
				cg.evalExprIntoA(sb, s.Right)
				sb.WriteString(fmt.Sprintf("  STA %s, X\n", targetSym))
			case TokenPlusEq:
				sb.WriteString(fmt.Sprintf("  LDA %s, X\n", targetSym))
				sb.WriteString("  CLC\n")
				cg.emitAddA(sb, s.Right)
				sb.WriteString(fmt.Sprintf("  STA %s, X\n", targetSym))
			case TokenMinusEq:
				sb.WriteString(fmt.Sprintf("  LDA %s, X\n", targetSym))
				sb.WriteString("  SEC\n")
				cg.emitSubA(sb, s.Right)
				sb.WriteString(fmt.Sprintf("  STA %s, X\n", targetSym))
			}
			return
		}
	}

	// 3. Simple variable assignment: v = expr
	if ident, ok := s.Left.(*Ident); ok {
		targetSym := cg.resolveSymbolName(ident.Name)
		switch s.Op {
		case TokenEq:
			cg.evalExprIntoA(sb, s.Right)
			sb.WriteString(fmt.Sprintf("  STA %s\n", targetSym))
		case TokenPlusEq:
			sb.WriteString(fmt.Sprintf("  LDA %s\n", targetSym))
			sb.WriteString("  CLC\n")
			cg.emitAddA(sb, s.Right)
			sb.WriteString(fmt.Sprintf("  STA %s\n", targetSym))
		case TokenMinusEq:
			sb.WriteString(fmt.Sprintf("  LDA %s\n", targetSym))
			sb.WriteString("  SEC\n")
			cg.emitSubA(sb, s.Right)
			sb.WriteString(fmt.Sprintf("  STA %s\n", targetSym))
		case TokenAmpEq:
			sb.WriteString(fmt.Sprintf("  LDA %s\n", targetSym))
			cg.emitAndA(sb, s.Right)
			sb.WriteString(fmt.Sprintf("  STA %s\n", targetSym))
		case TokenPipeEq:
			sb.WriteString(fmt.Sprintf("  LDA %s\n", targetSym))
			cg.emitOrA(sb, s.Right)
			sb.WriteString(fmt.Sprintf("  STA %s\n", targetSym))
		}
		return
	}
}

func (cg *codeGenerator) compileIncDecStmt(sb *strings.Builder, s *IncDecStmt) {
	// Array-of-struct: enemies[i].y++
	if mem, ok := s.Expr.(*MemberExpr); ok {
		if idx, ok := mem.Target.(*IndexExpr); ok {
			if ident, ok := idx.Array.(*Ident); ok {
				cg.evalExprIntoX(sb, idx.Index)
				targetSym := mangleSymbol(cg.file.PackageName, fmt.Sprintf("%s_%s", ident.Name, mem.Member))
				if s.IsInc {
					sb.WriteString(fmt.Sprintf("  INC %s, X\n", targetSym))
				} else {
					sb.WriteString(fmt.Sprintf("  DEC %s, X\n", targetSym))
				}
				return
			}
		}
	}

	// Array element: arr[i]++
	if idx, ok := s.Expr.(*IndexExpr); ok {
		if ident, ok := idx.Array.(*Ident); ok {
			cg.evalExprIntoX(sb, idx.Index)
			targetSym := cg.resolveSymbolName(ident.Name)
			if s.IsInc {
				sb.WriteString(fmt.Sprintf("  INC %s, X\n", targetSym))
			} else {
				sb.WriteString(fmt.Sprintf("  DEC %s, X\n", targetSym))
			}
			return
		}
	}

	// Simple variable
	if ident, ok := s.Expr.(*Ident); ok {
		targetSym := cg.resolveSymbolName(ident.Name)
		// Check if 16-bit variable (like frame_counter)
		is16Bit := false
		for _, decl := range cg.file.Decls {
			if vd, ok := decl.(*VarDecl); ok && vd.Name == ident.Name {
				if nt, ok := vd.Type.(*NamedType); ok && (nt.Name == "uint16" || nt.Name == "int16") {
					is16Bit = true
				}
			}
		}

		if is16Bit {
			if s.IsInc {
				skipLabel := cg.newLabel("skip_inc")
				sb.WriteString(fmt.Sprintf("  INC %s\n", targetSym))
				sb.WriteString(fmt.Sprintf("  BNE %s\n", skipLabel))
				sb.WriteString(fmt.Sprintf("  INC %s+1\n", targetSym))
				sb.WriteString(fmt.Sprintf("%s:\n", skipLabel))
			} else {
				skipLabel := cg.newLabel("skip_dec")
				sb.WriteString(fmt.Sprintf("  LDA %s\n", targetSym))
				sb.WriteString(fmt.Sprintf("  BNE %s\n", skipLabel))
				sb.WriteString(fmt.Sprintf("  DEC %s+1\n", targetSym))
				sb.WriteString(fmt.Sprintf("%s:\n", skipLabel))
				sb.WriteString(fmt.Sprintf("  DEC %s\n", targetSym))
			}
		} else {
			if s.IsInc {
				sb.WriteString(fmt.Sprintf("  INC %s\n", targetSym))
			} else {
				sb.WriteString(fmt.Sprintf("  DEC %s\n", targetSym))
			}
		}
	}
}

func (cg *codeGenerator) compileShortVarDeclStmt(sb *strings.Builder, s *ShortVarDeclStmt) {
	mangled := fmt.Sprintf("%s_%s", cg.curFunc.Name, s.Name)
	targetSym := mangleSymbol(cg.file.PackageName, mangled)
	cg.evalExprIntoA(sb, s.Value)
	sb.WriteString(fmt.Sprintf("  STA %s\n", targetSym))
}

func (cg *codeGenerator) compileIfStmt(sb *strings.Builder, s *IfStmt) {
	if s.Init != nil {
		cg.compileStmt(sb, s.Init)
	}

	elseLabel := cg.newLabel("if_else")
	endLabel := cg.newLabel("if_end")

	hasElse := s.Else != nil
	falseTarget := elseLabel
	if !hasElse {
		falseTarget = endLabel
	}

	cg.compileConditionBranch(sb, s.Cond, falseTarget, false)

	if s.Then != nil {
		for _, stmt := range s.Then.Stmts {
			cg.compileStmt(sb, stmt)
		}
	}

	if hasElse {
		sb.WriteString(fmt.Sprintf("  JMP %s\n", endLabel))
		sb.WriteString(fmt.Sprintf("%s:\n", elseLabel))
		if eb, ok := s.Else.(*BlockStmt); ok {
			for _, stmt := range eb.Stmts {
				cg.compileStmt(sb, stmt)
			}
		} else if ei, ok := s.Else.(*IfStmt); ok {
			cg.compileIfStmt(sb, ei)
		}
	}

	sb.WriteString(fmt.Sprintf("%s:\n", endLabel))
}

func (cg *codeGenerator) compileForStmt(sb *strings.Builder, s *ForStmt) {
	if s.Init != nil {
		cg.compileStmt(sb, s.Init)
	}

	headLabel := cg.newLabel("for_head")
	postLabel := cg.newLabel("for_post")
	exitLabel := cg.newLabel("for_exit")

	cg.loopStack = append(cg.loopStack, loopInfo{
		headLabel: headLabel,
		postLabel: postLabel,
		exitLabel: exitLabel,
	})

	sb.WriteString(fmt.Sprintf("%s:\n", headLabel))

	if s.Cond != nil {
		cg.compileConditionBranch(sb, s.Cond, exitLabel, false)
	}

	if s.Body != nil {
		for _, stmt := range s.Body.Stmts {
			cg.compileStmt(sb, stmt)
		}
	}

	sb.WriteString(fmt.Sprintf("%s:\n", postLabel))
	if s.Post != nil {
		cg.compileStmt(sb, s.Post)
	}

	sb.WriteString(fmt.Sprintf("  JMP %s\n", headLabel))
	sb.WriteString(fmt.Sprintf("%s:\n", exitLabel))

	cg.loopStack = cg.loopStack[:len(cg.loopStack)-1]
}

func (cg *codeGenerator) compileSwitchStmt(sb *strings.Builder, s *SwitchStmt) {
	exitLabel := cg.newLabel("switch_exit")
	cg.evalExprIntoA(sb, s.Expr)

	var defaultClause *CaseClause
	caseLabels := make([]string, len(s.Cases))

	for i, c := range s.Cases {
		if len(c.Values) == 0 {
			defaultClause = c
			caseLabels[i] = cg.newLabel("case_default")
			continue
		}
		caseLabels[i] = cg.newLabel(fmt.Sprintf("case_%d", i))
		for _, val := range c.Values {
			valStr := formatConstExpr(val, cg.file.PackageName)
			sb.WriteString(fmt.Sprintf("  CMP #%s\n", valStr))
			sb.WriteString(fmt.Sprintf("  BEQ %s\n", caseLabels[i]))
		}
	}

	if defaultClause != nil {
		defaultIdx := -1
		for i, c := range s.Cases {
			if c == defaultClause {
				defaultIdx = i
				break
			}
		}
		sb.WriteString(fmt.Sprintf("  JMP %s\n", caseLabels[defaultIdx]))
	} else {
		sb.WriteString(fmt.Sprintf("  JMP %s\n", exitLabel))
	}

	for i, c := range s.Cases {
		sb.WriteString(fmt.Sprintf("%s:\n", caseLabels[i]))
		for _, stmt := range c.Body {
			cg.compileStmt(sb, stmt)
		}
		sb.WriteString(fmt.Sprintf("  JMP %s\n", exitLabel))
	}

	sb.WriteString(fmt.Sprintf("%s:\n", exitLabel))
}

func (cg *codeGenerator) compileReturnStmt(sb *strings.Builder, s *ReturnStmt) {
	if s.Value != nil {
		cg.evalExprIntoA(sb, s.Value)
	}
	sb.WriteString("  RTS\n")
}

func (cg *codeGenerator) compileCallExpr(sb *strings.Builder, call *CallExpr) {
	// Function name resolution
	var funcPkg string
	var funcName string

	if ident, ok := call.Func.(*Ident); ok {
		funcPkg = cg.file.PackageName
		funcName = ident.Name
	} else if mem, ok := call.Func.(*MemberExpr); ok {
		if targetIdent, ok := mem.Target.(*Ident); ok {
			funcPkg = targetIdent.Name
			funcName = mem.Member
		}
	}

	mangledFunc := mangleSymbol(funcPkg, funcName)

	var targetFn *FuncDecl
	if funcPkg != "" {
		targetFn = cg.funcMap[funcPkg+"."+funcName]
	}
	if targetFn == nil {
		targetFn = cg.funcMap[funcName]
	}

	if targetFn != nil && len(targetFn.Params) > 0 {
		locs := computeParamLocations(targetFn, funcPkg)

		// 1. Evaluate memory / excess parameters first
		for i, loc := range locs {
			if loc.LocType == ParamLocMemory && i < len(call.Args) {
				arg := call.Args[i]
				if loc.Size == 1 {
					cg.evalExprIntoA(sb, arg)
					sb.WriteString(fmt.Sprintf("  STA %s\n", loc.MemSym))
				} else if loc.Size == 2 {
					cg.emitWordOrPointerLoad(sb, arg, loc.MemSym)
				} else if loc.Size == 4 {
					cg.emitDWordLoad(sb, arg, loc.MemSym)
				}
			}
		}

		// 2. Evaluate register parameters
		for i, loc := range locs {
			if i < len(call.Args) {
				arg := call.Args[i]
				switch loc.LocType {
				case ParamLocRegY:
					cg.evalExprIntoY(sb, arg)
				case ParamLocRegX:
					cg.evalExprIntoA(sb, arg)
					sb.WriteString("  STA _reg_x_shadow\n")
				case ParamLocRegA:
					cg.evalExprIntoA(sb, arg)
				case ParamLocRegAX:
					cg.emitAddressIntoAX(sb, arg)
				case ParamLocRegXY:
					cg.emitAddressIntoXY(sb, arg)
				}
			}
		}

		// Restore X if _reg_x_shadow was set
		for _, loc := range locs {
			if loc.LocType == ParamLocRegX {
				sb.WriteString("  LDX _reg_x_shadow\n")
				break
			}
		}

		sb.WriteString(fmt.Sprintf("  JSR %s\n", mangledFunc))
		return
	}

	// Fallback for calls without parameter info (e.g. asm stubs)
	if len(call.Args) >= 4 {
		cg.evalExprIntoA(sb, call.Args[3])
		sb.WriteString("  STA __leaf_param0\n")
	}
	if len(call.Args) >= 3 {
		cg.evalExprIntoY(sb, call.Args[2])
	}
	if len(call.Args) >= 2 {
		cg.evalExprIntoA(sb, call.Args[1])
		sb.WriteString("  STA _reg_x_shadow\n")
	}
	if len(call.Args) >= 1 {
		cg.evalExprIntoA(sb, call.Args[0])
	}
	if len(call.Args) >= 2 {
		sb.WriteString("  LDX _reg_x_shadow\n")
	}

	sb.WriteString(fmt.Sprintf("  JSR %s\n", mangledFunc))
}

func (cg *codeGenerator) emitJumpIf(sb *strings.Builder, branchMnemonic string, targetLabel string) {
	skip := cg.newLabel("skip_br")
	var opp string
	switch branchMnemonic {
	case "BEQ":
		opp = "BNE"
	case "BNE":
		opp = "BEQ"
	case "BCC":
		opp = "BCS"
	case "BCS":
		opp = "BCC"
	case "BMI":
		opp = "BPL"
	case "BPL":
		opp = "BMI"
	default:
		sb.WriteString(fmt.Sprintf("  %s %s\n", branchMnemonic, targetLabel))
		return
	}
	sb.WriteString(fmt.Sprintf("  %s %s\n", opp, skip))
	sb.WriteString(fmt.Sprintf("  JMP %s\n", targetLabel))
	sb.WriteString(fmt.Sprintf("%s:\n", skip))
}

func (cg *codeGenerator) compileConditionBranch(sb *strings.Builder, cond Expr, targetLabel string, branchOnTrue bool) {
	if bin, ok := cond.(*BinaryExpr); ok {
		// Evaluate LHS into A
		cg.evalExprIntoA(sb, bin.Left)

		// Compare with RHS
		rhsStr := cg.formatOperand(bin.Right)
		sb.WriteString(fmt.Sprintf("  CMP %s\n", rhsStr))

		switch bin.Op {
		case TokenEqEq:
			if branchOnTrue {
				cg.emitJumpIf(sb, "BEQ", targetLabel)
			} else {
				cg.emitJumpIf(sb, "BNE", targetLabel)
			}
		case TokenBangEq:
			if branchOnTrue {
				cg.emitJumpIf(sb, "BNE", targetLabel)
			} else {
				cg.emitJumpIf(sb, "BEQ", targetLabel)
			}
		case TokenLt:
			if branchOnTrue {
				cg.emitJumpIf(sb, "BCC", targetLabel)
			} else {
				cg.emitJumpIf(sb, "BCS", targetLabel)
			}
		case TokenLtEq:
			if branchOnTrue {
				take := cg.newLabel("take_br")
				skip := cg.newLabel("skip_br")
				sb.WriteString(fmt.Sprintf("  BEQ %s\n", take))
				sb.WriteString(fmt.Sprintf("  BCS %s\n", skip))
				sb.WriteString(fmt.Sprintf("%s:\n", take))
				sb.WriteString(fmt.Sprintf("  JMP %s\n", targetLabel))
				sb.WriteString(fmt.Sprintf("%s:\n", skip))
			} else {
				skip := cg.newLabel("skip_cmp")
				sb.WriteString(fmt.Sprintf("  BEQ %s\n", skip))
				sb.WriteString(fmt.Sprintf("  BCC %s\n", skip))
				sb.WriteString(fmt.Sprintf("  JMP %s\n", targetLabel))
				sb.WriteString(fmt.Sprintf("%s:\n", skip))
			}
		case TokenGt:
			if branchOnTrue {
				skip := cg.newLabel("skip_cmp")
				sb.WriteString(fmt.Sprintf("  BEQ %s\n", skip))
				sb.WriteString(fmt.Sprintf("  BCC %s\n", skip))
				sb.WriteString(fmt.Sprintf("  JMP %s\n", targetLabel))
				sb.WriteString(fmt.Sprintf("%s:\n", skip))
			} else {
				take := cg.newLabel("take_br")
				skip := cg.newLabel("skip_br")
				sb.WriteString(fmt.Sprintf("  BEQ %s\n", take))
				sb.WriteString(fmt.Sprintf("  BCS %s\n", skip))
				sb.WriteString(fmt.Sprintf("%s:\n", take))
				sb.WriteString(fmt.Sprintf("  JMP %s\n", targetLabel))
				sb.WriteString(fmt.Sprintf("%s:\n", skip))
			}
		case TokenGtEq:
			if branchOnTrue {
				cg.emitJumpIf(sb, "BCS", targetLabel)
			} else {
				cg.emitJumpIf(sb, "BCC", targetLabel)
			}
		}
		return
	}

	// Boolean variable or member expression: e.g. enemies[i].active
	cg.evalExprIntoA(sb, cond)
	if branchOnTrue {
		cg.emitJumpIf(sb, "BNE", targetLabel)
	} else {
		cg.emitJumpIf(sb, "BEQ", targetLabel)
	}
}

func (cg *codeGenerator) resolve2DArrayInfo(arrayExpr Expr) (pkg string, name string, is2D bool) {
	if ident, ok := arrayExpr.(*Ident); ok {
		name = ident.Name
		pkg = cg.file.PackageName
		if d, ok := cg.declMap[name]; ok {
			switch decl := d.(type) {
			case *DataDecl:
				if decl.Package != "" {
					pkg = decl.Package
				}
				is2D = is2DDecl(decl.Type, decl.Value)
			case *ConstDecl:
				if decl.Package != "" {
					pkg = decl.Package
				}
				is2D = is2DDecl(decl.Type, decl.Value)
			case *VarDecl:
				if decl.Package != "" {
					pkg = decl.Package
				}
				is2D = is2DDecl(decl.Type, decl.Init)
			}
		} else if d, ok := cg.declMap[cg.file.PackageName+"."+name]; ok {
			switch decl := d.(type) {
			case *DataDecl:
				if decl.Package != "" {
					pkg = decl.Package
				}
				is2D = is2DDecl(decl.Type, decl.Value)
			case *ConstDecl:
				if decl.Package != "" {
					pkg = decl.Package
				}
				is2D = is2DDecl(decl.Type, decl.Value)
			case *VarDecl:
				if decl.Package != "" {
					pkg = decl.Package
				}
				is2D = is2DDecl(decl.Type, decl.Init)
			}
		}
		return
	}
	if mem, ok := arrayExpr.(*MemberExpr); ok {
		if targetIdent, ok := mem.Target.(*Ident); ok {
			pkg = targetIdent.Name
			name = mem.Member
			key := pkg + "." + name
			if d, ok := cg.declMap[key]; ok {
				switch decl := d.(type) {
				case *DataDecl:
					is2D = is2DDecl(decl.Type, decl.Value)
				case *ConstDecl:
					is2D = is2DDecl(decl.Type, decl.Value)
				case *VarDecl:
					is2D = is2DDecl(decl.Type, decl.Init)
				}
			}
		}
		return
	}
	return "", "", false
}

func (cg *codeGenerator) evalExprIntoA(sb *strings.Builder, expr Expr) {
	if expr == nil {
		sb.WriteString("  LDA #$00\n")
		return
	}

	if length, ok := resolveLength(expr, cg.file.PackageName, cg.declMap); ok {
		sb.WriteString(fmt.Sprintf("  LDA #$%02X\n", length&0xFF))
		return
	}

	switch e := expr.(type) {
	case *NumberLit:
		sb.WriteString(fmt.Sprintf("  LDA #%s\n", formatConstExpr(e, cg.file.PackageName)))
	case *BoolLit:
		if e.Value {
			sb.WriteString("  LDA #$01\n")
		} else {
			sb.WriteString("  LDA #$00\n")
		}
	case *CharLit:
		sb.WriteString(fmt.Sprintf("  LDA #'%c'\n", e.Value))
	case *Ident:
		sym := cg.resolveSymbolName(e.Name)
		sb.WriteString(fmt.Sprintf("  LDA %s\n", sym))
	case *IndexExpr:
		if ident, ok := e.Array.(*Ident); ok {
			cg.evalExprIntoX(sb, e.Index)
			sym := cg.resolveSymbolName(ident.Name)
			sb.WriteString(fmt.Sprintf("  LDA %s, X\n", sym))
		}
	case *MemberExpr:
		if idx, ok := e.Target.(*IndexExpr); ok {
			if ident, ok := idx.Array.(*Ident); ok {
				cg.evalExprIntoX(sb, idx.Index)
				sym := mangleSymbol(cg.file.PackageName, fmt.Sprintf("%s_%s", ident.Name, e.Member))
				sb.WriteString(fmt.Sprintf("  LDA %s, X\n", sym))
			}
		} else if ident, ok := e.Target.(*Ident); ok {
			sym := mangleSymbol(ident.Name, e.Member)
			sb.WriteString(fmt.Sprintf("  LDA %s\n", sym))
		}
	case *UnaryExpr:
		if e.Op == TokenCaret {
			symStr := cg.formatRawSymbol(e.Operand)
			sb.WriteString(fmt.Sprintf("  LDA #^%s\n", symStr))
		} else if e.Op == TokenLt {
			symStr := cg.formatRawSymbol(e.Operand)
			sb.WriteString(fmt.Sprintf("  LDA #<%s\n", symStr))
		} else if e.Op == TokenGt {
			symStr := cg.formatRawSymbol(e.Operand)
			sb.WriteString(fmt.Sprintf("  LDA #>%s\n", symStr))
		}
	case *BinaryExpr:
		cg.evalExprIntoA(sb, e.Left)
		switch e.Op {
		case TokenPlus:
			sb.WriteString("  CLC\n")
			cg.emitAddA(sb, e.Right)
		case TokenMinus:
			sb.WriteString("  SEC\n")
			cg.emitSubA(sb, e.Right)
		case TokenAmp:
			cg.emitAndA(sb, e.Right)
		case TokenPipe:
			cg.emitOrA(sb, e.Right)
		}
	case *CallExpr:
		// Type cast like uint8(0)
		if len(e.Args) == 1 {
			cg.evalExprIntoA(sb, e.Args[0])
		}
	}
}

func (cg *codeGenerator) evalExprIntoX(sb *strings.Builder, expr Expr) {
	if length, ok := resolveLength(expr, cg.file.PackageName, cg.declMap); ok {
		sb.WriteString(fmt.Sprintf("  LDX #$%02X\n", length&0xFF))
		return
	}
	if num, ok := expr.(*NumberLit); ok {
		sb.WriteString(fmt.Sprintf("  LDX #%s\n", formatConstExpr(num, cg.file.PackageName)))
		return
	}
	if ident, ok := expr.(*Ident); ok {
		sym := cg.resolveSymbolName(ident.Name)
		sb.WriteString(fmt.Sprintf("  LDX %s\n", sym))
		return
	}
	cg.evalExprIntoA(sb, expr)
	sb.WriteString("  TAX\n")
}

func (cg *codeGenerator) evalExprIntoY(sb *strings.Builder, expr Expr) {
	if length, ok := resolveLength(expr, cg.file.PackageName, cg.declMap); ok {
		sb.WriteString(fmt.Sprintf("  LDY #$%02X\n", length&0xFF))
		return
	}
	if num, ok := expr.(*NumberLit); ok {
		sb.WriteString(fmt.Sprintf("  LDY #%s\n", formatConstExpr(num, cg.file.PackageName)))
		return
	}
	if ident, ok := expr.(*Ident); ok {
		sym := cg.resolveSymbolName(ident.Name)
		sb.WriteString(fmt.Sprintf("  LDY %s\n", sym))
		return
	}
	cg.evalExprIntoA(sb, expr)
	sb.WriteString("  TAY\n")
}

func (cg *codeGenerator) emitAddA(sb *strings.Builder, expr Expr) {
	sb.WriteString(fmt.Sprintf("  ADC %s\n", cg.formatOperand(expr)))
}

func (cg *codeGenerator) emitSubA(sb *strings.Builder, expr Expr) {
	sb.WriteString(fmt.Sprintf("  SBC %s\n", cg.formatOperand(expr)))
}

func (cg *codeGenerator) emitAndA(sb *strings.Builder, expr Expr) {
	sb.WriteString(fmt.Sprintf("  AND %s\n", cg.formatOperand(expr)))
}

func (cg *codeGenerator) emitOrA(sb *strings.Builder, expr Expr) {
	sb.WriteString(fmt.Sprintf("  ORA %s\n", cg.formatOperand(expr)))
}

func (cg *codeGenerator) emitPointerLoad(sb *strings.Builder, expr Expr, targetZP string) {
	// Pointer load into 16-bit zero page variable (targetZP, targetZP+1)
	if un, ok := expr.(*UnaryExpr); ok && un.Op == TokenAmp {
		if idx, ok := un.Operand.(*IndexExpr); ok {
			if ident, ok := idx.Array.(*Ident); ok {
				sym := cg.resolveSymbolName(ident.Name)
				offset := 0
				if num, ok := idx.Index.(*NumberLit); ok {
					offset = int(num.Value)
				}
				if offset > 0 {
					sb.WriteString(fmt.Sprintf("  LDA #<(%s + %d)\n", sym, offset))
					sb.WriteString(fmt.Sprintf("  STA %s\n", targetZP))
					sb.WriteString(fmt.Sprintf("  LDA #>(%s + %d)\n", sym, offset))
					sb.WriteString(fmt.Sprintf("  STA %s+1\n", targetZP))
				} else {
					sb.WriteString(fmt.Sprintf("  LDA #<%s\n", sym))
					sb.WriteString(fmt.Sprintf("  STA %s\n", targetZP))
					sb.WriteString(fmt.Sprintf("  LDA #>%s\n", sym))
					sb.WriteString(fmt.Sprintf("  STA %s+1\n", targetZP))
				}
				return
			}
		}
	}

	if idxExpr, ok := expr.(*IndexExpr); ok {
		pkg, name, is2D := cg.resolve2DArrayInfo(idxExpr.Array)
		if is2D {
			if num, ok := idxExpr.Index.(*NumberLit); ok {
				subSym := mangleSymbol(pkg, fmt.Sprintf("%s_%d", name, int(num.Value)))
				sb.WriteString(fmt.Sprintf("  LDA #<%s\n", subSym))
				sb.WriteString(fmt.Sprintf("  STA %s\n", targetZP))
				sb.WriteString(fmt.Sprintf("  LDA #>%s\n", subSym))
				sb.WriteString(fmt.Sprintf("  STA %s+1\n", targetZP))
				return
			}
			tableSym := mangleSymbol(pkg, name)
			cg.evalExprIntoA(sb, idxExpr.Index)
			sb.WriteString("  ASL\n")
			sb.WriteString("  TAX\n")
			sb.WriteString(fmt.Sprintf("  LDA %s, X\n", tableSym))
			sb.WriteString(fmt.Sprintf("  STA %s\n", targetZP))
			sb.WriteString(fmt.Sprintf("  LDA %s+1, X\n", tableSym))
			sb.WriteString(fmt.Sprintf("  STA %s+1\n", targetZP))
			return
		}
	}

	symStr := cg.formatRawSymbol(expr)
	sb.WriteString(fmt.Sprintf("  LDA #<%s\n", symStr))
	sb.WriteString(fmt.Sprintf("  STA %s\n", targetZP))
	sb.WriteString(fmt.Sprintf("  LDA #>%s\n", symStr))
	sb.WriteString(fmt.Sprintf("  STA %s+1\n", targetZP))
}

func (cg *codeGenerator) emitAddressIntoAX(sb *strings.Builder, expr Expr) {
	if length, ok := resolveLength(expr, cg.file.PackageName, cg.declMap); ok {
		sb.WriteString(fmt.Sprintf("  LDA #$%02X\n", length&0xFF))
		sb.WriteString(fmt.Sprintf("  LDX #$%02X\n", (length>>8)&0xFF))
		return
	}
	if num, ok := expr.(*NumberLit); ok {
		sb.WriteString(fmt.Sprintf("  LDA #$%02X\n", num.Value&0xFF))
		sb.WriteString(fmt.Sprintf("  LDX #$%02X\n", (num.Value>>8)&0xFF))
		return
	}
	if idxExpr, ok := expr.(*IndexExpr); ok {
		pkg, name, is2D := cg.resolve2DArrayInfo(idxExpr.Array)
		if is2D {
			if num, ok := idxExpr.Index.(*NumberLit); ok {
				subSym := mangleSymbol(pkg, fmt.Sprintf("%s_%d", name, int(num.Value)))
				sb.WriteString(fmt.Sprintf("  LDA #<%s\n", subSym))
				sb.WriteString(fmt.Sprintf("  LDX #>%s\n", subSym))
				return
			}
			tableSym := mangleSymbol(pkg, name)
			cg.evalExprIntoA(sb, idxExpr.Index)
			sb.WriteString("  ASL\n")
			sb.WriteString("  TAX\n")
			sb.WriteString(fmt.Sprintf("  LDA %s, X\n", tableSym))
			sb.WriteString("  PHA\n")
			sb.WriteString(fmt.Sprintf("  LDA %s+1, X\n", tableSym))
			sb.WriteString("  TAX\n")
			sb.WriteString("  PLA\n")
			return
		}
	}
	symStr := cg.formatRawSymbol(expr)
	sb.WriteString(fmt.Sprintf("  LDA #<%s\n", symStr))
	sb.WriteString(fmt.Sprintf("  LDX #>%s\n", symStr))
}

func (cg *codeGenerator) emitAddressIntoXY(sb *strings.Builder, expr Expr) {
	if length, ok := resolveLength(expr, cg.file.PackageName, cg.declMap); ok {
		sb.WriteString(fmt.Sprintf("  LDX #$%02X\n", length&0xFF))
		sb.WriteString(fmt.Sprintf("  LDY #$%02X\n", (length>>8)&0xFF))
		return
	}
	if num, ok := expr.(*NumberLit); ok {
		sb.WriteString(fmt.Sprintf("  LDX #$%02X\n", num.Value&0xFF))
		sb.WriteString(fmt.Sprintf("  LDY #$%02X\n", (num.Value>>8)&0xFF))
		return
	}
	if idxExpr, ok := expr.(*IndexExpr); ok {
		pkg, name, is2D := cg.resolve2DArrayInfo(idxExpr.Array)
		if is2D {
			if num, ok := idxExpr.Index.(*NumberLit); ok {
				subSym := mangleSymbol(pkg, fmt.Sprintf("%s_%d", name, int(num.Value)))
				sb.WriteString(fmt.Sprintf("  LDX #<%s\n", subSym))
				sb.WriteString(fmt.Sprintf("  LDY #>%s\n", subSym))
				return
			}
			tableSym := mangleSymbol(pkg, name)
			cg.evalExprIntoA(sb, idxExpr.Index)
			sb.WriteString("  ASL\n")
			sb.WriteString("  TAX\n")
			sb.WriteString(fmt.Sprintf("  LDA %s, X\n", tableSym))
			sb.WriteString("  PHA\n")
			sb.WriteString(fmt.Sprintf("  LDA %s+1, X\n", tableSym))
			sb.WriteString("  TAY\n")
			sb.WriteString("  PLA\n")
			sb.WriteString("  TAX\n")
			return
		}
	}
	symStr := cg.formatRawSymbol(expr)
	sb.WriteString(fmt.Sprintf("  LDX #<%s\n", symStr))
	sb.WriteString(fmt.Sprintf("  LDY #>%s\n", symStr))
}

func (cg *codeGenerator) emitWordOrPointerLoad(sb *strings.Builder, expr Expr, targetZP string) {
	if _, ok := resolveLength(expr, cg.file.PackageName, cg.declMap); ok {
		cg.emitWordLoad(sb, expr, targetZP)
		return
	}
	if num, ok := expr.(*NumberLit); ok {
		cg.emitWordLoad(sb, num, targetZP)
		return
	}
	cg.emitPointerLoad(sb, expr, targetZP)
}

func (cg *codeGenerator) emitDWordLoad(sb *strings.Builder, expr Expr, targetZP string) {
	if num, ok := expr.(*NumberLit); ok {
		sb.WriteString(fmt.Sprintf("  LDA #$%02X\n", num.Value&0xFF))
		sb.WriteString(fmt.Sprintf("  STA %s\n", targetZP))
		sb.WriteString(fmt.Sprintf("  LDA #$%02X\n", (num.Value>>8)&0xFF))
		sb.WriteString(fmt.Sprintf("  STA %s+1\n", targetZP))
		sb.WriteString(fmt.Sprintf("  LDA #$%02X\n", (num.Value>>16)&0xFF))
		sb.WriteString(fmt.Sprintf("  STA %s+2\n", targetZP))
		sb.WriteString(fmt.Sprintf("  LDA #$%02X\n", (num.Value>>24)&0xFF))
		sb.WriteString(fmt.Sprintf("  STA %s+3\n", targetZP))
		return
	}
	symStr := cg.formatRawSymbol(expr)
	sb.WriteString(fmt.Sprintf("  LDA %s\n", symStr))
	sb.WriteString(fmt.Sprintf("  STA %s\n", targetZP))
	sb.WriteString(fmt.Sprintf("  LDA %s+1\n", symStr))
	sb.WriteString(fmt.Sprintf("  STA %s+1\n", targetZP))
	sb.WriteString(fmt.Sprintf("  LDA %s+2\n", symStr))
	sb.WriteString(fmt.Sprintf("  STA %s+2\n", targetZP))
	sb.WriteString(fmt.Sprintf("  LDA %s+3\n", symStr))
	sb.WriteString(fmt.Sprintf("  STA %s+3\n", targetZP))
}

func (cg *codeGenerator) emitWordLoad(sb *strings.Builder, expr Expr, targetZP string) {
	if length, ok := resolveLength(expr, cg.file.PackageName, cg.declMap); ok {
		sb.WriteString(fmt.Sprintf("  LDA #$%02X\n", length&0xFF))
		sb.WriteString(fmt.Sprintf("  STA %s\n", targetZP))
		sb.WriteString(fmt.Sprintf("  LDA #$%02X\n", (length>>8)&0xFF))
		sb.WriteString(fmt.Sprintf("  STA %s+1\n", targetZP))
		return
	}
	if num, ok := expr.(*NumberLit); ok {
		sb.WriteString(fmt.Sprintf("  LDA #$%02X\n", num.Value&0xFF))
		sb.WriteString(fmt.Sprintf("  STA %s\n", targetZP))
		sb.WriteString(fmt.Sprintf("  LDA #$%02X\n", (num.Value>>8)&0xFF))
		sb.WriteString(fmt.Sprintf("  STA %s+1\n", targetZP))
		return
	}
	symStr := cg.formatRawSymbol(expr)
	sb.WriteString(fmt.Sprintf("  LDA #<%s\n", symStr))
	sb.WriteString(fmt.Sprintf("  STA %s\n", targetZP))
	sb.WriteString(fmt.Sprintf("  LDA #>%s\n", symStr))
	sb.WriteString(fmt.Sprintf("  STA %s+1\n", targetZP))
}

func (cg *codeGenerator) formatOperand(expr Expr) string {
	if length, ok := resolveLength(expr, cg.file.PackageName, cg.declMap); ok {
		return fmt.Sprintf("#$%02X", length&0xFF)
	}
	if num, ok := expr.(*NumberLit); ok {
		return fmt.Sprintf("#%s", formatConstExpr(num, cg.file.PackageName))
	}
	if b, ok := expr.(*BoolLit); ok {
		if b.Value {
			return "#$01"
		}
		return "#$00"
	}
	if ident, ok := expr.(*Ident); ok {
		return cg.resolveSymbolName(ident.Name)
	}
	if mem, ok := expr.(*MemberExpr); ok {
		if idx, ok := mem.Target.(*IndexExpr); ok {
			if ident, ok := idx.Array.(*Ident); ok {
				return mangleSymbol(cg.file.PackageName, fmt.Sprintf("%s_%s, X", ident.Name, mem.Member))
			}
		}
	}
	return "0"
}

func (cg *codeGenerator) formatRawSymbol(expr Expr) string {
	switch e := expr.(type) {
	case *Ident:
		return cg.resolveSymbolName(e.Name)
	case *MemberExpr:
		if targetIdent, ok := e.Target.(*Ident); ok {
			return mangleSymbol(targetIdent.Name, e.Member)
		}
		return mangleSymbol(cg.file.PackageName, e.Member)
	case *IndexExpr:
		pkg, name, is2D := cg.resolve2DArrayInfo(e.Array)
		if is2D {
			if num, ok := e.Index.(*NumberLit); ok {
				return mangleSymbol(pkg, fmt.Sprintf("%s_%d", name, int(num.Value)))
			}
			return mangleSymbol(pkg, name)
		}
		return formatConstExpr(expr, cg.file.PackageName)
	default:
		return formatConstExpr(expr, cg.file.PackageName)
	}
}

func (cg *codeGenerator) resolveSymbolName(name string) string {
	// Check parameters first
	if cg.curFunc != nil && cg.paramLocs != nil {
		if loc, ok := cg.paramLocs[name]; ok {
			return loc.MemSym
		}
	}
	// Check local variables in current function
	if cg.curFunc != nil {
		localMangled := fmt.Sprintf("%s_%s", cg.curFunc.Name, name)
		if _, ok := cg.localVars[localMangled]; ok {
			return mangleSymbol(cg.file.PackageName, localMangled)
		}
		for _, decl := range cg.file.Decls {
			if vd, ok := decl.(*VarDecl); ok && vd.Name == localMangled {
				return mangleSymbol(cg.file.PackageName, localMangled)
			}
		}
	}
	return mangleSymbol(cg.file.PackageName, name)
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

func is2DArray(t TypeSpec) bool {
	if arr, ok := t.(*ArrayType); ok {
		if _, isInnerArr := arr.Elem.(*ArrayType); isInnerArr {
			return true
		}
		if _, isInnerPtr := arr.Elem.(*PointerType); isInnerPtr {
			return true
		}
	}
	return false
}

func is2DDecl(typ TypeSpec, val Expr) bool {
	if is2DArray(typ) {
		return true
	}
	if arrLit, ok := val.(*ArrayLit); ok && len(arrLit.Elements) > 0 {
		switch arrLit.Elements[0].(type) {
		case *IncpalExpr, *IncchrExpr, *IncbinExpr, *ArrayLit, *StringLit:
			return true
		}
	}
	return false
}

func getExprDataLength(expr Expr) int {
	if expr == nil {
		return 0
	}
	switch e := expr.(type) {
	case *IncpalExpr:
		if e.Count != nil {
			if num, ok := e.Count.(*NumberLit); ok && num.Value > 0 {
				return int(num.Value)
			}
		}
		return 16
	case *ArrayLit:
		if e.Length != nil {
			if num, ok := e.Length.(*NumberLit); ok && num.Value > 0 {
				return int(num.Value)
			}
		}
		return len(e.Elements)
	case *StringLit:
		return len(e.Value)
	default:
		return 1
	}
}

func resolveLength(expr Expr, defaultPkg string, declMap map[string]Decl) (int, bool) {
	mem, ok := expr.(*MemberExpr)
	if !ok {
		return 0, false
	}
	if mem.Member != "Length" && mem.Member != "length" && mem.Member != "Len" && mem.Member != "len" {
		return 0, false
	}

	// Case 1: Target is IndexExpr, e.g. Palettes[0].Length or data.Palettes[0].Length
	if idxExpr, ok := mem.Target.(*IndexExpr); ok {
		var declKey string
		if ident, ok := idxExpr.Array.(*Ident); ok {
			declKey = ident.Name
		} else if targetMem, ok := idxExpr.Array.(*MemberExpr); ok {
			if targetIdent, ok := targetMem.Target.(*Ident); ok {
				declKey = targetIdent.Name + "." + targetMem.Member
			}
		}

		var targetDecl Decl
		if declKey != "" && declMap != nil {
			if d, ok := declMap[declKey]; ok {
				targetDecl = d
			} else if defaultPkg != "" {
				if d, ok := declMap[defaultPkg+"."+declKey]; ok {
					targetDecl = d
				}
			}
		}

		if targetDecl != nil {
			var val Expr
			var declType TypeSpec
			switch d := targetDecl.(type) {
			case *DataDecl:
				val = d.Value
				declType = d.Type
			case *ConstDecl:
				val = d.Value
				declType = d.Type
			case *VarDecl:
				val = d.Init
				declType = d.Type
			}

			if num, ok := idxExpr.Index.(*NumberLit); ok {
				idx := int(num.Value)
				if arrLit, ok := val.(*ArrayLit); ok {
					if idx >= 0 && idx < len(arrLit.Elements) {
						return getExprDataLength(arrLit.Elements[idx]), true
					}
				}
			} else {
				// Variable index: if elements are uniform
				if arrLit, ok := val.(*ArrayLit); ok && len(arrLit.Elements) > 0 {
					firstLen := getExprDataLength(arrLit.Elements[0])
					allSame := true
					for _, elem := range arrLit.Elements {
						if getExprDataLength(elem) != firstLen {
							allSame = false
							break
						}
					}
					if allSame {
						return firstLen, true
					}
				}
			}

			// Check inner array type length if fixed
			if arrType, ok := declType.(*ArrayType); ok {
				if innerArr, ok := arrType.Elem.(*ArrayType); ok && innerArr.Length != nil {
					if innerNum, ok := innerArr.Length.(*NumberLit); ok && innerNum.Value > 0 {
						return int(innerNum.Value), true
					}
				}
			}
		}
	}

	// Case 2: Target is Ident or MemberExpr directly, e.g. Palettes.Length or TilesSurfacePal.Length
	var declKey string
	if ident, ok := mem.Target.(*Ident); ok {
		declKey = ident.Name
	} else if targetMem, ok := mem.Target.(*MemberExpr); ok {
		if targetIdent, ok := targetMem.Target.(*Ident); ok {
			declKey = targetIdent.Name + "." + targetMem.Member
		}
	}

	if declKey != "" && declMap != nil {
		var targetDecl Decl
		if d, ok := declMap[declKey]; ok {
			targetDecl = d
		} else if defaultPkg != "" {
			if d, ok := declMap[defaultPkg+"."+declKey]; ok {
				targetDecl = d
			}
		}

		if targetDecl != nil {
			switch d := targetDecl.(type) {
			case *DataDecl:
				if is2DDecl(d.Type, d.Value) {
					if arrLit, ok := d.Value.(*ArrayLit); ok {
						return len(arrLit.Elements), true
					}
				}
				return getExprDataLength(d.Value), true
			case *ConstDecl:
				if is2DDecl(d.Type, d.Value) {
					if arrLit, ok := d.Value.(*ArrayLit); ok {
						return len(arrLit.Elements), true
					}
				}
				return getExprDataLength(d.Value), true
			case *VarDecl:
				if arrType, ok := d.Type.(*ArrayType); ok && arrType.Length != nil {
					if num, ok := arrType.Length.(*NumberLit); ok && num.Value > 0 {
						return int(num.Value), true
					}
				}
				return varSize(d), true
			}
		}
	}

	return 0, false
}

func emit2DDataOrConstDecl(sb *strings.Builder, name string, pkg string, isExport bool, typ TypeSpec, length Expr, val Expr, structMap map[string]*StructType) error {
	var innerType TypeSpec
	if arrType, ok := typ.(*ArrayType); ok {
		innerType = arrType.Elem
	}
	expectedLen := -1
	if length != nil {
		if num, ok := length.(*NumberLit); ok && num.Value >= 0 {
			expectedLen = int(num.Value)
		}
	} else if arrType, ok := typ.(*ArrayType); ok && arrType.Length != nil {
		if num, ok := arrType.Length.(*NumberLit); ok && num.Value >= 0 {
			expectedLen = int(num.Value)
		}
	}

	arrLit, ok := val.(*ArrayLit)
	if !ok {
		// Fallback to single value
		mainSym := mangleSymbol(pkg, name)
		if isExport {
			sb.WriteString(fmt.Sprintf(".export %s\n", mainSym))
		}
		sb.WriteString(fmt.Sprintf("%s:\n", mainSym))
		return emitConstDataDeclValue(sb, val, typ, structMap, pkg)
	}

	if innerType == nil && arrLit.ElemType != nil {
		innerType = arrLit.ElemType
	}

	// 1. Emit each sub-array with label _<pkg>_<name>_<i>
	for i, elem := range arrLit.Elements {
		subSym := mangleSymbol(pkg, fmt.Sprintf("%s_%d", name, i))
		if isExport {
			sb.WriteString(fmt.Sprintf(".export %s\n", subSym))
		}
		sb.WriteString(fmt.Sprintf("%s:\n", subSym))
		if err := emitConstDataDeclValue(sb, elem, innerType, structMap, pkg); err != nil {
			return err
		}
		sb.WriteString("\n")
	}

	// 2. Emit main pointer table
	mainSym := mangleSymbol(pkg, name)
	if isExport {
		sb.WriteString(fmt.Sprintf(".export %s\n", mainSym))
	}
	sb.WriteString(fmt.Sprintf("%s:\n", mainSym))

	var ptrItems []string
	for i := range arrLit.Elements {
		ptrItems = append(ptrItems, mangleSymbol(pkg, fmt.Sprintf("%s_%d", name, i)))
	}
	if expectedLen > len(arrLit.Elements) {
		for i := len(arrLit.Elements); i < expectedLen; i++ {
			ptrItems = append(ptrItems, "$0000")
		}
	}

	sb.WriteString(fmt.Sprintf("  .word %s\n", strings.Join(ptrItems, ", ")))
	return nil
}

func typeSize(t TypeSpec) int {
	return typeSizeWithStructs(t, nil)
}

func typeSizeWithStructs(t TypeSpec, structMap map[string]*StructType) int {
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
			if structMap != nil {
				if st, ok := structMap[typ.Name]; ok {
					sz := 0
					for _, f := range st.Fields {
						sz += typeSizeWithStructs(f.Type, structMap)
					}
					return sz
				}
			}
			return 1
		}
	case *ArrayType:
		if is2DArray(typ) {
			if typ.Length != nil {
				if num, ok := typ.Length.(*NumberLit); ok && num.Value > 0 {
					return 2 * int(num.Value)
				}
			}
			return 2
		}
		elemSize := typeSizeWithStructs(typ.Elem, structMap)
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

func emitConstDataDeclValue(sb *strings.Builder, val Expr, targetType TypeSpec, structMap map[string]*StructType, pkg string) error {
	if val == nil {
		sz := typeSizeWithStructs(targetType, structMap)
		emitZeroBytes(sb, sz)
		return nil
	}

	// Direct inclusion directives
	if strLit, ok := val.(*StringLit); ok && (targetType == nil || isStringType(targetType)) {
		sb.WriteString(fmt.Sprintf("  .asciiz %q\n", strLit.Value))
		return nil
	}
	if incbin, ok := val.(*IncbinExpr); ok {
		sb.WriteString(fmt.Sprintf("  .incbin %q\n", incbin.Path))
		return nil
	}
	if incchr, ok := val.(*IncchrExpr); ok {
		sb.WriteString(fmt.Sprintf("  .incchr %q\n", incchr.Path))
		return nil
	}
	if incpal, ok := val.(*IncpalExpr); ok {
		if incpal.Count != nil {
			sb.WriteString(fmt.Sprintf("  .incpal %q, %s\n", incpal.Path, formatConstExpr(incpal.Count, pkg)))
		} else {
			sb.WriteString(fmt.Sprintf("  .incpal %q\n", incpal.Path))
		}
		return nil
	}

	// Struct literal or struct type
	var structName string
	if targetType != nil {
		if nt, ok := targetType.(*NamedType); ok {
			if _, ok := structMap[nt.Name]; ok {
				structName = nt.Name
			}
		}
	}
	if structLit, ok := val.(*StructLit); ok {
		if structName == "" && structLit.Type != nil {
			if nt, ok := structLit.Type.(*NamedType); ok {
				structName = nt.Name
			}
		}
		if structName == "" {
			return fmt.Errorf("%s: cannot determine struct type for struct literal", structLit.Pos())
		}
		st, ok := structMap[structName]
		if !ok {
			return fmt.Errorf("%s: undefined struct type %q", structLit.Pos(), structName)
		}
		return emitStructLiteral(sb, structLit, st, structMap, pkg)
	}

	// Array literal or Array type
	if arrType, ok := targetType.(*ArrayType); ok {
		return emitArrayValue(sb, val, arrType, structMap, pkg)
	}
	if arrLit, ok := val.(*ArrayLit); ok {
		var elemType TypeSpec = arrLit.ElemType
		return emitArrayValue(sb, arrLit, &ArrayType{Elem: elemType, Length: arrLit.Length, pos: arrLit.Pos()}, structMap, pkg)
	}

	// Scalar values
	if numLit, ok := val.(*NumberLit); ok {
		sz := typeSizeWithStructs(targetType, structMap)
		if sz == 2 {
			sb.WriteString(fmt.Sprintf("  .word $%04X\n", numLit.Value&0xFFFF))
		} else if sz == 4 {
			sb.WriteString(fmt.Sprintf("  .dword $%08X\n", numLit.Value&0xFFFFFFFF))
		} else {
			sb.WriteString(fmt.Sprintf("  .byte $%02X\n", numLit.Value&0xFF))
		}
		return nil
	}

	if boolLit, ok := val.(*BoolLit); ok {
		if boolLit.Value {
			sb.WriteString("  .byte $01\n")
		} else {
			sb.WriteString("  .byte $00\n")
		}
		return nil
	}

	if charLit, ok := val.(*CharLit); ok {
		sb.WriteString(fmt.Sprintf("  .byte $%02X\n", uint8(charLit.Value)))
		return nil
	}

	// Fallback to formatted constant expression
	sz := typeSizeWithStructs(targetType, structMap)
	if sz == 2 {
		sb.WriteString(fmt.Sprintf("  .word %s\n", formatConstExpr(val, pkg)))
	} else {
		sb.WriteString(fmt.Sprintf("  .byte %s\n", formatConstExpr(val, pkg)))
	}
	return nil
}

func emitStructLiteral(sb *strings.Builder, lit *StructLit, st *StructType, structMap map[string]*StructType, pkg string) error {
	provided := make(map[string]*StructLitField)
	for _, f := range lit.Fields {
		if _, exists := provided[f.Name]; exists {
			return fmt.Errorf("%s: duplicate field %q in struct literal", f.Pos(), f.Name)
		}
		provided[f.Name] = f
	}

	// Check for unknown fields
	validFields := make(map[string]bool)
	for _, sf := range st.Fields {
		validFields[sf.Name] = true
	}
	for _, f := range lit.Fields {
		if !validFields[f.Name] {
			return fmt.Errorf("%s: unknown field %q in struct literal", f.Pos(), f.Name)
		}
	}

	// Emit fields in struct definition order
	for _, sf := range st.Fields {
		if f, ok := provided[sf.Name]; ok {
			if err := emitConstDataDeclValue(sb, f.Value, sf.Type, structMap, pkg); err != nil {
				return err
			}
		} else {
			// Zero-initialize omitted field
			sz := typeSizeWithStructs(sf.Type, structMap)
			emitZeroBytes(sb, sz)
		}
	}
	return nil
}

func emitArrayValue(sb *strings.Builder, val Expr, arrType *ArrayType, structMap map[string]*StructType, pkg string) error {
	elemType := arrType.Elem
	expectedLen := -1
	if arrType.Length != nil {
		if num, ok := arrType.Length.(*NumberLit); ok && num.Value >= 0 {
			expectedLen = int(num.Value)
		}
	}

	if strLit, ok := val.(*StringLit); ok {
		bytes := []byte(strLit.Value)
		var items []string
		for _, b := range bytes {
			items = append(items, fmt.Sprintf("$%02X", b))
		}
		if len(items) > 0 {
			sb.WriteString(fmt.Sprintf("  .byte %s\n", strings.Join(items, ", ")))
		}
		if expectedLen > len(bytes) {
			emitZeroBytes(sb, expectedLen-len(bytes))
		}
		return nil
	}

	if arrLit, ok := val.(*ArrayLit); ok {
		if expectedLen >= 0 && len(arrLit.Elements) > expectedLen {
			return fmt.Errorf("%s: too many elements in array literal (expected %d, got %d)", arrLit.Pos(), expectedLen, len(arrLit.Elements))
		}

		// If elements are primitive scalar bytes, compact into single .byte lines
		isSimpleBytes := false
		if elemType != nil {
			if nt, ok := elemType.(*NamedType); ok {
				if nt.Name == "uint8" || nt.Name == "int8" || nt.Name == "bool" || nt.Name == "byte" {
					isSimpleBytes = true
				}
			}
		} else {
			isSimpleBytes = true
		}

		if isSimpleBytes {
			allSimple := true
			for _, elem := range arrLit.Elements {
				switch elem.(type) {
				case *NumberLit, *BoolLit, *CharLit:
				default:
					allSimple = false
				}
			}
			if allSimple && len(arrLit.Elements) > 0 {
				var items []string
				for _, elem := range arrLit.Elements {
					switch e := elem.(type) {
					case *NumberLit:
						items = append(items, fmt.Sprintf("$%02X", e.Value&0xFF))
					case *BoolLit:
						if e.Value {
							items = append(items, "$01")
						} else {
							items = append(items, "$00")
						}
					case *CharLit:
						items = append(items, fmt.Sprintf("$%02X", uint8(e.Value)))
					}
				}
				sb.WriteString(fmt.Sprintf("  .byte %s\n", strings.Join(items, ", ")))

				if expectedLen > len(arrLit.Elements) {
					elemSz := typeSizeWithStructs(elemType, structMap)
					emitZeroBytes(sb, (expectedLen-len(arrLit.Elements))*elemSz)
				}
				return nil
			}
		}

		// Otherwise, emit each element recursively
		for _, elem := range arrLit.Elements {
			if err := emitConstDataDeclValue(sb, elem, elemType, structMap, pkg); err != nil {
				return err
			}
		}

		if expectedLen > len(arrLit.Elements) {
			elemSz := typeSizeWithStructs(elemType, structMap)
			emitZeroBytes(sb, (expectedLen-len(arrLit.Elements))*elemSz)
		}
		return nil
	}

	if incbin, ok := val.(*IncbinExpr); ok {
		sb.WriteString(fmt.Sprintf("  .incbin %q\n", incbin.Path))
		return nil
	}
	if incchr, ok := val.(*IncchrExpr); ok {
		sb.WriteString(fmt.Sprintf("  .incchr %q\n", incchr.Path))
		return nil
	}
	if incpal, ok := val.(*IncpalExpr); ok {
		if incpal.Count != nil {
			sb.WriteString(fmt.Sprintf("  .incpal %q, %s\n", incpal.Path, formatConstExpr(incpal.Count, pkg)))
		} else {
			sb.WriteString(fmt.Sprintf("  .incpal %q\n", incpal.Path))
		}
		return nil
	}

	return emitConstDataDeclValue(sb, val, elemType, structMap, pkg)
}

func emitZeroBytes(sb *strings.Builder, count int) {
	if count <= 0 {
		return
	}
	if count <= 4 {
		var items []string
		for i := 0; i < count; i++ {
			items = append(items, "$00")
		}
		sb.WriteString(fmt.Sprintf("  .byte %s\n", strings.Join(items, ", ")))
	} else {
		sb.WriteString(fmt.Sprintf("  .res %d, $00\n", count))
	}
}

func isStringType(t TypeSpec) bool {
	if t == nil {
		return false
	}
	if nt, ok := t.(*NamedType); ok && nt.Name == "string" {
		return true
	}
	if arr, ok := t.(*ArrayType); ok {
		if nt, ok := arr.Elem.(*NamedType); ok && (nt.Name == "string" || nt.Name == "uint8" || nt.Name == "byte") {
			return true
		}
	}
	return false
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
		if length, ok := resolveLength(e, defaultPkg, nil); ok {
			return fmt.Sprintf("%d", length)
		}
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

func emitBank(sb *strings.Builder, b *BankSpec, pkg string) {
	if b != nil {
		if b.IsAuto {
			sb.WriteString(".bank auto\n")
		} else if b.Value != nil {
			if num, ok := b.Value.(*NumberLit); ok {
				sb.WriteString(fmt.Sprintf(".bank %d\n", num.Value))
			} else {
				sb.WriteString(fmt.Sprintf(".bank %s\n", formatConstExpr(b.Value, pkg)))
			}
		} else {
			sb.WriteString(".bank auto\n")
		}
	} else {
		sb.WriteString(".bank auto\n")
	}
}


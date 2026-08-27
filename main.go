package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/qbradq/m3/pkg/asm"
	"github.com/qbradq/m3/pkg/compiler"
	"github.com/qbradq/m3/pkg/linker"
	"github.com/qbradq/m3/pkg/obj"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
	case "build":
		if err := handleBuild(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "m3 build error: %v\n", err)
			os.Exit(1)
		}
	case "compile":
		if err := handleCompile(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "m3 compile error: %v\n", err)
			os.Exit(1)
		}
	case "assemble":
		if err := handleAssemble(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "m3 assemble error: %v\n", err)
			os.Exit(1)
		}
	case "link":
		if err := handleLink(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "m3 link error: %v\n", err)
			os.Exit(1)
		}
	case "dump-obj":
		if err := handleDumpObj(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "m3 dump-obj error: %v\n", err)
			os.Exit(1)
		}
	case "version":
		fmt.Println("m3 version 0.1.0")
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: m3 <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  build <input.m3...> [-o output.nes] [-g]  Build and link .m3 source files into an NES ROM (.nes)")
	fmt.Println("  compile <input.m3> [output.s]             Compile an m3 source file into an assembly file")
	fmt.Println("  assemble <input.s> [output.mo]            Assemble an assembly file into an intermediate object file")
	fmt.Println("  link <input.mo...> [-o output.nes] [-g]   Link one or more object files into an NES ROM (.nes)")
	fmt.Println("  dump-obj <file.mo>                        Inspect and dump the contents of an object file")
	fmt.Println("  version                                   Display the m3 version")
	fmt.Println("  help                                      Display this help message")
}

func handleBuild(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing input files\nUsage: m3 build <input.m3...> [-o output.nes] [-g]")
	}

	var inputFiles []string
	var outputFile string
	var debugSymbols bool

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-o" || arg == "--output":
			if i+1 >= len(args) {
				return fmt.Errorf("missing argument for %s flag", arg)
			}
			outputFile = args[i+1]
			i++
		case arg == "-g" || arg == "-g1" || arg == "-g2" || arg == "-g3" || arg == "--debug":
			debugSymbols = true
		case arg == "-g0":
			debugSymbols = false
		default:
			inputFiles = append(inputFiles, arg)
		}
	}

	if len(inputFiles) == 0 {
		return fmt.Errorf("missing input source files\nUsage: m3 build <input.m3...> [-o output.nes] [-g]")
	}

	for _, file := range inputFiles {
		if strings.ToLower(filepath.Ext(file)) != ".m3" {
			return fmt.Errorf("invalid input file %q: all input files must be .m3 source files", file)
		}
	}

	if outputFile == "" {
		first := inputFiles[0]
		ext := filepath.Ext(first)
		if ext != "" {
			outputFile = strings.TrimSuffix(first, ext) + ".nes"
		} else {
			outputFile = first + ".nes"
		}
	}

	return compiler.BuildFilesWithOptions(inputFiles, outputFile, compiler.BuildOptions{
		Debug: debugSymbols,
	})
}

func handleCompile(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing input file\nUsage: m3 compile <input.m3> [output.s]")
	}

	inputFile := args[0]
	outputFile := ""
	if len(args) >= 2 {
		outputFile = args[1]
	} else {
		// Default output file to same base with .s extension
		ext := filepath.Ext(inputFile)
		if ext != "" {
			outputFile = strings.TrimSuffix(inputFile, ext) + ".s"
		} else {
			outputFile = inputFile + ".s"
		}
	}

	return compiler.CompileFile(inputFile, outputFile)
}

func handleAssemble(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing input file\nUsage: m3 assemble <input.s> [output.mo]")
	}

	inputFile := args[0]
	outputFile := ""
	if len(args) >= 2 {
		outputFile = args[1]
	} else {
		// Default output file to same base with .mo extension
		ext := filepath.Ext(inputFile)
		if ext != "" {
			outputFile = strings.TrimSuffix(inputFile, ext) + ".mo"
		} else {
			outputFile = inputFile + ".mo"
		}
	}

	content, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read source file %q: %w", inputFile, err)
	}

	objectFile, err := asm.Assemble(inputFile, string(content))
	if err != nil {
		return err
	}

	if err := objectFile.WriteFile(outputFile); err != nil {
		return fmt.Errorf("failed to write object file %q: %w", outputFile, err)
	}

	return nil
}

func handleDumpObj(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing object file\nUsage: m3 dump-obj <file.mo>")
	}
	objFile, err := obj.ReadFile(args[0])
	if err != nil {
		return err
	}
	fmt.Print(objFile.Dump())
	return nil
}

func handleLink(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing input files\nUsage: m3 link <input.mo> [input2.mo...] [-o output.nes] [-g]")
	}

	var inputFiles []string
	var outputFile string
	var debugSymbols bool

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-o" || arg == "--output":
			if i+1 >= len(args) {
				return fmt.Errorf("missing argument for %s flag", arg)
			}
			outputFile = args[i+1]
			i++
		case arg == "-g" || arg == "-g1" || arg == "-g2" || arg == "-g3" || arg == "--debug":
			debugSymbols = true
		case arg == "-g0":
			debugSymbols = false
		default:
			inputFiles = append(inputFiles, arg)
		}
	}

	if len(inputFiles) == 0 {
		return fmt.Errorf("missing input object files\nUsage: m3 link <input.mo> [input2.mo...] [-o output.nes] [-g]")
	}

	if outputFile == "" {
		if len(inputFiles) >= 2 && strings.HasSuffix(strings.ToLower(inputFiles[len(inputFiles)-1]), ".nes") {
			outputFile = inputFiles[len(inputFiles)-1]
			inputFiles = inputFiles[:len(inputFiles)-1]
		} else {
			first := inputFiles[0]
			ext := filepath.Ext(first)
			if ext != "" {
				outputFile = strings.TrimSuffix(first, ext) + ".nes"
			} else {
				outputFile = first + ".nes"
			}
		}
	}

	return linker.LinkFilesWithOptions(inputFiles, outputFile, linker.LinkOptions{
		Debug: debugSymbols,
	})
}


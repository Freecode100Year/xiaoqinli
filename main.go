package main

import (
	"fmt"
	"os"

	"xiaoqinli/ast"
	"xiaoqinli/check"
	"xiaoqinli/codegen"
	"xiaoqinli/server"
)

const Version = "3.1.0"

var allTargets = []string{
	"go", "rust", "ts", "kotlin", "swift", "py",
	"java", "csharp", "dart", "lua", "ruby", "php",
	"zig", "nim", "julia", "cpp", "c", "scala", "haskell",
	"mql4", "mql5",
	"ocaml", "fsharp", "elixir", "clojure",
	"ada", "awk", "bash", "crystal", "d", "fortran",
	"objc", "pascal", "perl", "powershell", "tcl", "v",
	"vala",
	"groovy",
	"bat",
}

const usage = `xiaoqinli - AST-First transpiler v` + Version + `

Usage:
  xiaoqinli compile --file <path.xql.json> --target <lang> [--out <output>]
  xiaoqinli validate --file <path.xql.json>
  xiaoqinli targets                         List all supported target languages
  xiaoqinli stdio                           MCP stdio mode
  xiaoqinli http [<:port>] [--mode rest]    MCP/REST HTTP mode (default :8080)

Targets: go | rust | ts | kotlin | swift | py | java | csharp | dart | lua | ruby | php | zig | nim | julia | cpp | c | scala | haskell | ocaml | fsharp | elixir | clojure | mql4 | mql5 | ada | awk | bash | bat | crystal | d | fortran | objc | pascal | perl | powershell | tcl | v | vala | groovy (default: go)

Exit codes: 0=success 1=validation failed 2=compilation error 3=argument error`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(3)
	}

	switch os.Args[1] {
	case "compile":
		cmdCompile(os.Args[2:])
	case "validate":
		cmdValidate(os.Args[2:])
	case "targets":
		for _, t := range allTargets {
			fmt.Println(t)
		}
	case "stdio":
		mcp := server.NewMCPServer()
		if err := mcp.ServeStdio(); err != nil {
			fmt.Fprintf(os.Stderr, "stdio error: %v\n", err)
			os.Exit(1)
		}
	case "http":
		cmdHTTP(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "error: unknown command '%s'\n\n%s\n", os.Args[1], usage)
		os.Exit(3)
	}
}

// parseFlags extracts --key value pairs from args.
func parseFlags(args []string) map[string]string {
	flags := make(map[string]string)
	for i := 0; i < len(args); i++ {
		if len(args[i]) > 2 && args[i][:2] == "--" {
			key := args[i][2:]
			if i+1 < len(args) {
				flags[key] = args[i+1]
				i++
			}
		}
	}
	return flags
}

func cmdValidate(args []string) {
	flags := parseFlags(args)
	filePath := flags["file"]
	if filePath == "" {
		fmt.Fprintln(os.Stderr, "error: --file is required")
		os.Exit(3)
	}

	root, err := loadAST(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	if err := check.RunAll(root); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	fmt.Println("ok: all checks passed")
}

func cmdCompile(args []string) {
	flags := parseFlags(args)
	filePath := flags["file"]
	target := flags["target"]
	outPath := flags["out"]

	if filePath == "" {
		fmt.Fprintln(os.Stderr, "error: --file is required")
		os.Exit(3)
	}
	if target == "" {
		target = "go"
	}

	// Load and parse AST.
	root, err := loadAST(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	// Compile MUST first pass validate (architecture requirement).
	if err := check.RunAll(root); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	// Code generation.
	output, err := codegen.Generate(root, target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "XQL_E401: codegen error: %v\n", err)
		os.Exit(2)
	}

	// Output result.
	if outPath != "" {
		if err := os.WriteFile(outPath, output, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing output: %v\n", err)
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "ok: compiled to %s\n", outPath)
	} else {
		fmt.Print(string(output))
	}
}

func cmdHTTP(args []string) {
	flags := parseFlags(args)
	addr := flags["addr"]
	if addr == "" {
		if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
			addr = args[0]
		} else {
			addr = ":8080"
		}
	}
	mode := flags["mode"]
	if mode == "rest" {
		rest := server.NewRESTServer()
		if err := rest.Serve(addr); err != nil {
			fmt.Fprintf(os.Stderr, "REST error: %v\n", err)
			os.Exit(1)
		}
	} else {
		mcp := server.NewMCPServer()
		if err := mcp.ServeHTTP(addr); err != nil {
			fmt.Fprintf(os.Stderr, "MCP HTTP error: %v\n", err)
			os.Exit(1)
		}
	}
}

// loadAST reads a .xql.json file and parses it into a typed AST.
func loadAST(path string) (ast.Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("XQL_E404: %v", err)
	}
	root, err := ast.Parse(data)
	if err != nil {
		return nil, err
	}
	return root, nil
}

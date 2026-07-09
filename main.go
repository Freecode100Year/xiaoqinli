package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"xiaoqinli/compiler"
	"xiaoqinli/server"
)

const Version = compiler.Version

func getUsage() string {
	return fmt.Sprintf(`xiaoqinli - AST-First transpiler v%s

Usage:
  xiaoqinli compile --file <path.xql.json> --target <lang> [--out <output>]
  xiaoqinli validate --file <path.xql.json>
  xiaoqinli targets                         List all supported target languages
  xiaoqinli stdio                           MCP stdio mode
  xiaoqinli http [<:port>] [--mode rest]    MCP/REST HTTP mode (default :8080)

Targets: %s (default: go)

Exit codes: 0=success 1=validation failed 2=compilation error 3=argument error`,
		Version, strings.Join(compiler.GetSupportedTargets(), " | "))
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, getUsage())
		os.Exit(3)
	}

	switch os.Args[1] {
	case "compile":
		cmdCompile(os.Args[2:])
	case "validate":
		cmdValidate(os.Args[2:])
	case "targets":
		for _, t := range compiler.GetSupportedTargets() {
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
		fmt.Fprintf(os.Stderr, "error: unknown command '%s'\n\n%s\n", os.Args[1], getUsage())
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

	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "XQL_E404: %v\n", err)
		os.Exit(1)
	}

	pr := compiler.ParseAST(compiler.ParseRequest{Data: data, FilePath: filePath})
	if !pr.Success {
		fmt.Fprintf(os.Stderr, "%s\n", pr.Error)
		os.Exit(1)
	}

	vr := compiler.Validate(compiler.ValidateRequest{AST: pr.AST})
	if !vr.Success {
		fmt.Fprintf(os.Stderr, "%s\n", vr.Error)
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

	// Use the compiler library for the full pipeline.
	result := compiler.CompileFromFile(filePath, target, outPath)

	if !result.Success {
		fmt.Fprintf(os.Stderr, "%s\n", result.Error)
		// Output structured diagnostics for AI/IDE consumers.
		if len(result.Diagnostics) > 0 {
			diagJSON, _ := json.MarshalIndent(result.Diagnostics, "", "  ")
			fmt.Fprintf(os.Stderr, "Diagnostics: %s\n", string(diagJSON))
		}
		os.Exit(2)
	}

	// Output result.
	if outPath != "" {
		if target == "chrome" {
			fmt.Fprintf(os.Stderr, "ok: Chrome extension unpacked to %s/\n", outPath)
			fmt.Fprintf(os.Stderr, "    Load unpacked in chrome://extensions (Developer mode)\n")
		} else {
			fmt.Fprintf(os.Stderr, "ok: compiled to %s\n", outPath)
		}
	} else {
		fmt.Print(string(result.Code))
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


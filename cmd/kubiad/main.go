package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/FunnyFoXD/kubiad/internal/generator"
	"github.com/FunnyFoXD/kubiad/internal/ir"
	"github.com/FunnyFoXD/kubiad/internal/parser"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}
	if os.Args[1] == "check" {
		runCheck(os.Args[2:])
		return
	}
	if os.Args[1] == "build" {
		runBuild(os.Args[2:])
		return
	}
	program, ok := programForCommand(os.Args[1])
	if !ok {
		printUsage()
		os.Exit(2)
	}
	flags := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	output := flags.String("output", "", "Generated project directory")
	module := flags.String("module", "", "Generated Go module path")
	_ = flags.Parse(os.Args[2:])
	if *output == "" {
		fmt.Fprintln(os.Stderr, "--output is required")
		os.Exit(2)
	}
	if err := generator.Generate(program, *output, generator.Options{Module: *module}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runCheck(arguments []string) {
	if len(arguments) != 1 {
		fmt.Fprintln(os.Stderr, "usage: kubiad check <source.kbi>")
		os.Exit(2)
	}
	if _, err := parser.ParseFile(arguments[0]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%s: valid\n", arguments[0])
}

func runBuild(arguments []string) {
	if len(arguments) == 0 {
		fmt.Fprintln(os.Stderr, "usage: kubiad build <source.kbi> --output <directory> [--module <module-path>]")
		os.Exit(2)
	}
	source := arguments[0]
	flags := flag.NewFlagSet("build", flag.ExitOnError)
	output := flags.String("output", "", "Generated project directory")
	module := flags.String("module", "", "Generated Go module path")
	_ = flags.Parse(arguments[1:])
	if *output == "" {
		fmt.Fprintln(os.Stderr, "--output is required")
		os.Exit(2)
	}
	program, err := parser.ParseFile(source)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := generator.Generate(program, *output, generator.Options{Module: *module}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: kubiad <check|build|generate-web-application|generate-worker-application|generate-scalable-web-application> ...")
}

func programForCommand(command string) (ir.Program, bool) {
	switch command {
	case "generate-web-application":
		return ir.WebApplication(), true
	case "generate-worker-application":
		return ir.WorkerApplication(), true
	case "generate-scalable-web-application":
		return ir.ScalableWebApplication(), true
	default:
		return ir.Program{}, false
	}
}

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/FunnyFoXD/kubiad/internal/generator"
	"github.com/FunnyFoXD/kubiad/internal/ir"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: kubiad <generate-web-application|generate-worker-application|generate-scalable-web-application> --output <directory> [--module <module-path>]")
		os.Exit(2)
	}
	program, ok := programForCommand(os.Args[1])
	if !ok {
		fmt.Fprintln(os.Stderr, "usage: kubiad <generate-web-application|generate-worker-application|generate-scalable-web-application> --output <directory> [--module <module-path>]")
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

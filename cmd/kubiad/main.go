package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/FunnyFoXD/kubiad/internal/generator"
	"github.com/FunnyFoXD/kubiad/internal/ir"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "generate-web-application" {
		fmt.Fprintln(os.Stderr, "usage: kubiad generate-web-application --output <directory> [--module <module-path>]")
		os.Exit(2)
	}
	flags := flag.NewFlagSet("generate-web-application", flag.ExitOnError)
	output := flags.String("output", "", "Generated project directory")
	module := flags.String("module", "", "Generated Go module path")
	_ = flags.Parse(os.Args[2:])
	if *output == "" {
		fmt.Fprintln(os.Stderr, "--output is required")
		os.Exit(2)
	}
	if err := generator.Generate(ir.WebApplication(), *output, generator.Options{Module: *module}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

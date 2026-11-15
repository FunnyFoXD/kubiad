package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/FunnyFoXD/kubiad/internal/evaluation"
)

func main() {
	suite := flag.String("suite", "unit", "Evaluation suite: unit, envtest, or kind")
	output := flag.String("output", "", "New report directory")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	if *output == "" {
		*output = filepath.Join("reports", *suite+"-"+time.Now().UTC().Format("20060102T150405.000000000Z"))
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 40*time.Minute)
	defer cancel()
	if err := evaluation.Run(ctx, *suite, *output, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

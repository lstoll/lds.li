package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	ctx := context.Background()

	switch os.Args[1] {
	case "generate":
		runGenerate(ctx, logger, os.Args[2:])
	case "sync":
		runSync(ctx, logger, os.Args[2:])
	case "cf":
		runCF(ctx, logger, os.Args[2:])
	case "deploy":
		runDeployAll(ctx, logger, os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s <command> [args]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\nCommands:\n")
	fmt.Fprintf(os.Stderr, "  generate    Generate the static site\n")
	fmt.Fprintf(os.Stderr, "  sync        Sync the static site to S3\n")
	fmt.Fprintf(os.Stderr, "  cf          Manage CloudFront functions\n")
	fmt.Fprintf(os.Stderr, "  deploy      Shortcut to sync site and deploy function\n")
}

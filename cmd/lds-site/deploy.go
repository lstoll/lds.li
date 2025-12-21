package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
)

func runDeployAll(ctx context.Context, logger *slog.Logger, args []string) {
	fs := flag.NewFlagSet("deploy", flag.ExitOnError)
	
	// Sync Flags
	bucket := fs.String("bucket", "", "S3 bucket name")
	dir := fs.String("dir", "build", "Directory to sync")
	generate := fs.Bool("generate", true, "Generate site before syncing")
	distributionID := fs.String("distribution-id", "", "CloudFront distribution ID to invalidate")
	
	// CF Deploy Flags
	functionARN := fs.String("function-arn", "", "CloudFront Function Name or ARN (must exist)")
	stage := fs.String("stage", "LIVE", "Stage (DEVELOPMENT or LIVE)")
	configFile := fs.String("config", "site.yaml", "Site configuration file")
	runTests := fs.Bool("test", true, "Run tests after updating development stage")

	// Shared Flags
	emailAddr := fs.String("email", os.Getenv("EMAIL_ADDRESS"), "Email address")

	awsAuth := addAWSAuthFlags(fs)

	fs.Parse(args)

	if err := parseEnvFlags(fs); err != nil {
		logger.Error("Failed to parse env flags", "error", err)
		os.Exit(1)
	}

	// Validate required flags for both
	if *bucket == "" {
		logger.Error("Bucket name is required")
		os.Exit(1)
	}
	if *functionARN == "" {
		logger.Error("Function name or ARN is required")
		os.Exit(1)
	}
	if *emailAddr == "" {
		logger.Error("Email address is required")
		os.Exit(1)
	}

	cfg, err := awsAuth.Load(ctx)
	if err != nil {
		logger.Error("Failed to load AWS config", "error", err)
		os.Exit(1)
	}

	// Run Sync
	logger.Info("Starting Site Sync...")
	if err := doSync(ctx, logger, cfg, *bucket, *dir, *generate, *emailAddr, *distributionID); err != nil {
		logger.Error("Sync failed", "error", err)
		os.Exit(1)
	}

	// Run CF Deploy
	logger.Info("Starting CloudFront Deploy...")
	if err := doCFDeploy(ctx, logger, cfg, *functionARN, *stage, *emailAddr, *configFile, *runTests); err != nil {
		logger.Error("CloudFront Deploy failed", "error", err)
		os.Exit(1)
	}

	logger.Info("Full deployment completed successfully.")
}

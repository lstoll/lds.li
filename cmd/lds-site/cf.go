package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
)

func runCF(ctx context.Context, logger *slog.Logger, args []string) {
	if len(args) < 1 {
		logger.Error("Subcommand required: deploy, test")
		os.Exit(1)
	}

	switch args[0] {
	case "deploy":
		runCFDeploy(ctx, logger, args[1:])
	case "test":
		runCFTest(ctx, logger, args[1:])
	default:
		logger.Error("Unknown subcommand", "command", args[0])
		os.Exit(1)
	}
}

func getFunctionName(input string) string {
	if strings.HasPrefix(input, "arn:aws:cloudfront::") {
		parts := strings.Split(input, ":")
		if len(parts) >= 6 { // arn:aws:cloudfront::<account-id>:function/my-function
			resource := parts[len(parts)-1] // function/my-function
			nameParts := strings.SplitN(resource, "/", 2)
			if len(nameParts) == 2 {
				return nameParts[1]
			}
		}
	}
	return input // Assume it's already the name
}

func runCFDeploy(ctx context.Context, logger *slog.Logger, args []string) {
	fs := flag.NewFlagSet("cf deploy", flag.ExitOnError)
	nameInput := fs.String("function-arn", "", "CloudFront Function Name or ARN (must exist)")
	stage := fs.String("stage", "LIVE", "Stage (DEVELOPMENT or LIVE)")
	emailAddr := fs.String("email", os.Getenv("EMAIL_ADDRESS"), "Email address")
	configFile := fs.String("config", "site.yaml", "Site configuration file")
	runTests := fs.Bool("test", true, "Run tests after updating development stage")

	awsAuth := addAWSAuthFlags(fs)

	fs.Parse(args)

	if err := parseEnvFlags(fs); err != nil {
		logger.Error("Failed to parse env flags", "error", err)
		os.Exit(1)
	}

	cfg, err := awsAuth.Load(ctx)
	if err != nil {
		logger.Error("Failed to load AWS config", "error", err)
		os.Exit(1)
	}

	if err := doCFDeploy(ctx, logger, cfg, *nameInput, *stage, *emailAddr, *configFile, *runTests); err != nil {
		logger.Error("Deploy failed", "error", err)
		os.Exit(1)
	}
}

func doCFDeploy(ctx context.Context, logger *slog.Logger, cfg aws.Config, nameInput, stage, emailAddr, configFile string, runTests bool) error {
	if nameInput == "" {
		return context.DeadlineExceeded // Just a placeholder error? No, fmt.Errorf
	}
	// Better check empty string before calling if possible, but here:
	if nameInput == "" {
		return fmt.Errorf("function name or ARN is required")
	}
	if emailAddr == "" {
		return fmt.Errorf("email is required")
	}

	siteCfg, err := LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load site config: %w", err)
	}

	functionName := getFunctionName(nameInput)

	// Prepare Code
	modJSON, _ := json.Marshal(siteCfg.Modules)
	wfJSON, _ := json.Marshal(siteCfg.Webfinger)

	// Read template
	tmplContent, err := os.ReadFile("cmd/lds-site/function.tmpl.js")
	if err != nil {
		return fmt.Errorf("failed to read function template: %w", err)
	}

	codeStr := string(tmplContent)
	codeStr = strings.Replace(codeStr, "var moduleRegistry = {}; // %%MODULES_JSON%%", "var moduleRegistry = "+string(modJSON)+";", 1)
	codeStr = strings.Replace(codeStr, "var webfingerLinks = []; // %%WEBFINGER_JSON%%", "var webfingerLinks = "+string(wfJSON)+";", 1)
	codeStr = strings.Replace(codeStr, "var email = \"\"; // %%EMAIL%%", "var email = \""+emailAddr+"\";", 1)
	codeStr = strings.Replace(codeStr, "var canonicalHost = \"\"; // %%CANONICAL_HOST%%", "var canonicalHost = \""+siteCfg.CanonicalHost+"\";", 1)

	functionCode := []byte(codeStr)

	client := cloudfront.NewFromConfig(cfg)

	// Get existing function configuration (DescribeFunction)
	logger.Info("Getting function configuration", "name", functionName)
	descOut, err := client.DescribeFunction(ctx, &cloudfront.DescribeFunctionInput{
		Name:  &functionName,
		Stage: types.FunctionStage("DEVELOPMENT"),
	})

	if err != nil {
		return fmt.Errorf("failed to describe function %s: %w", functionName, err)
	}

	etag := descOut.ETag
	logger.Info("Function exists, updating", "etag", *etag)

	updateOut, err := client.UpdateFunction(ctx, &cloudfront.UpdateFunctionInput{
		Name:           &functionName,
		IfMatch:        etag,
		FunctionConfig: descOut.FunctionSummary.FunctionConfig,
		FunctionCode:   functionCode,
	})
	if err != nil {
		return fmt.Errorf("failed to update function: %w", err)
	}
	etag = updateOut.ETag

	logger.Info("Function updated in DEVELOPMENT")

	if runTests {
		logger.Info("Running tests against DEVELOPMENT stage")
		if err := RunTests(ctx, client, functionName, *etag, logger); err != nil {
			return fmt.Errorf("tests failed, aborting deployment: %w", err)
		}
		logger.Info("Tests passed")
	}

	if stage == "LIVE" {
		logger.Info("Publishing function to LIVE")
		_, err = client.PublishFunction(ctx, &cloudfront.PublishFunctionInput{
			Name:    &functionName,
			IfMatch: etag,
		})
		if err != nil {
			return fmt.Errorf("failed to publish function: %w", err)
		}
		logger.Info("Function published")
	}
	return nil
}

func runCFTest(ctx context.Context, logger *slog.Logger, args []string) {
	fs := flag.NewFlagSet("cf test", flag.ExitOnError)
	nameInput := fs.String("function-arn", "", "CloudFront Function Name or ARN (must exist)")

	awsAuth := addAWSAuthFlags(fs)

	fs.Parse(args)

	if err := parseEnvFlags(fs); err != nil {
		logger.Error("Failed to parse env flags", "error", err)
		os.Exit(1)
	}

	if *nameInput == "" {
		logger.Error("Function name or ARN is required")
		os.Exit(1)
	}

	functionName := getFunctionName(*nameInput)

	cfg, err := awsAuth.Load(ctx)
	if err != nil {
		logger.Error("Failed to load AWS config", "error", err)
		os.Exit(1)
	}

	client := cloudfront.NewFromConfig(cfg)

	// Get ETag
	logger.Info("Getting function configuration for test", "name", functionName)
	descOut, err := client.DescribeFunction(ctx, &cloudfront.DescribeFunctionInput{
		Name:  &functionName,
		Stage: types.FunctionStage("DEVELOPMENT"),
	})
	if err != nil {
		logger.Error("Failed to describe function. Ensure the function exists and you have permissions.", "name", functionName, "error", err)
		os.Exit(1)
	}

	if err := RunTests(ctx, client, functionName, *descOut.ETag, logger); err != nil {
		logger.Error("Tests failed", "error", err)
		os.Exit(1)
	}
}

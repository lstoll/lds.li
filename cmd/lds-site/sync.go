package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func runSync(ctx context.Context, logger *slog.Logger, args []string) {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	bucket := fs.String("bucket", "", "S3 bucket name")
	dir := fs.String("dir", "build", "Directory to sync")
	generate := fs.Bool("generate", true, "Generate site before syncing")
	emailAddr := fs.String("email", os.Getenv("EMAIL_ADDRESS"), "Email address (required if generate is true)")

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

	if err := doSync(ctx, logger, cfg, *bucket, *dir, *generate, *emailAddr); err != nil {
		logger.Error("Sync failed", "error", err)
		os.Exit(1)
	}
}

func doSync(ctx context.Context, logger *slog.Logger, cfg aws.Config, bucket, dir string, generate bool, emailAddr string) error {
	if bucket == "" {
		return fmt.Errorf("bucket name is required")
	}

	if generate {
		if emailAddr == "" {
			return fmt.Errorf("email address is required for generation")
		}
		logger.Info("Generating site...")
		if err := generateSite(ctx, logger, dir, emailAddr); err != nil {
			return fmt.Errorf("generation failed: %w", err)
		}
	}

	s3Client := s3.NewFromConfig(cfg)
	uploader := manager.NewUploader(s3Client)

	logger.Info("Syncing directory to S3", "dir", dir, "bucket", bucket)

	// List existing objects for pruning
	existingObjects := make(map[string]bool)
	paginator := s3.NewListObjectsV2Paginator(s3Client, &s3.ListObjectsV2Input{
		Bucket: &bucket,
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list objects: %w", err)
		}
		for _, obj := range page.Contents {
			existingObjects[*obj.Key] = true
		}
	}

	walker := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		// S3 uses / as separator
		key := filepath.ToSlash(relPath)

		// Mark as present locally
		delete(existingObjects, key)

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		contentType := getContentType(path)

		logger.Info("Uploading", "key", key)
		_, err = uploader.Upload(ctx, &s3.PutObjectInput{
			Bucket:      &bucket,
			Key:         aws.String(key),
			Body:        f,
			ContentType: aws.String(contentType),
		})
		if err != nil {
			return err
		}
		return nil
	}

	if err := filepath.Walk(dir, walker); err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	// Prune removed files
	if len(existingObjects) > 0 {
		logger.Info("Pruning removed files", "count", len(existingObjects))
		var toDelete []types.ObjectIdentifier
		for key := range existingObjects {
			logger.Info("Deleting", "key", key)
			toDelete = append(toDelete, types.ObjectIdentifier{Key: aws.String(key)})
		}

		// Batch delete (max 1000 per request)
		for i := 0; i < len(toDelete); i += 1000 {
			end := i + 1000
			if end > len(toDelete) {
				end = len(toDelete)
			}
			batch := toDelete[i:end]
			_, err := s3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
				Bucket: &bucket,
				Delete: &types.Delete{
					Objects: batch,
				},
			})
			if err != nil {
				return fmt.Errorf("failed to delete objects: %w", err)
			}
		}
	}

	logger.Info("Sync complete")
	return nil
}

func getContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".svg":
		return "image/svg+xml"
	case ".xml":
		return "application/xml"
	default:
		return "application/octet-stream"
	}
}

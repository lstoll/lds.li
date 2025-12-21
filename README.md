# lds.li Site Tooling

This repository contains the source code and tooling for [lds.li](https://lds.li).

The site uses a simple static generator and deploys to AWS S3 + CloudFront.
Dynamic logic (redirects, Go modules, Webfinger) is handled by CloudFront Functions.

## Usage

The CLI tool `lds-site` manages the site lifecycle.

```bash
# Build the tool
go build -o lds-site ./cmd/lds-site

# Deploy everything (sync site + update CloudFront function)
export LDS_SITE_EMAIL="me@example.com"
export LDS_SITE_BUCKET="my-bucket-name"
export LDS_SITE_FUNCTION_ARN="my-function-arn"
./lds-site deploy
```

Configuration is handled in `site.yaml`.
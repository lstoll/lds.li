# lds.li Site Tooling

This repository contains the source code and tooling for [lds.li](https://lds.li).
The site is statically generated and hosted on AWS S3 + CloudFront.
Dynamic logic (redirects, Go modules, Webfinger) is handled by CloudFront Functions.

## Prerequisites

- Go 1.25+
- AWS Credentials configured (e.g. `~/.aws/credentials` or environment variables)

## Tooling

The CLI tool `lds-site` manages the site lifecycle.

### Build the Tool

```bash
go build -o lds-site ./cmd/lds-site
```

### 1. Generate Static Site

Generates the `index.html` (with email proof-of-work) and copies static assets to the output directory (`build/` by default).

```bash
export LDS_SITE_EMAIL="me@example.com"
./lds-site generate -out build
```

### 2. Sync to S3

Syncs the generated directory to the S3 bucket.

By default, this command **generates the site** before syncing and **prunes** any files in the S3 bucket that are not present locally.

```bash
# Generate and Sync (recommended)
export LDS_SITE_EMAIL="me@example.com"
export LDS_SITE_BUCKET="my-bucket-name"
./lds-site sync

# Sync existing build only (no generation)
./lds-site sync -bucket my-bucket-name -generate=false -dir build
```

### 3. Deploy CloudFront Function

Updates the CloudFront Function that handles:
- `www.` to apex redirects.
- Go module vanity imports and documentation redirects.
- Webfinger responses.

The function logic is in `cmd/lds-site/function.js.tmpl`. Configuration (modules, webfinger) is injected from `internal/config/config.go`.

**Note:** The CloudFront function must be pre-provisioned (e.g., using Terraform) as this tool only updates existing functions, it does not create them.

When deploying, the tool automatically runs a suite of tests against the **DEVELOPMENT** stage. If these tests pass, the function is **automatically published to LIVE** by default.

To deploy, test, and publish to **LIVE** (default):
```bash
./lds-site cf deploy -function-arn MyFunction # or MyFunction ARN
```

To deploy to **DEVELOPMENT** stage only (skip promotion to LIVE):
```bash
./lds-site cf deploy -function-arn MyFunction -stage DEVELOPMENT
```

To run tests manually against the DEVELOPMENT stage:
```bash
./lds-site cf test -function-arn MyFunction # or MyFunction ARN
```

### Configuration

### Environment Variables & Flags

All CLI flags can be set via environment variables. The variable name is the flag name, upper-cased, prefixed with `LDS_SITE_`, and with dashes replaced by underscores.

**General:**
- `-email` / `LDS_SITE_EMAIL`: Email address (for homepage and webfinger).

**AWS Authentication:**
By default, the tool uses the standard AWS credential chain (profile, env vars). To use the custom OIDC authentication:
- `-oidc-issuer` / `LDS_SITE_OIDC_ISSUER`: OIDC Issuer URL (default: `https://id.lds.li`)
- `-oidc-client-id` / `LDS_SITE_OIDC_CLIENT_ID`: OIDC Client ID (default: `sts.amazonaws.com`)
- `-oidc-client-secret` / `LDS_SITE_OIDC_CLIENT_SECRET`: OIDC Client Secret (default: `public`)
- `-aws-role-arn` / `LDS_SITE_AWS_ROLE_ARN`: IAM Role ARN to assume (default: `arn:aws:iam::041050768191:role/lstoll-admin`)
- `-aws-region` / `LDS_SITE_AWS_REGION`: AWS Region (default: `us-east-1`)

**Sync:**
- `-bucket` / `LDS_SITE_BUCKET`: S3 Bucket name.
- `-dir` / `LDS_SITE_DIR`: Build directory (default: `build`).
- `-generate` / `LDS_SITE_GENERATE`: boolean (default: `true`).

**CloudFront:**
- `-function-arn` / `LDS_SITE_FUNCTION_ARN`: CloudFront Function Name or ARN.
- `-stage` / `LDS_SITE_STAGE`: `DEVELOPMENT` or `LIVE` (default: `LIVE`).
- `-test` / `LDS_SITE_TEST`: boolean (default: `true`).
- `-config` / `LDS_SITE_CONFIG`: Path to site configuration file (default: `site.yaml`).



### Application Config



Configuration for Go modules and Webfinger is defined in `site.yaml` in the project root.



Example `site.yaml`:



```yaml



canonical_host: lds.li # The main hostname for the site. All other hostnames will redirect here.



modules:



  oauth2ext:



    path: lds.li/oauth2ext



    git_url: https://github.com/lstoll/oauth2ext



    # redirect_to is optional. If set, it forces a fixed redirect to this URL.



    # If unset, it defaults to pkg.go.dev/path (with subpath support).



    redirect_to: "" 



  oidccli:



    path: lds.li/oidccli



    git_url: https://github.com/lstoll/oidccli



    redirect_to: https://github.com/lstoll/oidccli







webfinger:

  - rel: http://openid.net/specs/connect/1.0/issuer

    href: https://id.lds.li

```

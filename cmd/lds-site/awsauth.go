package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"golang.org/x/oauth2"
	"lds.li/oauth2ext/clitoken"
	"lds.li/oauth2ext/oidc"
	"lds.li/oauth2ext/provider"
)

type AWSAuthConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RoleARN      string
	Region       string
}

func addAWSAuthFlags(fs *flag.FlagSet) *AWSAuthConfig {
	c := &AWSAuthConfig{}
	fs.StringVar(&c.Issuer, "oidc-issuer", "https://id.lds.li", "OIDC Issuer URL")
	fs.StringVar(&c.ClientID, "oidc-client-id", "sts.amazonaws.com", "OIDC Client ID")
	fs.StringVar(&c.ClientSecret, "oidc-client-secret", "public", "OIDC Client Secret")
	fs.StringVar(&c.RoleARN, "aws-role-arn", "arn:aws:iam::041050768191:role/lstoll-admin", "AWS Role ARN")
	fs.StringVar(&c.Region, "aws-region", "us-east-1", "AWS Region")
	return c
}

func (c *AWSAuthConfig) Load(ctx context.Context) (aws.Config, error) {
	// If RoleARN is empty, assume standard AWS credential chain
	if c.RoleARN == "" {
		return config.LoadDefaultConfig(ctx, config.WithRegion(c.Region))
	}

	prov, err := provider.DiscoverOIDCProvider(ctx, c.Issuer)
	if err != nil {
		return aws.Config{}, fmt.Errorf("discovering issuer %s: %w", c.Issuer, err)
	}

	oa2Cfg := oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		Endpoint:     prov.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID},
	}
	clitokCfg := clitoken.Config{
		OAuth2Config: oa2Cfg,
	}

	ts, err := clitokCfg.TokenSource(ctx)
	if err != nil {
		return aws.Config{}, fmt.Errorf("getting cli token source: %w", err)
	}
	// Create STS client with no credentials (we'll use web identity)
	baseCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(c.Region))
	if err != nil {
		return aws.Config{}, fmt.Errorf("loading AWS config: %w", err)
	}

	stsClient := sts.NewFromConfig(baseCfg)

	// Create the credentials provider
	creds := stscreds.NewWebIdentityRoleProvider(
		stsClient,
		c.RoleARN,
		identityTokenSource{ts},
	)

	return config.LoadDefaultConfig(ctx,
		config.WithRegion(c.Region),
		config.WithCredentialsProvider(creds),
	)
}

type identityTokenSource struct {
	oauth2.TokenSource
}

func (its identityTokenSource) GetIdentityToken() ([]byte, error) {
	tok, err := its.Token()
	if err != nil {
		return nil, err
	}
	idToken, ok := oidc.GetIDToken(tok)
	if !ok {
		return nil, fmt.Errorf("response has no id_token")
	}
	return []byte(idToken), nil
}

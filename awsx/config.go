package awsx

import (
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	awsCredentials "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/sts"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-multierror"
	terr_aws "github.com/hashicorp/terraform/builtin/providers/aws"
	"github.com/hashicorp/terraform/helper/logging"
	"github.com/hashicorp/terraform/helper/schema"
)

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	c := terr_aws.Config{
		AccessKey:     d.Get("access_key").(string),
		SecretKey:     d.Get("secret_key").(string),
		Profile:       d.Get("profile").(string),
		CredsFilename: d.Get("shared_credentials_file").(string),
		Token:         d.Get("token").(string),
		Region:        d.Get("region").(string),
		MaxRetries:    d.Get("max_retries").(int),
	}

	// Get the auth and region. This can fail if keys/regions were not
	// specified and we're attempting to use the environment.
	var errs []error
	var elasticacheconn *elasticache.ElastiCache

	log.Println("[INFO] Building AWS region structure")
	err := c.ValidateRegion()
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		log.Println("[INFO] Building AWS auth structure")
		creds := GetCredentials(c.AccessKey, c.SecretKey, c.Token, c.Profile, c.CredsFilename)
		// Call Get to check for credential provider. If nothing found, we'll get an
		// error, and we can present it nicely to the user
		cp, err := creds.Get()
		if err != nil {
			if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == "NoCredentialProviders" {
				errs = append(errs, fmt.Errorf(`No valid credential sources found for AWS Provider.
  Please see https://terraform.io/docs/providers/aws/index.html for more information on
  providing credentials for the AWS Provider`))
			} else {
				errs = append(errs, fmt.Errorf("Error loading credentials for AWS Provider: %s", err))
			}
			return nil, &multierror.Error{Errors: errs}
		}

		log.Printf("[INFO] AWS Auth provider used: %q", cp.ProviderName)

		awsConfig := &aws.Config{
			Credentials: creds,
			Region:      aws.String(c.Region),
			MaxRetries:  aws.Int(c.MaxRetries),
			HTTPClient:  cleanhttp.DefaultClient(),
		}

		if logging.IsDebugOrHigher() {
			awsConfig.LogLevel = aws.LogLevel(aws.LogDebugWithHTTPBody)
			awsConfig.Logger = awsLogger{}
		}

		// Set up base session
		sess := session.New(awsConfig)

		stsconn := sts.New(sess)
		if _, err := stsconn.GetCallerIdentity(&sts.GetCallerIdentityInput{}); err != nil {
			errs = append(errs, err)
			return nil, &multierror.Error{Errors: errs}
		}

		elasticacheconn = elasticache.New(sess)
	}

	if len(errs) > 0 {
		return nil, &multierror.Error{Errors: errs}
	}

	return elasticacheconn, nil
}

// This function is responsible for reading credentials from the
// environment in the case that they're not explicitly specified
// in the Terraform configuration.
func GetCredentials(key, secret, token, profile, credsfile string) *awsCredentials.Credentials {
	// build a chain provider, lazy-evaulated by aws-sdk
	providers := []awsCredentials.Provider{
		&awsCredentials.StaticProvider{Value: awsCredentials.Value{
			AccessKeyID:     key,
			SecretAccessKey: secret,
			SessionToken:    token,
		}},
		&awsCredentials.EnvProvider{},
		&awsCredentials.SharedCredentialsProvider{
			Filename: credsfile,
			Profile:  profile,
		},
	}

	// Build isolated HTTP client to avoid issues with globally-shared settings
	client := cleanhttp.DefaultClient()

	// Keep the timeout low as we don't want to wait in non-EC2 environments
	client.Timeout = 100 * time.Millisecond
	cfg := &aws.Config{
		HTTPClient: client,
	}

	// Real AWS should reply to a simple metadata request.
	// We check it actually does to ensure something else didn't just
	// happen to be listening on the same IP:Port
	metadataClient := ec2metadata.New(session.New(cfg))
	if metadataClient.Available() {
		providers = append(providers, &ec2rolecreds.EC2RoleProvider{
			Client: metadataClient,
		})
		log.Printf("[INFO] AWS EC2 instance detected via default metadata" +
			" API endpoint, EC2RoleProvider added to the auth chain")
	}

	return awsCredentials.NewChainCredentials(providers)
}

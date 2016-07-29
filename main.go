package main

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
	//"github.com/aws/aws-sdk-go/service/iam"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-multierror"
	//"github.com/hashicorp/terraform/plugin"
)

func main() {
	//plugin.Serve(new(MyPlugin))

	conn, err := initClient()
	if err != nil {
		panic(err)
	}

	tags := make([]*elasticache.Tag, 0)
	cache_security_groups := make([]*string, 0)
	security_groups := []*string{aws.String("sg-3d7a355a")}
	repl_group_id := "testt"

	describe_req := &elasticache.DescribeReplicationGroupsInput{
		ReplicationGroupId: aws.String(repl_group_id),
	}

	desc_resp, err := conn.DescribeReplicationGroups(describe_req)
	fmt.Println(desc_resp)
	if err != nil {
		if eccErr, ok := err.(awserr.Error); ok && eccErr.Code() == "ReplicationGroupNotFoundFault" {
			desc_resp = nil
		} else {
			panic(err)
		}
	}

	if desc_resp == nil {
		req := &elasticache.CreateReplicationGroupInput{
			ReplicationGroupId:          aws.String(repl_group_id),
			ReplicationGroupDescription: aws.String("testt repl group"),
			AutomaticFailoverEnabled:    aws.Bool(true),
			CacheNodeType:               aws.String("cache.r3.large"),
			CacheSecurityGroupNames:     cache_security_groups,
			SecurityGroupIds:            security_groups,
			CacheSubnetGroupName:        aws.String("base"),
			NumCacheClusters:            aws.Int64(2),
			Engine:                      aws.String("redis"),
			Port:                        aws.Int64(6379),
			Tags:                        tags,
			// CacheParameterGroupName:
			// CacheClusterId:          aws.String(clusterId),
			// EngineVersion:        aws.String(engineVersion),
		}

		resp, err := conn.CreateReplicationGroup(req)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%#v\n", resp)
	}

	if err := conn.WaitUntilReplicationGroupAvailable(describe_req); err != nil {
		panic(err)
	}

	delete_req := &elasticache.DeleteReplicationGroupInput{
		// TODO keep primary
		ReplicationGroupId: aws.String(repl_group_id),
	}
	del_resp, err := conn.DeleteReplicationGroup(delete_req)
	if err != nil {
		panic(err)
	}
	fmt.Println(del_resp)
	if err := conn.WaitUntilReplicationGroupDeleted(describe_req); err != nil {
		panic(err)
	}
}

func initClient() (*elasticache.ElastiCache, error) {
	var errs []error
	// TODO c.AccessKey, c.SecretKey, c.Token, c.Profile, c.CredsFilename
	creds := GetCredentials("", "", "", "", "")
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
		Region:      aws.String("eu-west-1"),
		MaxRetries:  aws.Int(3),
		HTTPClient:  cleanhttp.DefaultClient(),
	}
	sess := session.New(awsConfig)
	conn := elasticache.New(sess)

	return conn, nil
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

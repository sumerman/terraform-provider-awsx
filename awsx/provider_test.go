package awsx

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/elasticache"

	terr_aws "github.com/hashicorp/terraform/builtin/providers/aws"
	"github.com/hashicorp/terraform/helper/acctest"
	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
)

var testAccProviders map[string]terraform.ResourceProvider
var testAccProvider *schema.Provider
var builtinAws *schema.Provider

func init() {
	builtinAws = terr_aws.Provider().(*schema.Provider)
	testAccProvider = Provider().(*schema.Provider)
	testAccProviders = map[string]terraform.ResourceProvider{
		"aws":  builtinAws,
		"awsx": testAccProvider,
	}
}

func TestProvider(t *testing.T) {
	if err := Provider().(*schema.Provider).InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestProvider_impl(t *testing.T) {
	var _ terraform.ResourceProvider = Provider()
}

func testAccPreCheck(t *testing.T) {
	if v := os.Getenv("AWS_ACCESS_KEY_ID"); v == "" {
		t.Fatal("AWS_ACCESS_KEY_ID must be set for acceptance tests")
	}
	if v := os.Getenv("AWS_SECRET_ACCESS_KEY"); v == "" {
		t.Fatal("AWS_SECRET_ACCESS_KEY must be set for acceptance tests")
	}
	if v := os.Getenv("AWS_DEFAULT_REGION"); v == "" {
		log.Println("[INFO] Test: Using us-west-2 as test region")
		os.Setenv("AWS_DEFAULT_REGION", "us-west-2")
	}
}

func TestAccAWSElasticacheReplicationGroup_basic(t *testing.T) {
	var rg elasticache.ReplicationGroup
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSElasticacheReplicationGroupDestroy,
		Steps: []resource.TestStep{
			resource.TestStep{
				Config: testAccAWSElasticacheReplicationGroupConfig,
				Check:  testAccCheckAWSElasticacheReplicationGroupExists("awsx_elasticache_replication_group.bar", &rg),
			},
		},
	})
}

func testAccCheckAWSElasticacheReplicationGroupExists(n string, v *elasticache.ReplicationGroup) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		fmt.Println(s)
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No cache cluster ID is set")
		}

		conn := testAccProvider.Meta().(*elasticache.ElastiCache)
		resp, err := conn.DescribeReplicationGroups(&elasticache.DescribeReplicationGroupsInput{
			ReplicationGroupId: aws.String(rs.Primary.ID),
		})
		if err != nil {
			return fmt.Errorf("Elasticache error: %v", err)
		}

		for _, c := range resp.ReplicationGroups {
			if *c.ReplicationGroupId == rs.Primary.ID {
				*v = *c
			}
		}

		return nil
	}
}

func testAccCheckAWSElasticacheReplicationGroupDestroy(s *terraform.State) error {
	conn := testAccProvider.Meta().(*elasticache.ElastiCache)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "awsx_elasticache_replication_group" {
			continue
		}
		res, err := conn.DescribeReplicationGroups(&elasticache.DescribeReplicationGroupsInput{
			ReplicationGroupId: aws.String(rs.Primary.ID),
		})
		if err != nil {
			// Verify the error is what we want
			if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == "ReplicationGroupNotFoundFault" {
				continue
			}
			return err
		}
		if len(res.ReplicationGroups) > 0 {
			return fmt.Errorf("still exist.")
		}
	}
	return nil
}

var testAccAWSElasticacheReplicationGroupConfig = fmt.Sprintf(`
provider "aws" {
	region = "eu-west-1"
}
provider "awsx" {
	region = "eu-west-1"
}
resource "aws_security_group" "bar" {
    name = "tf-test-security-group-%03d"
    description = "tf-test-security-group-descr"
    ingress {
        from_port = -1
        to_port = -1
        protocol = "icmp"
        cidr_blocks = ["0.0.0.0/0"]
    }
}

resource "aws_elasticache_security_group" "bar" {
    name = "tf-test-security-group-%03d"
    description = "tf-test-security-group-descr"
    security_group_names = ["${aws_security_group.bar.name}"]
}

resource "awsx_elasticache_replication_group" "bar" {
    replication_group_id = "tf-%s"
    node_type = "cache.m1.small"
    num_cache_clusters = 2
    port = 11211
    parameter_group_name = "default.redis2.8"
    security_group_names = ["${aws_elasticache_security_group.bar.name}"]
}
`, acctest.RandInt(), acctest.RandInt(), acctest.RandString(10))

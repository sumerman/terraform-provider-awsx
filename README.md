# Terraform AWSX provider

**WIP**

- [X] Initial CRD callbacks + test
- [X] Binary
- [ ] Update
- [ ] Tags
- [ ] Creating with an existent primary and leaving it be after a deletion
- [ ] Better test coverage

Resources missing from the builtin AWS provider:

- Elasticache Replication Groups

Code is heavily based on the builtin AWS provider.

## Usage

Terraform config:

```
provider "aws" {
	region = "eu-west-1"
}
provider "awsx" {
	region = "eu-west-1"
}
resource "aws_security_group" "bar" {
    name = "my-sec-group"
    description = "my-sec-group"
    ingress {
        from_port = -1
        to_port = -1
        protocol = "icmp"
        cidr_blocks = ["0.0.0.0/0"]
    }
}

resource "aws_elasticache_security_group" "bar" {
    name = "my-elc-sec-grp"
    description = "my-elc-sec-grp"
    security_group_names = ["${aws_security_group.bar.name}"]
}

resource "awsx_elasticache_replication_group" "bar" {
    replication_group_id = "my-repl-group"
    node_type = "cache.m1.small"
    num_cache_clusters = 2
    port = 6380
    parameter_group_name = "default.redis2.8"
    security_group_names = ["${aws_elasticache_security_group.bar.name}"]
}
```

Plugin setup is described [here](https://www.terraform.io/docs/plugins/basics.html)

## Run acceptance tests

In order to run the test suite you have to specify AWS environment variables and execute `TF_ACC=true TF_LOG=DEBUG go test -v -timeout 20m`

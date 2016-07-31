# Terraform AWSX provider

Code is heavily based on the builtin AWS provider.

Resources missing from the builtin AWS provider:

- Elasticache Replication Groups  
  Unfortunately there is an AWS API limitation that does not allow to update a number of replicas without manipulating individual cache clusters that comprise a replication group. There is a way to work around this: add a read replica manually and then change your tf config. **WIP**:

	- [X] Initial CRD callbacks + test
	- [X] Binary
	- [X] Update
	- [ ] Tags
	- [ ] Increase a number of read replicas
	- [ ] Decrease a number of read replicas
	- [ ] Creation with an existent primary and leaving it be after a deletion

## Usage

- `go build`
- `go get` (Due to some internal Terraform plugin API changes you may need to **checkout a specific version**)
-  copy `terraform-provider-awsx` binary to your terraform workspace (same place where your `*.tf` files are)

Terraform config may look like:

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

## Run acceptance tests

In order to run the test suite you have to specify AWS environment variables and execute `cd awsx; TF_ACC=true TF_LOG=DEBUG go test -v -timeout 120m`

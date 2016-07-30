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

## Run acceptance tests

In order to run the test suite you have to specify AWS environment variables and execute `TF_ACC=true TF_LOG=DEBUG go test -v -timeout 20m`

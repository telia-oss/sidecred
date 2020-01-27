terraform {
  required_version = "~> 0.12.24"
}

provider "aws" {
  version = "~> 2.61.0"
  region  = "eu-west-1"
}

module "sidecred" {
  source = "../"

  name_prefix = "sidecred-example"
  binary_path = "../../dist/sidecred-lambda_linux_amd64/sidecred-lambda"

  configurations = [{
    namespace = "example"
    config    = "./config.yml"
  }]

  tags = {
    terraform   = "true"
    environment = "dev"
  }
}

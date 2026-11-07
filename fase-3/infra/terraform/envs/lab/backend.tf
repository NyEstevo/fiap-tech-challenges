terraform {
  backend "s3" {
    bucket         = "tc-fiap-tfstate-730335507360"
    key            = "fase-3/lab/terraform.tfstate"
    region         = "us-east-1"
    dynamodb_table = "tc-fiap-tflock"
    encrypt        = true
  }
}

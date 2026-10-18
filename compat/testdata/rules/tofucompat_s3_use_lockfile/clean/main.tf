terraform {
  backend "s3" {
    bucket                      = "tfstate"
    key                         = "app.tfstate"
    region                      = "eu-west-1"
    dynamodb_table              = "tfstate-lock"
    skip_credentials_validation = true
  }
}

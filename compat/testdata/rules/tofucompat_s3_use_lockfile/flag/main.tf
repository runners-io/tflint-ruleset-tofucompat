terraform {
  backend "s3" {
    bucket                      = "tfstate"
    key                         = "app.tfstate"
    region                      = "eu-west-1"
    use_lockfile                = true
    skip_credentials_validation = true
  }
}

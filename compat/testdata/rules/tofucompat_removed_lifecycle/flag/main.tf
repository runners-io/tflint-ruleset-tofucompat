removed {
  from = terraform_data.legacy

  lifecycle {
    destroy = false
  }
}

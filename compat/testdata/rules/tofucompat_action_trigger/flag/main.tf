resource "terraform_data" "deploy" {
  input = "v1"

  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.terraform_data.ping]
    }
  }
}

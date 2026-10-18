variable "min_size" {
  type    = number
  default = 1
}

variable "max_size" {
  type    = number
  default = 3

  validation {
    condition     = var.max_size >= var.min_size
    error_message = "max_size must be at least min_size."
  }
}

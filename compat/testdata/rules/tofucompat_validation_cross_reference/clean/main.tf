variable "min_size" {
  type    = number
  default = 1
}

variable "max_size" {
  type    = number
  default = 3

  validation {
    condition     = var.max_size >= 1 && alltrue([for n in [var.max_size] : n < 100])
    error_message = "max_size must be between 1 and 99."
  }
}

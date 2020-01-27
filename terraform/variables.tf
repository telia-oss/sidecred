# ------------------------------------------------------------------------------
# Variables
# ------------------------------------------------------------------------------
variable "name_prefix" {
  description = "A prefix used for naming resources."
  type        = string
}

variable "binary_path" {
  description = "Path to the autoapprover linux binary."
  type        = string
}

variable "configurations" {
  description = "A list of configurations that will trigger the sidecred lambda."
  type        = list(object({ namespace = string, config = string }))
}

variable "environment" {
  description = "Environment variables for the lambda. This is how you configure sidecred."
  type        = map(string)
  default = {
    SIDECRED_STS_PROVIDER_ENABLED          = "true"
    SIDECRED_STS_PROVIDER_SESSION_DURATION = "20m"
    SIDECRED_SECRET_STORE_BACKEND          = "ssm"
    SIDECRED_SSM_STORE_PATH_TEMPLATE       = "/sidecred/{{ .Namespace }}/{{ .Name }}"
    SIDECRED_DEBUG                         = "true"
  }
}

variable "tags" {
  description = "A map of tags (key-value pairs) passed to resources."
  type        = map(string)
  default     = {}
}

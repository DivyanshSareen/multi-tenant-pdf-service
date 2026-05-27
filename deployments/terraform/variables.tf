variable "namespace" {
  description = "Kubernetes namespace for all resources"
  type        = string
  default     = "pdf-service"
}

variable "postgres_image" {
  type    = string
  default = "postgres:16"
}

variable "postgres_storage" {
  type    = string
  default = "1Gi"
}

variable "postgres_password" {
  type      = string
  sensitive = true
  default   = "secretpass"
}

variable "mongodb_image" {
  type    = string
  default = "mongo:7"
}

variable "mongodb_storage" {
  type    = string
  default = "1Gi"
}

variable "minio_image" {
  type    = string
  default = "minio/minio:latest"
}

variable "minio_storage" {
  type    = string
  default = "5Gi"
}

variable "minio_root_user" {
  type    = string
  default = "minioadmin"
}

variable "minio_root_password" {
  type      = string
  sensitive = true
  default   = "minioadmin"
}

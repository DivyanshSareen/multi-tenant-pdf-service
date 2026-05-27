output "postgres_service_endpoint" {
  description = "In-cluster DNS endpoint for PostgreSQL"
  value       = "postgres-service.${var.namespace}.svc.cluster.local:5432"
}

output "mongodb_service_endpoint" {
  description = "In-cluster DNS endpoint for MongoDB"
  value       = "mongodb-service.${var.namespace}.svc.cluster.local:27017"
}

output "minio_service_endpoint" {
  description = "In-cluster DNS endpoint for MinIO API"
  value       = "minio-service.${var.namespace}.svc.cluster.local:9000"
}

output "namespace" {
  description = "Kubernetes namespace where infra was provisioned"
  value       = var.namespace
}

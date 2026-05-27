terraform {
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.27"
    }
  }
}

# Uses the current kubeconfig context — run `kubectl config use-context kind-pdf-service` before apply.
provider "kubernetes" {
  config_path    = "~/.kube/config"
  config_context = "kind-pdf-service"
}

# ─── Namespace ────────────────────────────────────────────────────────────────

resource "kubernetes_namespace" "pdf_service" {
  metadata {
    name = var.namespace
  }
}

# ─── PostgreSQL ───────────────────────────────────────────────────────────────

resource "kubernetes_persistent_volume_claim" "postgres" {
  metadata {
    name      = "postgres-pvc"
    namespace = kubernetes_namespace.pdf_service.metadata[0].name
  }
  spec {
    access_modes = ["ReadWriteOnce"]
    resources {
      requests = { storage = var.postgres_storage }
    }
  }
}

resource "kubernetes_stateful_set" "postgres" {
  metadata {
    name      = "postgres"
    namespace = kubernetes_namespace.pdf_service.metadata[0].name
  }
  spec {
    service_name = "postgres-service"
    replicas     = 1
    selector {
      match_labels = { app = "postgres" }
    }
    template {
      metadata {
        labels = { app = "postgres" }
      }
      spec {
        container {
          name  = "postgres"
          image = var.postgres_image
          port {
            container_port = 5432
          }
          env {
            name  = "POSTGRES_DB"
            value = "master_registry"
          }
          env {
            name  = "POSTGRES_USER"
            value = "admin"
          }
          env {
            name  = "POSTGRES_PASSWORD"
            value = var.postgres_password
          }
          volume_mount {
            name       = "postgres-storage"
            mount_path = "/var/lib/postgresql/data"
          }
        }
        volume {
          name = "postgres-storage"
          persistent_volume_claim {
            claim_name = kubernetes_persistent_volume_claim.postgres.metadata[0].name
          }
        }
      }
    }
  }
}

resource "kubernetes_service" "postgres" {
  metadata {
    name      = "postgres-service"
    namespace = kubernetes_namespace.pdf_service.metadata[0].name
  }
  spec {
    selector = { app = "postgres" }
    port {
      port        = 5432
      target_port = 5432
    }
  }
}

# ─── MongoDB ──────────────────────────────────────────────────────────────────

resource "kubernetes_persistent_volume_claim" "mongodb" {
  metadata {
    name      = "mongodb-pvc"
    namespace = kubernetes_namespace.pdf_service.metadata[0].name
  }
  spec {
    access_modes = ["ReadWriteOnce"]
    resources {
      requests = { storage = var.mongodb_storage }
    }
  }
}

resource "kubernetes_stateful_set" "mongodb" {
  metadata {
    name      = "mongodb"
    namespace = kubernetes_namespace.pdf_service.metadata[0].name
  }
  spec {
    service_name = "mongodb-service"
    replicas     = 1
    selector {
      match_labels = { app = "mongodb" }
    }
    template {
      metadata {
        labels = { app = "mongodb" }
      }
      spec {
        container {
          name  = "mongodb"
          image = var.mongodb_image
          port {
            container_port = 27017
          }
          volume_mount {
            name       = "mongodb-storage"
            mount_path = "/data/db"
          }
        }
        volume {
          name = "mongodb-storage"
          persistent_volume_claim {
            claim_name = kubernetes_persistent_volume_claim.mongodb.metadata[0].name
          }
        }
      }
    }
  }
}

resource "kubernetes_service" "mongodb" {
  metadata {
    name      = "mongodb-service"
    namespace = kubernetes_namespace.pdf_service.metadata[0].name
  }
  spec {
    selector = { app = "mongodb" }
    port {
      port        = 27017
      target_port = 27017
    }
  }
}

# ─── MinIO ────────────────────────────────────────────────────────────────────

resource "kubernetes_persistent_volume_claim" "minio" {
  metadata {
    name      = "minio-pvc"
    namespace = kubernetes_namespace.pdf_service.metadata[0].name
  }
  spec {
    access_modes = ["ReadWriteOnce"]
    resources {
      requests = { storage = var.minio_storage }
    }
  }
}

resource "kubernetes_stateful_set" "minio" {
  metadata {
    name      = "minio"
    namespace = kubernetes_namespace.pdf_service.metadata[0].name
  }
  spec {
    service_name = "minio-service"
    replicas     = 1
    selector {
      match_labels = { app = "minio" }
    }
    template {
      metadata {
        labels = { app = "minio" }
      }
      spec {
        container {
          name    = "minio"
          image   = var.minio_image
          args    = ["server", "/data", "--console-address", ":9001"]
          port {
            container_port = 9000
          }
          port {
            container_port = 9001
          }
          env {
            name  = "MINIO_ROOT_USER"
            value = var.minio_root_user
          }
          env {
            name  = "MINIO_ROOT_PASSWORD"
            value = var.minio_root_password
          }
          volume_mount {
            name       = "minio-storage"
            mount_path = "/data"
          }
        }
        volume {
          name = "minio-storage"
          persistent_volume_claim {
            claim_name = kubernetes_persistent_volume_claim.minio.metadata[0].name
          }
        }
      }
    }
  }
}

resource "kubernetes_service" "minio" {
  metadata {
    name      = "minio-service"
    namespace = kubernetes_namespace.pdf_service.metadata[0].name
  }
  spec {
    selector = { app = "minio" }
    port {
      name        = "api"
      port        = 9000
      target_port = 9000
    }
    port {
      name        = "console"
      port        = 9001
      target_port = 9001
    }
  }
}

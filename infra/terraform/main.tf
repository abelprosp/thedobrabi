# Terraform — TheDobra (esqueleto aplicável por ambiente)
#
# Estado: backend S3 (descomente e preencha).
# Módulos: VPC, EKS, RDS Postgres, ElastiCache Redis, S3/MinIO, MSK/Redpanda.
# Os manifests em ../kubernetes aplicam-se sobre o cluster após `terraform apply`.

terraform {
  required_version = ">= 1.6.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
  # backend "s3" {
  #   bucket = "thedobra-tf-state"
  #   key    = "prod/terraform.tfstate"
  #   region = "eu-west-1"
  # }
}

variable "environment" {
  type    = string
  default = "prod"
}

variable "region" {
  type    = string
  default = "eu-west-1"
}

provider "aws" {
  region = var.region
}

# Substitua por módulos reais (vpc / eks / rds) no repositório de infra da org.
# resource "aws_eks_cluster" "thedobra" { ... }
# resource "aws_db_instance" "postgres" { engine = "postgres" ... }
# resource "aws_elasticache_cluster" "redis" { ... }
# ClickHouse e Redpanda: operadores no cluster (Altinity / Strimzi) ou serviços geridos.

output "kubeconfig_hint" {
  value = "aws eks update-kubeconfig --name thedobra-${var.environment} --region ${var.region}"
}

output "next_step" {
  value = "kubectl apply -k ../../infra/kubernetes"
}

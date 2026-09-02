# Terraform skeleton — environments live in their own state.

TheDobra assumes three layers:

1. **This folder** — VPC, EKS (or equivalent), RDS Postgres, Redis, object storage, Kafka/Redpanda.
2. **`../kubernetes`** — API, web, worker, HPA, PDB, Ingress applied with `kubectl apply -k`.
3. **Secrets** — JWT, encryption key, Stripe, SMTP, OAuth; never commit. Use `secret.example.yaml` as template.

```bash
cd infra/terraform
terraform init
terraform plan -var="environment=staging"
# after apply:
kubectl apply -k ../kubernetes
```

Managed ClickHouse (Altinity Cloud / ClickHouse Cloud) and MinIO/S3 are expected as endpoints in the Kubernetes ConfigMap, not as in-cluster toys for production.

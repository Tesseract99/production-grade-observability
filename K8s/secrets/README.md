# Secrets Management with Sealed Secrets

ArgoCD cannot use plain secret files from git. Use Sealed Secrets to encrypt secrets that can be safely committed.

## Setup

1. Install Sealed Secrets controller in your cluster:
```bash
kubectl apply -f https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.24.5/controller.yaml
```

2. Install kubeseal CLI:
```bash
# macOS
brew install kubeseal
```

## Creating Sealed Secrets

1. Create a regular secret locally (don't commit this):
```bash
kubectl create secret generic mysql-secret \
  --namespace=mydb \
  --from-env-file=mysql.env \
  --dry-run=client -o yaml > mysql-secret.yaml
```

2. Seal it:
```bash
kubeseal --format yaml < mysql-secret.yaml > mysql-sealed-secret.yaml
```

3. Commit the sealed secret (safe to commit):
```bash
git add mysql-sealed-secret.yaml
```

4. Repeat for app-secret:
```bash
kubectl create secret generic app-secret \
  --namespace=myapp-go \
  --from-env-file=app.env \
  --dry-run=client -o yaml > app-secret.yaml

kubeseal --format yaml < app-secret.yaml > app-sealed-secret.yaml
```

## Alternative: External Secrets Operator

For AWS/GCP/Azure secret stores, use External Secrets Operator instead.

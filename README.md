# Movie Service - Kubernetes Observability Stack

A Go-based REST API for movie management with distributed tracing using OpenTelemetry and Jaeger on Kubernetes.

## Tech Stack

| Component | Technology | Version |
|-----------|------------|---------|
| Application | Go | 1.24 |
| Database | MySQL | 8.0 |
| Tracing | OpenTelemetry SDK | 1.39.0 |
| Trace Collector | OpenTelemetry Collector | latest |
| Trace Backend | Jaeger (all-in-one) | latest |
| Container Runtime | Docker | - |
| Orchestration | Kubernetes | - |
| SQL Instrumentation | otelsql | 0.41.0 |

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/` | Health check |
| GET | `/movies` | List all movies |
| POST | `/movie` | Add a new movie |

## Low-Level Design (LLD)

```mermaid
flowchart TB
    subgraph ns_myapp["Namespace: myapp-go"]
        subgraph deploy_app["Deployment: app"]
            app["myapp-go<br/>Container<br/>Port: 8003"]
        end
        svc_app["Service: app-service<br/>ClusterIP<br/>Port: 8003"]
        cm_app["ConfigMap: app-config"]
        secret_app["Secret: app-secret"]
    end

    subgraph ns_mydb["Namespace: mydb"]
        subgraph sts_mysql["StatefulSet: mysql"]
            mysql[("MySQL 8.0<br/>Container<br/>Port: 3306")]
            pvc_mysql[("PVC: mysql-data<br/>1Gi")]
        end
        svc_mysql["Service: mysql<br/>Headless (ClusterIP: None)<br/>Port: 3306"]
        cm_mysql["ConfigMap: mysql-init-script"]
        secret_mysql["Secret: mysql-secret"]
    end

    subgraph ns_observability["Namespace: observability"]
        subgraph deploy_otel["Deployment: otel-collector<br/>Replicas: 2"]
            otel["OTel Collector<br/>Container<br/>Port: 4317 (gRPC)"]
        end
        svc_otel["Service: otel-collector-svc<br/>ClusterIP<br/>Port: 4317"]
        cm_otel["ConfigMap: otel-config"]

        subgraph deploy_jaeger["Deployment: jaeger"]
            jaeger["Jaeger All-in-One<br/>Container<br/>UI: 16686<br/>OTLP: 4317"]
        end
        svc_jaeger["Service: jaeger<br/>ClusterIP<br/>Ports: 16686, 4317"]

        subgraph deploy_es["Deployment: elasticsearch"]
            elasticsearch[("Elasticsearch 8.11<br/>Container<br/>Ports: 9200, 9300")]
            pvc_es[("PVC: elasticsearch-data<br/>1Gi")]
        end
        svc_es["Service: elasticsearch<br/>ClusterIP<br/>Ports: 9200, 9300"]
    end

    %% Config/Secret relationships
    cm_app -.-> app
    secret_app -.-> app
    cm_mysql -.-> mysql
    secret_mysql -.-> mysql
    cm_otel -.-> otel
    pvc_mysql --- mysql
    pvc_es --- elasticsearch

    %% Service relationships
    svc_app --> app
    svc_mysql --> mysql
    svc_otel --> otel
    svc_jaeger --> jaeger
    svc_es --> elasticsearch

    %% Data flow
    app -->|"OTLP/gRPC<br/>:4317"| svc_otel
    svc_otel --> otel
    otel -->|"OTLP/gRPC<br/>:4317"| svc_jaeger
    svc_jaeger --> jaeger
    jaeger -->|"HTTP :9200"| svc_es
    svc_es --> elasticsearch
    app -->|"MySQL Protocol<br/>:3306"| svc_mysql
    svc_mysql --> mysql

    %% External access
    client["Client"]
    client -->|"HTTP :8003"| svc_app

    %% Styling
    classDef namespace fill:#e1f5fe,stroke:#01579b,color:#01579b,font-size:14px
    classDef statefulset fill:#fff3e0,stroke:#e65100,color:#bf360c,font-size:14px
    classDef deployment fill:#e8f5e9,stroke:#2e7d32,color:#1b5e20,font-size:14px
    classDef service fill:#f3e5f5,stroke:#7b1fa2,color:#4a148c,font-size:14px
    classDef config fill:#fce4ec,stroke:#c2185b,color:#880e4f,font-size:14px
    classDef storage fill:#e3f2fd,stroke:#1565c0,color:#0d47a1,font-size:14px
    classDef container fill:#e8f5e9,stroke:#2e7d32,color:#1b5e20,font-size:14px
    classDef clientStyle fill:#fff3e0,stroke:#e65100,color:#bf360c,font-size:14px
    classDef default color:#212121,font-size:14px

    class ns_myapp,ns_mydb,ns_observability namespace
    class sts_mysql,deploy_es statefulset
    class deploy_app,deploy_otel,deploy_jaeger deployment
    class svc_app,svc_mysql,svc_otel,svc_jaeger,svc_es service
    class cm_app,cm_mysql,cm_otel,secret_app,secret_mysql config
    class pvc_mysql,pvc_es,mysql,elasticsearch storage
    class app,otel,jaeger container
    class client clientStyle

    %% Link styling for thicker arrows
    linkStyle default stroke-width:4px
```

## Architecture Overview

### Namespaces

| Namespace | Purpose |
|-----------|---------|
| `myapp-go` | Application workload |
| `mydb` | Database layer |
| `observability` | Tracing infrastructure |

### Workloads

| Workload | Type | Namespace | Replicas | Ports |
|----------|------|-----------|----------|-------|
| app | Deployment | myapp-go | 1 | 8003 |
| mysql | StatefulSet | mydb | 1 | 3306 |
| otel-collector | Deployment | observability | 2 | 4317 |
| jaeger | Deployment | observability | 1 | 16686, 4317 |
| elasticsearch | Deployment | observability | 1 | 9200, 9300 |

### Services

| Service | Type | Namespace | Port(s) | Target |
|---------|------|-----------|---------|--------|
| app-service | ClusterIP | myapp-go | 8003 | app pods |
| mysql | Headless | mydb | 3306 | mysql pods |
| otel-collector-svc | ClusterIP | observability | 4317 | otel-collector pods |
| jaeger | ClusterIP | observability | 16686, 4317 | jaeger pods |
| elasticsearch | ClusterIP | observability | 9200, 9300 | elasticsearch pods |

### Data Flow

1. Client sends HTTP requests to `app-service:8003`
2. Go application processes requests and queries MySQL via `mysql.mydb.svc.cluster.local:3306`
3. Application sends traces via OTLP/gRPC to `otel-collector-svc.observability.svc.cluster.local:4317`
4. OTel Collector forwards traces to `jaeger.observability.svc.cluster.local:4317`
5. Jaeger persists traces to Elasticsearch via `elasticsearch.observability.svc.cluster.local:9200`
6. Jaeger UI available at port `16686` for trace visualization

## Quick Start

```bash
# Apply manifests in order
kubectl apply -f K8s/mysql.yaml
kubectl apply -f K8s/elasticsearch/elasticsearch.yaml
kubectl apply -f K8s/jaegar/jaegar.yaml
kubectl apply -f K8s/otel-collector/otel-collector.yaml
kubectl apply -f K8s/app.yaml

# Port-forward Jaeger UI
kubectl port-forward -n observability svc/jaeger 16686:16686

# Port-forward application
kubectl port-forward -n myapp-go svc/app-service 8003:8003
```

## Project Structure

```
app-db/
├── K8s/
│   ├── app.yaml              # Application deployment
│   ├── mysql.yaml            # MySQL StatefulSet
│   ├── elasticsearch/
│   │   └── elasticsearch.yaml # Elasticsearch for Jaeger storage
│   ├── jaegar/
│   │   └── jaegar.yaml       # Jaeger deployment
│   └── otel-collector/
│       └── otel-collector.yaml
└── src/
    ├── main.go               # HTTP handlers & DB operations
    ├── telemetry.go          # OTel tracer initialization
    ├── go.mod
    └── Dockerfile
```

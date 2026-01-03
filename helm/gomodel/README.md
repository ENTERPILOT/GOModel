# GOModel Helm Chart

High-performance AI gateway for multiple LLM providers (OpenAI, Anthropic, Gemini, Groq, xAI).

## Prerequisites

- Kubernetes 1.29+ (for Gateway API v1 support)
- Helm 3.x
- (Optional) Prometheus Operator for ServiceMonitor support

## Installation

### Add the Helm repository (if published)

```bash
helm repo add gomodel https://your-org.github.io/gomodel
helm repo update
```

### Install from local chart

```bash
# Basic install with OpenAI
helm install gomodel ./helm/gomodel \
  -n gomodel --create-namespace \
  --set providers.openai.enabled=true \
  --set providers.openai.apiKey="sk-..."

# Multi-provider setup with Redis cache
helm install gomodel ./helm/gomodel \
  -n gomodel --create-namespace \
  --set providers.openai.enabled=true \
  --set providers.openai.apiKey="sk-..." \
  --set providers.anthropic.enabled=true \
  --set providers.anthropic.apiKey="sk-ant-..." \
  --set redis.enabled=true

# Using existing secrets (GitOps-friendly)
helm install gomodel ./helm/gomodel \
  -n gomodel --create-namespace \
  --set providers.existingSecret="llm-api-keys" \
  --set providers.openai.enabled=true \
  --set providers.anthropic.enabled=true
```

## Configuration

### Key Values

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas | `2` |
| `image.repository` | Image repository | `your-registry/gomodel` |
| `image.tag` | Image tag | `""` (uses appVersion) |
| `server.port` | Server port | `8080` |
| `server.bodySizeLimit` | Max request body size | `"10M"` |
| `auth.masterKey` | Master key for auth | `""` |
| `auth.existingSecret` | Existing secret for auth | `""` |
| `providers.existingSecret` | Existing secret for API keys | `""` |
| `providers.openai.enabled` | Enable OpenAI | `false` |
| `providers.anthropic.enabled` | Enable Anthropic | `false` |
| `providers.gemini.enabled` | Enable Gemini | `false` |
| `providers.groq.enabled` | Enable Groq | `false` |
| `providers.xai.enabled` | Enable xAI | `false` |
| `cache.type` | Cache type (local/redis) | `"local"` |
| `redis.enabled` | Deploy Redis subchart | `false` |
| `metrics.enabled` | Enable Prometheus metrics | `true` |
| `metrics.serviceMonitor.enabled` | Create ServiceMonitor | `false` |
| `ingress.enabled` | Enable Ingress | `false` |
| `gateway.enabled` | Enable Gateway API HTTPRoute | `false` |
| `autoscaling.enabled` | Enable HPA | `false` |

### Using Existing Secrets

Create a secret with your API keys:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: llm-api-keys
type: Opaque
stringData:
  OPENAI_API_KEY: "sk-..."
  ANTHROPIC_API_KEY: "sk-ant-..."
  GEMINI_API_KEY: "..."
```

Then reference it:

```bash
helm install gomodel ./helm/gomodel \
  --set providers.existingSecret="llm-api-keys" \
  --set providers.openai.enabled=true
```

### Ingress Example

```yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: gomodel.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: gomodel-tls
      hosts:
        - gomodel.example.com
```

### Gateway API Example

```yaml
gateway:
  enabled: true
  parentRef:
    name: my-gateway
    namespace: gateway-system
  hostnames:
    - gomodel.example.com
```

## Upgrading

```bash
helm upgrade gomodel ./helm/gomodel -n gomodel -f values.yaml
```

## Uninstalling

```bash
helm uninstall gomodel -n gomodel
```

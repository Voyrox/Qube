## Qube Root Page

### Local Build

```bash
go build -o qube .
```

### Local Run

```bash
./qube
```

The website listens on `http://localhost:32002`.

### Docker

```bash
docker build -t harbor.ewenmacculloch.com/core/qube:1.0.0 .
docker push harbor.ewenmacculloch.com/core/qube:1.0.0
```

### Kubernetes

```bash
kubectl apply -f qube.yaml
kubectl rollout restart deployment/qube -n apps
kubectl rollout status deployment/qube -n apps --timeout=120s
```

### One-Step Deploy

```bat
apply.bat
```

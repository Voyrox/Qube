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
docker build -t registry.ewenmacculloch.com/qube:latest .
docker push registry.ewenmacculloch.com/qube:latest
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

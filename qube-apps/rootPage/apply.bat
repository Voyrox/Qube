@echo off
setlocal

git pull
if errorlevel 1 goto :error

docker build -t registry.ewenmacculloch.com/portfolio:latest .
if errorlevel 1 goto :error

docker push registry.ewenmacculloch.com/portfolio:latest
if errorlevel 1 goto :error

kubectl apply -f go-profile.yaml
if errorlevel 1 goto :error

kubectl rollout restart deployment/go-profile -n apps
if errorlevel 1 goto :error

kubectl rollout status deployment/go-profile -n apps
if errorlevel 1 goto :error

echo Deployment updated successfully.
exit /b 0

:error
echo Deployment update failed.
exit /b 1

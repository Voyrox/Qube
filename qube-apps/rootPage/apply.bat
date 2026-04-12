@echo off
setlocal

git pull
if errorlevel 1 goto :error

docker build -t registry.ewenmacculloch.com/qube:latest .
if errorlevel 1 goto :error

docker push registry.ewenmacculloch.com/qube:latest
if errorlevel 1 goto :error

kubectl apply -f qube.yaml
if errorlevel 1 goto :error

kubectl rollout restart deployment/qube -n apps
if errorlevel 1 goto :error

kubectl rollout status deployment/qube -n apps --timeout=120s
if errorlevel 1 goto :error

echo Deployment updated successfully.
exit /b 0

:error
echo Deployment update failed.
exit /b 1

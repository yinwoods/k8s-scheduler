kubectl delete -f deployments/nginx.yaml
go build -o scheduler anchor/*.go
kubectl create -f deployments/nginx.yaml
./scheduler

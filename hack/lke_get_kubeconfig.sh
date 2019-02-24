kubectl get secret -n $1 admin-kubeconfig -o json | jq -r '.data["admin.conf"]' | base64 -d

# kube-rbac-proxy

This directory is a hack to provide a main module for ko to build the kube-rbac-proxy image. Dependabot will check for new releases in the upstream repository and PR updated version.

main.go is directly from https://github.com/brancz/kube-rbac-proxy/blob/v0.19.1/cmd/kube-rbac-proxy/main.go. While it is minimal and unlikely to change, if/when it does change we will need to update the content here.

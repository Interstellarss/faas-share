# code-generator (branch release-1.17) commit hash: 4ae19cfe9b46bf48d232c065a9078d1dff3de06c
CODEGEN_PKG=$(echo `go env GOPATH`"/pkg/mod/k8s.io/code-generator@v0.23.6")

${CODEGEN_PKG}/generate-groups.sh all \
github.com/Interstellarss/faas-share/pkg/client \
github.com/Interstellarss/faas-share/pkg/apis \
kubeshare:v1

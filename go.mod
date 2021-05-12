module github.com/verrazzano/verrazzano-monitoring-operator

go 1.13

require (
	github.com/gorilla/mux v1.7.3 // indirect
	github.com/kylelemons/godebug v1.1.0
	github.com/oracle/oci-go-sdk v24.3.0+incompatible // indirect
	github.com/stretchr/testify v1.5.1
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20200220183623-bac4c82f6975
	gopkg.in/resty.v1 v1.12.0
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.18.2
	k8s.io/apiextensions-apiserver v0.18.2
	k8s.io/apimachinery v0.18.2
	k8s.io/client-go v0.18.2
	k8s.io/code-generator v0.18.2
	sigs.k8s.io/controller-runtime v0.6.0
)

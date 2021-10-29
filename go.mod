module github.com/verrazzano/verrazzano-monitoring-operator

go 1.13

require (
	github.com/go-logr/logr v0.4.0 // indirect
	github.com/go-logr/zapr v0.4.0 // indirect
	github.com/go-resty/resty/v2 v2.6.0
	github.com/stretchr/testify v1.6.1
	github.com/verrazzano/pkg v0.0.2
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.21.1
	k8s.io/apiextensions-apiserver v0.19.2
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
	k8s.io/code-generator v0.19.2
	sigs.k8s.io/controller-runtime v0.6.2
)

replace (
	k8s.io/api => k8s.io/api v0.19.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.2
	k8s.io/client-go => k8s.io/client-go v0.19.2
)

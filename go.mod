module github.com/verrazzano/verrazzano-monitoring-operator

go 1.16

require (
	github.com/go-resty/resty/v2 v2.6.0
	github.com/stretchr/testify v1.7.0
	github.com/verrazzano/pkg v0.0.2
	go.uber.org/zap v1.17.0
	golang.org/x/crypto v0.0.0-20210220033148-5ea612d1eb83
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.21.2
	k8s.io/apiextensions-apiserver v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v0.21.2
	k8s.io/code-generator v0.21.2
	sigs.k8s.io/controller-runtime v0.9.2
)

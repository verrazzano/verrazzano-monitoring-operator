module github.com/verrazzano/verrazzano-monitoring-operator

go 1.13

require (
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/gorilla/mux v1.7.3
	github.com/imdario/mergo v0.3.8 // indirect
	github.com/kylelemons/godebug v1.1.0
	github.com/prometheus/client_golang v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.4.0
	gopkg.in/resty.v1 v1.12.0
	gopkg.in/yaml.v2 v2.2.5
	k8s.io/api v0.15.7
	k8s.io/apiextensions-apiserver v0.15.7
	k8s.io/apimachinery v0.15.7
	k8s.io/client-go v0.15.7
	k8s.io/code-generator v0.15.7

)

replace (
	// pinning kubernetes-1.15.5 - latest in 1.15.* series, which was first to use go.mod
	k8s.io/api => k8s.io/api v0.15.7
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.15.7
	k8s.io/apimachinery => k8s.io/apimachinery v0.15.7
	k8s.io/client-go => k8s.io/client-go v0.15.7
	k8s.io/code-generator => k8s.io/code-generator v0.15.7
)

module github.com/verrazzano/verrazzano-monitoring-operator

go 1.13

require (
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b // indirect
	github.com/gorilla/mux v1.7.3
	github.com/kylelemons/godebug v1.1.0
	github.com/prometheus/client_golang v1.2.1
	github.com/rs/zerolog v1.20.0
	github.com/stretchr/testify v1.4.0
	gopkg.in/resty.v1 v1.12.0
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.18.2
	k8s.io/apiextensions-apiserver v0.18.2
	k8s.io/apimachinery v0.18.2
	k8s.io/client-go v0.18.2
	k8s.io/code-generator v0.18.2
)

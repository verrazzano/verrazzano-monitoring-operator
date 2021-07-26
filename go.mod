module github.com/verrazzano/verrazzano-monitoring-operator

go 1.13

require (
	github.com/go-resty/resty/v2 v2.6.0
	github.com/gordonklaus/ineffassign v0.0.0-20210522101830-0589229737b2 // indirect
	github.com/gorilla/mux v1.7.3 // indirect
	github.com/oracle/oci-go-sdk v24.3.0+incompatible // indirect
	github.com/stretchr/testify v1.6.1
	github.com/verrazzano/pkg v0.0.2
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/lint v0.0.0-20210508222113-6edffad5e616 // indirect
	golang.org/x/sys v0.0.0-20210616094352-59db8d763f22 // indirect
	golang.org/x/tools v0.1.3 // indirect
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.21.1
	k8s.io/apiextensions-apiserver v0.18.2
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
	k8s.io/code-generator v0.18.2
	sigs.k8s.io/controller-runtime v0.6.0
	sigs.k8s.io/yaml v1.2.0
)

replace (
	k8s.io/api => k8s.io/api v0.18.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.2
	k8s.io/client-go => k8s.io/client-go v0.18.2
)

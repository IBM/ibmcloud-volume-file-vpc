module github.com/IBM/ibmcloud-volume-file-vpc

go 1.16

require (
	github.com/IBM-Cloud/ibm-cloud-cli-sdk v0.6.7
	github.com/IBM/ibm-csi-common v1.0.0-beta9
	github.com/IBM/ibmcloud-volume-interface v1.0.0-beta8
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/fatih/structs v1.1.0
	github.com/pierrre/gotestcover v0.0.0-20160517101806-924dca7d15f0 // indirect
	github.com/satori/go.uuid v1.2.0
	github.com/stretchr/testify v1.6.1
	go.uber.org/zap v1.15.0
	golang.org/x/net v0.0.0-20201021035429-f5854403a974
	google.golang.org/grpc v1.38.0 // indirect
)

replace k8s.io/apiserver => k8s.io/apiserver v0.0.0-20190313205120-8b27c41bdbb1

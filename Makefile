all: binary

binary:
	CGO_ENABLE=0 go build -o bin/kubewatch ./main.go

plugin:
	CGO_ENABLE=0 go build -o plugins/appsv1.so -buildmode=plugin ./plugins/appsv1
	CGO_ENABLE=0 go build -o plugins/corev1.so -buildmode=plugin ./plugins/appsv1
	CGO_ENABLE=0 go build -o plugins/autoscalingv2.so -buildmode=plugin ./plugins/autoscalingv2

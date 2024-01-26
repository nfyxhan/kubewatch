package plugins

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Plugin interface {
	AddToScheme(s *runtime.Scheme) error
	ListObjects() []Object
}

type Object struct {
	Name      string
	ShortName string
	client.Object
	client.ObjectList
}

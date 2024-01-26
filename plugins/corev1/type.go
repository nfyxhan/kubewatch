package main

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/nfyxhan/kubewatch/pkg/plugins"
)

type Plugin struct {
}

func New() plugins.Plugin {
	return &Plugin{}
}

func (p *Plugin) AddToScheme(s *runtime.Scheme) error {
	return v1.AddToScheme(s)
}

func (p *Plugin) ListObjects() []plugins.Object {
	return []plugins.Object{
		{
			Name:       "configmap",
			ShortName:  "cm",
			Object:     &v1.ConfigMap{},
			ObjectList: &v1.ConfigMapList{},
		}, {
			Name:       "namespace",
			ShortName:  "ns",
			Object:     &v1.Namespace{},
			ObjectList: &v1.NamespaceList{},
		}, {
			Name:       "pod",
			Object:     &v1.Pod{},
			ObjectList: &v1.PodList{},
		},
	}
}

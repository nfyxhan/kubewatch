package main

import (
	v2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/nfyxhan/kubewatch/pkg/plugins"
)

type Plugin struct {
}

func New() plugins.Plugin {
	return &Plugin{}
}

func (p *Plugin) AddToScheme(s *runtime.Scheme) error {
	return v2.AddToScheme(s)
}

func (p *Plugin) ListObjects() []plugins.Object {
	return []plugins.Object{
		{
			Name:       "horizontalpodautoscaler",
			ShortName:  "hpa",
			Object:     &v2.HorizontalPodAutoscaler{},
			ObjectList: &v2.HorizontalPodAutoscalerList{},
		},
	}
}

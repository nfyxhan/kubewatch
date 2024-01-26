package main

import (
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"

	plugins "github.com/nfyxhan/kubewatch/pkg/plugins"
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
			Name:       "deployment",
			ShortName:  "dp",
			Object:     &v1.Deployment{},
			ObjectList: &v1.DeploymentList{},
		},
	}
}

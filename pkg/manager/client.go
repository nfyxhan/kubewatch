package manager

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SchemeObject struct {
	Name       string
	ShortNames []string
	client.Object
	client.ObjectList
}

type SchemeClient interface {
	AddToScheme(ctx context.Context, scm *runtime.Scheme) error
	GetRestConfig() *rest.Config
	ListApiGroups(ctx context.Context) ([]metav1.APIGroup, error)
	ListApiResources(ctx context.Context, group string) (*metav1.APIResourceList, error)
	GetObjectMap(ctx context.Context, group string) (map[string]SchemeObject, error)
	ListNamespace(ctx context.Context) ([]corev1.Namespace, error)
}

func NewSchemeClient(cfg *rest.Config) SchemeClient {
	return &schemeClient{
		config: cfg,
	}
}

type ObjectClient interface {
	Start(ctx context.Context) error
	GetObjectsKind(ctx context.Context, groupVersion string, objects string) ([]SchemeObject, error)
	ListObjects(ctx context.Context, groupVersion string, objects string, namespace string) ([]string, error)
	ListObjectPathPrefix(ctx context.Context, groupVersion, kind, pathPrefix string) ([]string, error)
}

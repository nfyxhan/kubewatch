package manager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/rawhttp"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

type schemeClient struct {
	config *rest.Config
}

func (c *schemeClient) GetRestConfig() *rest.Config {
	return c.config
}

func (c *schemeClient) ListNamespace(ctx context.Context) ([]corev1.Namespace, error) {
	path := "/api/v1/namespaces"
	obj := &corev1.NamespaceList{}
	if err := c.rawGet(ctx, path, obj); err != nil {
		return nil, err
	}
	return obj.Items, nil
}

func (c *schemeClient) GetObjectMap(ctx context.Context, group string) (map[string]SchemeObject, error) {
	objects, err := c.listObjects(ctx, group)
	if err != nil {
		return nil, err
	}
	var objectMap = make(map[string]SchemeObject)
	for _, o := range objects {
		name := o.Name
		objectMap[name] = o
		for _, shortName := range o.ShortNames {
			objectMap[shortName] = o
		}
	}
	return objectMap, nil
}

func (c *schemeClient) ListApiGroups(ctx context.Context) ([]metav1.APIGroup, error) {
	path := "/apis"
	obj := &metav1.APIGroupList{}
	if err := c.rawGet(ctx, path, obj); err != nil {
		return nil, err
	}
	groups := obj.Groups
	corev1GroupVersion := metav1.GroupVersionForDiscovery{
		GroupVersion: "v1",
		Version:      "v1",
	}
	groups = append(groups, metav1.APIGroup{
		Name: "v1",
		Versions: []metav1.GroupVersionForDiscovery{
			corev1GroupVersion,
		},
		PreferredVersion: corev1GroupVersion,
	})
	return groups, nil
}

func (c *schemeClient) ListApiResources(ctx context.Context, group string) (*metav1.APIResourceList, error) {
	var path string
	if group != "v1" {
		path = fmt.Sprintf("/apis/%s", group)
	} else {
		path = "/api/v1"
	}
	obj := &metav1.APIResourceList{}
	if err := c.rawGet(ctx, path, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func (c *schemeClient) AddToScheme(ctx context.Context, scm *runtime.Scheme) error {
	groups, err := c.ListApiGroups(ctx)
	if err != nil {
		return err
	}
	for _, group := range groups {
		GroupVersion := schema.GroupVersion{
			Group:   group.Name,
			Version: group.PreferredVersion.Version,
		}
		SchemeBuilder := &scheme.Builder{
			GroupVersion: GroupVersion,
		}
		if err := SchemeBuilder.AddToScheme(scm); err != nil {
			return err
		}
	}
	return nil
}

func (c *schemeClient) listObjects(ctx context.Context, group string) ([]SchemeObject, error) {
	apiResourceList, err := c.ListApiResources(ctx, group)
	if err != nil {
		return nil, err
	}
	version := apiResourceList.GroupVersion
	res := make([]SchemeObject, 0)
	for _, r := range apiResourceList.APIResources {
		name := r.SingularName
		if name == "" {
			name = r.Kind
		}
		obj := map[string]interface{}{
			"kind":       r.Kind,
			"apiVersion": version,
		}
		o := SchemeObject{
			Name:       name,
			ShortNames: r.ShortNames,
			Object: &unstructured.Unstructured{
				Object: obj,
			},
			ObjectList: &unstructured.UnstructuredList{
				Object: obj,
				Items:  make([]unstructured.Unstructured, 0),
			},
		}
		res = append(res, o)
	}
	return res, nil
}

func (c *schemeClient) rawGet(ctx context.Context, path string, object interface{}) error {
	config := c.config
	config.APIPath = "api"
	config.GroupVersion = &corev1.SchemeGroupVersion
	config.NegotiatedSerializer = clientgoscheme.Codecs
	cli, err := rest.RESTClientFor(config)
	if err != nil {
		return err
	}
	bf := bytes.NewBuffer(make([]byte, 0))
	stream := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    bf,
		ErrOut: os.Stderr,
	}
	if err := rawhttp.RawGet(cli, stream, path); err != nil {
		return err
	}
	if err := json.Unmarshal(bf.Bytes(), object); err != nil {
		return err
	}
	return nil
}

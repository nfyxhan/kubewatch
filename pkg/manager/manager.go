package manager

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/jedib0t/go-pretty/table"
	"github.com/r3labs/diff/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/strings/slices"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/nfyxhan/kubewatch/pkg/utils"
)

const (
	Split      = ","
	SplitPrint = "/"
)

var (
	AnnotationsPaths = []string{
		"metadata" + Split + "annotations",
	}
)

var ignorePath = map[string]struct{}{
	"kind":                                 {},
	"apiversion":                           {},
	"metadata" + Split + "resourceVersion": {},
	"metadata" + Split + "generation":      {},
	"metadata" + Split + "managedFields":   {},
}

func init() {
	for k, v := range ignorePath {
		ignorePath["Object"+Split+k] = v
	}
}

type Config struct {
	Objects           string            `json:"objects,omitempty"`
	Namespace         string            `json:"namespace,omitempty"`
	GroupVersion      string            `json:"groupVersion,omitempty"`
	Names             []string          `json:"names,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	EnableAnnotations bool              `json:"enableAnnotations"`
	PathPrefix        string            `json:"pathPrefix,omitempty"`
	ToComplete        string            `json:"toComplete,omitempty"`
}

func (c Config) GetKubeConfig() (*rest.Config, error) {
	flag.CommandLine.Set("kubeconfig", utils.Kubeconfig)
	flag.Parse()
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	log.FromContext(context.Background()).Info("load config", "config", cfg.Host)
	return cfg, nil
}

type manager struct {
	mgr          ctrl.Manager
	schemeClient SchemeClient
	client.Client
	objects map[string]SchemeObject
}

func NewManager(ctx context.Context, config Config, cli SchemeClient) (ObjectClient, error) {
	if cli == nil {
		cfg, err := config.GetKubeConfig()
		if err != nil {
			return nil, err
		}
		cli = NewSchemeClient(cfg)
	}
	cfg := cli.GetRestConfig()
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	sc := NewSchemeClient(cfg)
	objectMap, err := sc.GetObjectMap(ctx, config.GroupVersion)
	if err != nil {
		return nil, err
	}
	objectsStr := config.Objects
	objects := make(map[string]SchemeObject, 0)
	for _, obj := range strings.Split(objectsStr, ",") {
		if o, ok := objectMap[obj]; ok {
			objects[obj] = o
		}
	}
	if len(objects) == 0 {
		fmt.Println(objectMap)
		return nil, fmt.Errorf("no validate objects in %s", objectsStr)
	}
	if err := cli.AddToScheme(ctx, scheme); err != nil {
		return nil, err
	}
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
	})
	if err != nil {
		return nil, err
	}
	r := &manager{
		mgr:          mgr,
		schemeClient: sc,
		Client:       mgr.GetClient(),
		objects:      objects,
	}
	builder := ctrl.NewControllerManagedBy(mgr).For(&corev1.Namespace{})
	writer := os.Stdout
	for _, obj := range objects {
		builder = builder.Watches(&source.Kind{Type: obj.Object}, &handler.Funcs{
			CreateFunc: func(e event.CreateEvent, w workqueue.RateLimitingInterface) {
				obj := e.Object
				if !r.filterObject(obj, config, writer, "create") {
					return
				}
				r.log(obj).Info("object created")
			},
			UpdateFunc: func(e event.UpdateEvent, w workqueue.RateLimitingInterface) {
				objNew := e.ObjectNew
				if !r.filterObject(objNew, config, writer, "update") {
					return
				}
				objOld := e.ObjectOld
				r.log(objNew).Info("object updated")
				r.DiffObject(objNew, objOld, config, writer)
			},
			DeleteFunc: func(e event.DeleteEvent, w workqueue.RateLimitingInterface) {
				obj := e.Object
				if !r.filterObject(obj, config, writer, "delete") {
					return
				}
				r.log(obj).Info("object deleted")
			},
		})
	}
	if err := builder.Complete(r); err != nil {
		return nil, err
	}
	return r, nil
}

func (m *manager) log(object client.Object) logr.Logger {
	return log.FromContext(context.Background()).
		WithValues("name", object.GetName()).
		WithValues("namespace", object.GetNamespace()).
		WithValues("type", object.GetObjectKind().GroupVersionKind())
}

func (m *manager) filterObject(obj client.Object, config Config, w io.Writer, action string) bool {
	name := obj.GetName()
	namespace := obj.GetNamespace()
	if ns := config.Namespace; ns != "" && !strings.Contains(namespace, ns) {
		return false
	}
	names := config.Names
	if len(names) > 0 {
		var check bool
		for _, n := range names {
			if len(names) > 0 && strings.Contains(name, n) {
				check = true
				break
			}
		}
		if !check {
			return false
		}
	}
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	fmt.Fprintf(w, "%s %s %s/%s: \n", action, kind, namespace, name)
	return true
}

func (m *manager) DiffObject(objNew, objOld client.Object, config Config, w io.Writer) {
	changeLogs, _ := diff.Diff(objOld, objNew)
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.AppendHeader(table.Row{"time", "op", "path", "from", "to"})
	var rows []table.Row
	for _, changeLog := range changeLogs {
		t := changeLog.Type
		path := strings.Join(changeLog.Path, Split)
		if _, ok := ignorePath[path]; ok {
			continue
		}
		p := strings.ToLower(path)
		var ignore bool
		for k := range ignorePath {
			if strings.HasPrefix(p, strings.ToLower(k)) {
				ignore = true
				break
			}
		}
		if !config.EnableAnnotations {
			for _, k := range AnnotationsPaths {
				if strings.HasPrefix(p, strings.ToLower(k)) {
					ignore = true
					break
				}
			}
		}
		if ignore {
			continue
		}
		if pathPrefix := config.PathPrefix; pathPrefix != "" {
			if !strings.HasPrefix(p, strings.ToLower(pathPrefix)) {
				continue
			}
			path = path[len(pathPrefix):]
		}
		path = strings.ReplaceAll(path, Split, SplitPrint)
		from := changeLog.From
		if from == nil {
			from = "<nil>"
		}
		to := changeLog.To
		if to == nil {
			to = "<nil>"
		}
		rows = append(rows, table.Row{
			time.Now().Local().Format("2006-01-02T15:04:05.999999999"), t, path, from, to,
		})
	}
	if len(rows) == 0 {
		return
	}
	t.AppendRows(rows)
	t.Render()
}

func (m *manager) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func (m *manager) Start(ctx context.Context) error {
	go m.mgr.Start(ctx)
	if ok := m.mgr.GetCache().WaitForCacheSync(ctx); ok {
		return nil
	}
	return fmt.Errorf("cache not sync")

}

func (m *manager) listObjects(ctx context.Context, objList client.ObjectList, namespace string) error {
	opts := []client.ListOption{}
	if namespace != "" {
		opts = append(opts, client.InNamespace(namespace))
	}
	if err := m.List(ctx, objList, opts...); err != nil {
		return err
	}
	return nil
}

func (m *manager) ListObject(ctx context.Context, objList client.ObjectList, namespace string) ([]string, error) {
	if err := m.listObjects(ctx, objList, namespace); err != nil {
		return nil, err
	}
	var result []string
	objectList := reflect.ValueOf(objList)
	items := objectList.Elem().FieldByName("Items")
	if items.IsValid() && (items.Kind() == reflect.Array || items.Kind() == reflect.Slice) {
		totalNum := items.Len()
		for i := 0; i < totalNum; i++ {
			item := items.Index(i)
			obj := interface{}(item.Interface())
			switch o := (obj).(type) {
			case unstructured.Unstructured:
				result = append(result, o.GetName())
				continue
			}
			nameField := item.FieldByName("Name").Interface()
			name, ok := nameField.(string)
			if !ok {
				continue
			}
			result = append(result, name)
		}
	}
	return result, nil
}

func (m *manager) GetRandObject(ctx context.Context, objList client.ObjectList, namespace string, name string) (interface{}, error) {
	if err := m.listObjects(ctx, objList, namespace); err != nil {
		return nil, err
	}
	objectList := reflect.ValueOf(objList)
	items := objectList.Elem().FieldByName("Items")
	totalNum := items.Len()
	i := rand.Intn(totalNum)
	obj := items.Index(i).Interface()
	return obj, nil
}

func (m *manager) ListObjectPathPrefix(ctx context.Context, groupVersion, kind, pathPrefix string) ([]string, error) {
	o, err := m.getSchemeObject(ctx, groupVersion, kind)
	if err != nil {
		return nil, err
	}
	obj, err := m.GetRandObject(ctx, o.ObjectList, "", "")
	if err != nil {
		return nil, err
	}
	var result = make([]string, 0)
	// result := utils.ListObjectFields(ctx, obj, Split)
	// logger := log.FromContext(ctx)
	// logger.Info("list object fields", "result", result)
	var prefix []string
	for _, p := range strings.Split(pathPrefix, Split) {
		if p == "" {
			continue
		}
		result = utils.ListFields(obj)
		if !slices.Contains(result, p) {
			break
		}
		prefix = append(prefix, p)
		obj = utils.GetFieldByName(obj, p)
		if obj == nil {
			break
		}
	}
	if obj != nil {
		result = utils.ListFields(obj)
	}
	l := len(result)
	for i := 0; i < l; i++ {
		result[i] = strings.Join(append(prefix, result[i]), Split)
	}
	return result, nil
}

// func (m *manager) GetObjectsKind(ctx context.Context, groupVersion string, objects string) ([]string, error) {
// 	sos, err := m.getObjectsKind(ctx, groupVersion, objects)
// 	if err != nil {
// 		return nil, err
// 	}
// 	result := make([]string, 0)
// 	for _, o := range sos {
// 		result = append(result, o.Name)
// 		result = append(result, o.ShortNames...)
// 	}
// 	return result, nil
// }

func (m *manager) GetObjectsKind(ctx context.Context, groupVersion string, objects string) ([]SchemeObject, error) {
	result := make([]SchemeObject, 0)
	for _, object := range strings.Split(objects, ",") {
		o, err := m.getSchemeObject(ctx, groupVersion, object)
		if err != nil {
			return nil, err
		}
		result = append(result, *o)
	}
	return result, nil
}

func (m *manager) ListObjects(ctx context.Context, groupVersion string, objects string, namespace string) ([]string, error) {
	sos, err := m.GetObjectsKind(ctx, groupVersion, objects)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0)
	for _, s := range sos {
		res := s.ObjectList
		ss, err := m.ListObject(ctx, res, namespace)
		if err != nil {
			return nil, err
		}
		result = append(result, ss...)
	}
	return result, nil
}

func (m *manager) getSchemeObject(ctx context.Context, groupVersion string, kind string) (*SchemeObject, error) {
	for _, v := range m.objects {
		if v.Name == kind || slices.Contains(v.ShortNames, kind) {
			return &v, nil
		}
	}
	return nil, fmt.Errorf("no kind %s/%s", groupVersion, kind)
}

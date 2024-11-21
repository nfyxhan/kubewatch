package manager

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"reflect"
	"regexp"
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

	"github.com/nfyxhan/kubewatch/pkg/metrics"
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
	ExcludeObjects     string            `json:"excludeObjects,omitempty"`
	Objects            string            `json:"objects,omitempty"`
	Namespace          string            `json:"namespace,omitempty"`
	GroupVersion       string            `json:"groupVersion,omitempty"`
	Names              []string          `json:"names,omitempty"`
	Labels             map[string]string `json:"labels,omitempty"`
	EnableAnnotations  bool              `json:"enableAnnotations"`
	SliceOrdering      bool              `json:"sliceOrdering"`
	ColumnWidthMax     int               `json:"columnWidthMax"`
	RowWidthMax        int               `json:"rowWidthMax"`
	IgnoreMetadata     bool              `json:"ignoreMetadata"`
	PathPrefix         string            `json:"pathPrefix,omitempty"`
	PathTemplate       string            `json:"pathTemplate"`
	ToComplete         string            `json:"toComplete,omitempty"`
	MaxRows            int               `json:"maxRows"`
	MetricsBindAddress string
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
	table   func() table.Writer
	rows    []table.Row
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
			objects[o.Name] = o
		}
	}
	if len(objects) == 0 {
		for _, o := range objectMap {
			objects[o.Name] = o
		}
	}
	excludeObjectsStr := config.ExcludeObjects
	for _, obj := range strings.Split(excludeObjectsStr, ",") {
		if o, ok := objectMap[obj]; ok {
			delete(objects, o.Name)
		}
	}
	kinds := make([]string, 0)
	for k := range objects {
		kinds = append(kinds, k)
	}
	fmt.Println("watching ", kinds)
	if err := cli.AddToScheme(ctx, scheme); err != nil {
		return nil, err
	}
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: config.MetricsBindAddress,
	})
	if err != nil {
		return nil, err
	}
	r := &manager{
		mgr:          mgr,
		schemeClient: sc,
		Client:       mgr.GetClient(),
		objects:      objects,
		table: func() table.Writer {
			return NewTable(config)
		},
	}
	builder := ctrl.NewControllerManagedBy(mgr).For(&corev1.Namespace{})
	writer := os.Stdout
	for _, obj := range objects {
		builder = builder.Watches(&source.Kind{Type: obj.Object}, &handler.Funcs{
			CreateFunc: func(e event.CreateEvent, w workqueue.RateLimitingInterface) {
				obj := e.Object
				if !r.filterObject(ctx, obj, config, "create") {
					return
				}
				r.log(obj).Info("object created")
			},
			UpdateFunc: func(e event.UpdateEvent, w workqueue.RateLimitingInterface) {
				objNew := e.ObjectNew
				if !r.filterObject(ctx, objNew, config, "update") {
					return
				}
				objOld := e.ObjectOld
				r.log(objNew).Info("object updated")
				r.DiffObject(objNew, objOld, config, writer)
			},
			DeleteFunc: func(e event.DeleteEvent, w workqueue.RateLimitingInterface) {
				obj := e.Object
				if !r.filterObject(ctx, obj, config, "delete") {
					return
				}
				r.log(obj).Info("object deleted")
				metr := metrics.GetMetricsFieldValues()
				gvk := obj.GetObjectKind().GroupVersionKind()
				m, err := metr.CurryWith(map[string]string{
					"group":     gvk.Group,
					"version":   gvk.Version,
					"kind":      gvk.Kind,
					"namespace": obj.GetNamespace(),
					"name":      obj.GetName(),
				})
				if err != nil {
					r.log(obj).Info("deleted object metrics", "err", err)
				} else {
					m.Reset()
				}
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

func (m *manager) filterObject(ctx context.Context, obj client.Object, config Config, action string) bool {
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
	log.FromContext(ctx).Info(action, "kind", kind, "namespace", namespace, "name", name)
	return true
}

func (m *manager) DiffObject(objNew, objOld client.Object, config Config, w io.Writer) {
	changeLogs, _ := diff.Diff(objOld, objNew, diff.SliceOrdering(config.SliceOrdering))
	metr := metrics.GetMetricsFieldValues()
	gvk := objNew.GetObjectKind().GroupVersionKind()
	var rows []table.Row
	for _, changeLog := range changeLogs {
		t := changeLog.Type
		path := strings.Join(changeLog.Path, Split)
		if !config.IgnoreMetadata {
			ignorePath = make(map[string]struct{})
		}
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
		if t := config.PathTemplate; t != "" {
			if ok, _ := regexp.MatchString(t, path); !ok {
				continue
			}
		}
		path = strings.TrimPrefix(path, "Object/")
		labels := []string{
			gvk.Group,
			gvk.Version,
			gvk.Kind,
			objNew.GetNamespace(),
			objNew.GetName(),
			path,
		}
		fn := func(v interface{}) {
			if v == nil {
				metr.DeleteLabelValues(labels...)
				return
			}
			var vvv float64
			t := reflect.ValueOf(v)
			if t.CanAddr() {
				v = t.Interface()
			}
			switch vv := v.(type) {
			case float32, float64:
				vvv = t.Float()
			case int, int16, int32, int64, int8:
				vvv = float64(t.Int())
			default:
				if vv == nil {

				}
				return
			}
			metr.WithLabelValues(labels...).Set(vvv)
		}
		from := changeLog.From
		if from == nil {
			from = "<nil>"
		}
		to := changeLog.To
		fn(to)
		if to == nil {
			to = "<nil>"
		}
		rows = append(rows, table.Row{
			"",
			path,
			from,
			to,
			t,
		})
	}
	if len(rows) == 0 {
		return
	}
	kind := objNew.GetObjectKind().GroupVersionKind().Kind
	now := time.Now().Local().Format("15:04:05.999")
	key := fmt.Sprintf("%s/%s", kind, objNew.GetName())
	row := table.Row{
		utils.ColorString(utils.Blue, now),
		utils.ColorString(utils.Blue, key),
		"",
		"",
		"",
	}
	rows = append([]table.Row{row}, rows...)
	maxRows := config.MaxRows
	if len(rows) > maxRows {
		maxRows = len(rows)
	}
	m.rows = append(m.rows, rows...)
	if len(m.rows) > maxRows {
		m.rows = m.rows[len(m.rows)-maxRows:]
	}
	t := m.table()
	t.AppendRows(m.rows)
	s := t.Render()
	fmt.Fprintf(w, "\033c%s", s)
}

func NewTable(config Config) table.Writer {
	t := table.NewWriter()
	columnConfigs := make([]table.ColumnConfig, 0)
	header := table.Row{"time", "key", "from", "to", "op"}
	for i := 0; i < len(header); i++ {
		columnConfigs = append(columnConfigs, table.ColumnConfig{
			WidthMax: config.ColumnWidthMax,
			Number:   i + 1,
		})
	}
	t.SetColumnConfigs(columnConfigs)
	t.AppendHeader(header)
	// t.SetAutoIndex(true)
	t.SetAllowedRowLength(config.RowWidthMax)
	return t
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

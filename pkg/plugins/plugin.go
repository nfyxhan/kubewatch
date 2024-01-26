package plugins

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"plugin"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
)

var scheme = runtime.NewScheme()
var objectMap = make(map[string]Object)

func init() {
	exePath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	binaryPath, err := filepath.EvalSymlinks(filepath.Dir(exePath))
	if err != nil {
		log.Fatal(err)
	}
	for _, path := range []string{
		"./plugins",
		fmt.Sprintf("%s/../plugins", binaryPath),
	} {
		if err := filepath.Walk(path, func(src string, f os.FileInfo, e error) error {
			if f == nil {
				return nil
			}
			if f.IsDir() {
				return nil
			}
			if !strings.HasSuffix(src, ".so") {
				return nil
			}
			p, err := plugin.Open(src)
			if err != nil {
				return err
			}
			New, err := p.Lookup("New")
			if err != nil {
				return err
			}
			pl := New.(func() Plugin)()
			return AddPlugin(pl)
		}); err != nil {
			fmt.Println(err)
		}
	}
}

func GetScheme() *runtime.Scheme {
	return scheme
}

func AddPlugin(p Plugin) error {
	objects := p.ListObjects()
	for _, o := range objects {
		name := o.Name
		objectMap[name] = o
		if shortName := o.ShortName; shortName != "" {
			objectMap[shortName] = o
		}
	}
	return p.AddToScheme(scheme)
}

func GetObjectMap() map[string]Object {
	return objectMap
}

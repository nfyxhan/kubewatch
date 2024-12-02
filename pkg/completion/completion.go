package completion

import (
	"context"
	"strings"

	"k8s.io/utils/strings/slices"

	"github.com/nfyxhan/kubewatch/pkg/manager"
)

func NamespaceCompletionFunc(ctx context.Context, config manager.Config) ([]string, error) {
	cfg, err := config.GetKubeConfig()
	if err != nil {
		return nil, err
	}
	sc := manager.NewSchemeClient(cfg)
	res, err := sc.ListNamespace(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0)
	for _, r := range res {
		result = append(result, r.Name)
	}
	return result, nil
}

func GroupVersionComplitionFunc(ctx context.Context, config manager.Config) ([]string, error) {
	cfg, err := config.GetKubeConfig()
	if err != nil {
		return nil, err
	}
	cli := manager.NewSchemeClient(cfg)
	groups, err := cli.ListApiGroups(ctx)
	if err != nil {
		return nil, err
	}
	res := make([]string, 0)
	for _, g := range groups {
		for _, v := range g.Versions {
			version := v.GroupVersion
			res = append(res, version)
		}
	}
	return res, nil
}

func PathPrefixComplitionFunc(ctx context.Context, config manager.Config) ([]string, error) {
	cfg, err := config.GetKubeConfig()
	if err != nil {
		return nil, err
	}
	cli := manager.NewSchemeClient(cfg)
	mgr, err := manager.NewManager(ctx, config, cli)
	if err != nil {
		return nil, err
	}
	groupVersion := config.GroupVersion
	kinds, err := mgr.GetObjectsKind(ctx, groupVersion, config.Objects)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0)
	for _, kind := range kinds {
		res, err := mgr.ListObjectPathPrefix(ctx, groupVersion, kind.Name, config.ToComplete)
		if err != nil {
			return nil, err
		}
		result = append(result, res...)
	}
	return result, nil
}

func KindComplitionFunc(ctx context.Context, config manager.Config) ([]string, error) {
	cfg, err := config.GetKubeConfig()
	if err != nil {
		return nil, err
	}
	cli := manager.NewSchemeClient(cfg)
	result := make([]string, 0)
	res, err := cli.GetObjectMap(ctx, config.GroupVersion)
	if err != nil {
		return nil, err
	}
	o := config.ToComplete
	ss := strings.Split(o, ",")
	l := len(ss)
	s := ss[l-1]
	for k := range res {
		// if len(ss) > 1 {
		// 	return []string{o + "prefix", "app", "as"}, nil
		// } else {
		// 	return []string{o + "prefix", "am", "amh"}, nil
		// }
		if o != "" && s != "" && !strings.HasPrefix(k, s) {
			continue
		}
		if slices.Contains(ss, k) {
			continue
		}
		result = append(result, strings.Join(append(ss[:l-1], k), ","))
	}
	return result, nil

}

func NameComplitionFunc(ctx context.Context, config manager.Config) ([]string, error) {
	cfg, err := config.GetKubeConfig()
	if err != nil {
		return nil, err
	}
	cli := manager.NewSchemeClient(cfg)
	if config.Objects == "" {
		// res, err := cli.GetObjectMap(ctx, config.GroupVersion)
		// if err != nil {
		// 	return nil, err
		// }
		result := make([]string, 0)
		// for k := range res {
		// 	result = append(result, k)
		// }
		return result, nil
	}
	mgr, err := manager.NewManager(ctx, config, cli)
	if err != nil {
		return nil, err
	}
	res, err := mgr.ListObjects(ctx, config.GroupVersion, config.Objects, config.Namespace)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0)
	for _, r := range res {
		if slices.Contains(config.Names, r) {
			continue
		}
		result = append(result, r)
	}
	return result, nil
}

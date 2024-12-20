/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"context"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/nfyxhan/kubewatch/pkg/completion"
	"github.com/nfyxhan/kubewatch/pkg/manager"
)

var mgrConfig = manager.Config{}

func init() {
	// watchCmd represents the watch command
	var watchCmd = &cobra.Command{
		// PreRun: func(cmd *cobra.Command, args []string) {
		// 	utils.SetMgrConfig(mgrConfig)
		// 	completion.SetMgrConfig(mgrConfig)
		// },
		Use:               "watch type [nameprefix]",
		ValidArgsFunction: makeCobraFunc(cobra.ShellCompDirectiveNoFileComp, completion.NameComplitionFunc),
		Short:             "A brief description of your command",
		Args:              cobra.MinimumNArgs(0),
		//	BashCompletionFunction: "ls -la",
		Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := mgrConfig.GetKubeConfig()
			if err != nil {
				panic(err)
			}
			sc := manager.NewSchemeClient(cfg)
			ctx := context.Background()
			mgrConfig.Names = args
			mgr, err := manager.NewManager(ctx, mgrConfig, sc)
			if err != nil {
				panic(err)
			}
			log.FromContext(ctx).Info("started", "config", mgrConfig)
			if err := mgr.Start(ctx); err != nil {
				panic(err)
			}
			<-ctx.Done()
			if err := ctx.Err(); err != nil {
				panic(err)
			}
		},
	}
	size := GetTtySize()
	watchCmd.PersistentFlags().StringVarP(&mgrConfig.Namespace, "namespace", "n", "", "object namespace prefix")
	watchCmd.PersistentFlags().StringVarP(&mgrConfig.GroupVersion, "group-version", "g", "", "group version")
	watchCmd.PersistentFlags().StringVarP(&mgrConfig.PathPrefix, "path-prefix", "p", "", "object path prefix")
	watchCmd.PersistentFlags().StringVarP(&mgrConfig.PathTemplate, "path-template", "t", "", "object path template")
	watchCmd.PersistentFlags().StringVarP(&mgrConfig.Objects, "kind", "k", "", "kind")
	watchCmd.PersistentFlags().StringVarP(&mgrConfig.ExcludeObjects, "exclude-kind", "", "", "exclude kind")
	watchCmd.PersistentFlags().StringVarP(&mgrConfig.MetricsBindAddress, "metrics-address", "m", ":6666", "metrics address")
	watchCmd.PersistentFlags().BoolVarP(&mgrConfig.EnableAnnotations, "enable-annotations", "a", true, "enable annotations")
	watchCmd.PersistentFlags().BoolVarP(&mgrConfig.IgnoreMetadata, "ignore-metadate", "i", true, "ignore metadata")
	watchCmd.PersistentFlags().BoolVarP(&mgrConfig.SliceOrdering, "slice-ordering", "", true, "slice ordering")
	watchCmd.PersistentFlags().IntVarP(&mgrConfig.ColumnWidthMax, "column-width-max", "", size[1]/4, "column width max")
	watchCmd.PersistentFlags().IntVarP(&mgrConfig.RowWidthMax, "row-width-max", "", size[1], "column width max")
	watchCmd.PersistentFlags().IntVarP(&mgrConfig.MaxRows, "max-rows", "", size[0]-4, "max rows")
	watchCmd.RegisterFlagCompletionFunc("kind", makeCobraFunc(cobra.ShellCompDirectiveNoSpace, completion.KindComplitionFunc))
	watchCmd.RegisterFlagCompletionFunc("exclude-kind", makeCobraFunc(cobra.ShellCompDirectiveNoSpace, completion.KindComplitionFunc))
	watchCmd.RegisterFlagCompletionFunc("namespace", makeCobraFunc(cobra.ShellCompDirectiveNoFileComp, completion.NamespaceCompletionFunc))
	watchCmd.RegisterFlagCompletionFunc("path-prefix", makeCobraFunc(cobra.ShellCompDirectiveNoSpace, completion.PathPrefixComplitionFunc))
	watchCmd.RegisterFlagCompletionFunc("group-version", makeCobraFunc(cobra.ShellCompDirectiveNoFileComp, completion.GroupVersionComplitionFunc))
	rootCmd.AddCommand(watchCmd)
}

func GetTtySize() []int {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, _ := cmd.Output()
	s := string(out)
	s = strings.ReplaceAll(s, "\n", "")
	ll := strings.Split(s, " ")
	res := make([]int, 0)
	for _, l := range ll {
		i, _ := strconv.Atoi(l)
		res = append(res, i)
	}
	if len(res) < 2 {
		res = append(res, 10)
	}
	return res[:2]
}

func makeCobraFunc(directive cobra.ShellCompDirective, f func(ctx context.Context, mgrConfig manager.Config) ([]string, error)) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			mgrConfig.Names = args
		}
		mgrConfig.ToComplete = toComplete
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		fnName := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
		logger := log.FromContext(ctx).WithValues("cobra func", fnName, "config", mgrConfig)
		res, err := f(ctx, mgrConfig)
		if err != nil {
			logger.Error(err, "failed completion")
			return nil, cobra.ShellCompDirectiveError
		}
		logger.Info("completed", "result", res, "direcive", directive)
		return res, directive
	}
}

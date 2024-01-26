package completion

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/nfyxhan/kubewatch/pkg/manager"
)

func TestGroupVersionComplitionFunc(t *testing.T) {
	type args struct {
		ctx    context.Context
		config manager.Config
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := GroupVersionComplitionFunc(tt.args.ctx, tt.args.config)
			fmt.Printf("GroupVersionComplitionFunc() got = %v\n", got)
			if (got1 != nil) != tt.wantErr {
				t.Errorf("GroupVersionComplitionFunc() got1 = %v, want %v", got1, tt.wantErr)
			}
		})
	}
}

func TestNamespaceCompletionFunc(t *testing.T) {
	type args struct {
		ctx    context.Context
		config manager.Config
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := NamespaceCompletionFunc(tt.args.ctx, tt.args.config)
			fmt.Printf("NamespaceCompletionFunc() got = %v\n", got)
			if (got1 != nil) != tt.wantErr {
				t.Errorf("NamespaceCompletionFunc() got1 = %v, want %v", got1, tt.wantErr)
			}
		})
	}
}

func TestPathPrefixComplitionFunc(t *testing.T) {
	type args struct {
		ctx    context.Context
		config manager.Config
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			args: args{
				ctx: context.Background(),
				config: manager.Config{
					Objects:      "deploy",
					GroupVersion: "apps/v1",
					ToComplete:   "Object",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := PathPrefixComplitionFunc(tt.args.ctx, tt.args.config)
			fmt.Printf("PathPrefixComplitionFunc() got = %v, want %v\n", got, tt.want)
			if (got1 != nil) != tt.wantErr {
				t.Errorf("PathPrefixComplitionFunc() got1 = %v, want %v", got1, tt.wantErr)
			}
		})
	}
}
func TestKindComplitionFunc(t *testing.T) {
	type args struct {
		ctx    context.Context
		config manager.Config
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{},
		{
			args: args{
				ctx: context.Background(),
				config: manager.Config{
					Objects:      "deploy",
					GroupVersion: "apps/v1",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := KindAndNameComplitionFunc(tt.args.ctx, tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("KindComplitionFunc() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				fmt.Printf("KindComplitionFunc() = %v, want %v\n", got, tt.want)
			}
		})
	}
}

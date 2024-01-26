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
package utils

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	logger     = log.FromContext(context.Background())
	Kubeconfig string
)

func ListObjectFields(ctx context.Context, obj interface{}, split string) []string {
	if obj == nil {
		return []string{}
	}
	fields := ListFields(obj)
	m := make(map[string][]string)
	for _, f := range fields {
		o := GetFieldByName(obj, f)
		m[f] = ListObjectFields(ctx, o, split)
	}
	res := make([]string, 0)
	for k, v := range m {
		logger.Info("3", k, v)
		if len(v) == 0 {
			res = append(res, k)
		}
		for _, s := range v {
			res = append(res, fmt.Sprintf("%s%s%s", k, split, s))
		}
	}
	return res
}

func GetFieldByName(obj interface{}, fieldName string) interface{} {
	v := reflect.ValueOf(obj)
	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		i, _ := strconv.ParseInt(fieldName, 10, 64)
		return v.Index(int(i)).Interface()
	case reflect.Map:
		for _, k := range v.MapKeys() {
			if k.String() == fieldName {
				return v.MapIndex(k).Interface()
			}
		}
	case reflect.Ptr:
		v = v.Elem()
		return GetFieldByName(v, fieldName)
	case reflect.Struct:
		field := v.FieldByName(fieldName)
		if !field.IsZero() && field.CanInterface() {
			i := field.Interface()
			return i
		}
	default:
		log.FromContext(context.Background()).Error(fmt.Errorf("field kind not support"), "", "kind", v.Kind(), "filed", fieldName, "obj", obj)
	}
	return nil
}

func ListFields(obj interface{}) []string {
	var result []string
	v := reflect.ValueOf(obj)
	t := reflect.TypeOf(obj)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
		t = t.Elem()
	}
	switch v.Kind() {
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if fields := ListFields(v.Index(i).Interface()); len(fields) > 0 {
				result = append(result, fmt.Sprintf("%v", i))
			}
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).CanInterface() {
				field := t.Field(i)
				result = append(result, field.Name)
			}
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			result = append(result, k.String())
		}
	}
	return result
}

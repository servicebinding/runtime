/*
Copyright 2021 the original author or authors.

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

package projector

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/jsonpath"
	"k8s.io/utils/pointer"

	servicebindingv1 "github.com/servicebinding/runtime/apis/v1"
)

// metaPodTemplate contains the subset of a PodTemplateSpec that is appropriate for service binding.
type metaPodTemplate struct {
	workload runtime.Object
	mapping  *servicebindingv1.ClusterWorkloadResourceMappingTemplate

	WorkloadAnnotations    map[string]string
	PodTemplateAnnotations map[string]string
	Containers             []metaContainer
	Volumes                []corev1.Volume
}

// metaContainer contains the aspects of a Container that are appropriate for service binding.
type metaContainer struct {
	Name         *string
	Env          []corev1.EnvVar
	VolumeMounts []corev1.VolumeMount
}

// NewMetaPodTemplate coerces the workload object into a MetaPodTemplate following the mapping definition. The
// resulting MetaPodTemplate may have one or more service bindings applied to it at a time, but should not be reused.
// The workload must be JSON marshalable.
func NewMetaPodTemplate(ctx context.Context, workload runtime.Object, mapping *servicebindingv1.ClusterWorkloadResourceMappingTemplate) (*metaPodTemplate, error) {
	mpt := &metaPodTemplate{
		workload: workload,
		mapping:  mapping,

		WorkloadAnnotations:    map[string]string{},
		PodTemplateAnnotations: map[string]string{},
		Containers:             []metaContainer{},
		Volumes:                []corev1.Volume{},
	}

	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(workload)
	if err != nil {
		return nil, err
	}
	uv := reflect.ValueOf(u)

	if err := mpt.getAt(".metadata.annotations", uv, &mpt.WorkloadAnnotations); err != nil {
		return nil, err
	}
	if err := mpt.getAt(mpt.mapping.Annotations, uv, &mpt.PodTemplateAnnotations); err != nil {
		return nil, err
	}
	for i := range mpt.mapping.Containers {
		cp := jsonpath.New("")
		if err := cp.Parse(fmt.Sprintf("{%s}", mpt.mapping.Containers[i].Path)); err != nil {
			return nil, err
		}
		cr, err := cp.FindResults(u)
		if err != nil {
			// errors are expected if a path is not found
			continue
		}
		for _, cv := range cr[0] {
			mc := metaContainer{
				Name:         nil,
				Env:          []corev1.EnvVar{},
				VolumeMounts: []corev1.VolumeMount{},
			}

			if mpt.mapping.Containers[i].Name != "" {
				// name is optional
				mc.Name = pointer.String("")
				if err := mpt.getAt(mpt.mapping.Containers[i].Name, cv, mc.Name); err != nil {
					return nil, err
				}
			}
			if err := mpt.getAt(mpt.mapping.Containers[i].Env, cv, &mc.Env); err != nil {
				return nil, err
			}
			if err := mpt.getAt(mpt.mapping.Containers[i].VolumeMounts, cv, &mc.VolumeMounts); err != nil {
				return nil, err
			}

			mpt.Containers = append(mpt.Containers, mc)
		}
	}
	if err := mpt.getAt(mpt.mapping.Volumes, uv, &mpt.Volumes); err != nil {
		return nil, err
	}

	return mpt, nil
}

// WriteToWorkload applies mutation defined on the MetaPodTemplate since it was created to the workload resource the
// MetaPodTemplate was created from. This method should generally be called once per instance.
func (mpt *metaPodTemplate) WriteToWorkload(ctx context.Context) error {
	// convert structured workload to unstructured
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(mpt.workload)
	if err != nil {
		return err
	}
	uv := reflect.ValueOf(u)

	if err := mpt.setAt(".metadata.annotations", &mpt.WorkloadAnnotations, uv); err != nil {
		return err
	}
	if err := mpt.setAt(mpt.mapping.Annotations, &mpt.PodTemplateAnnotations, uv); err != nil {
		return err
	}
	ci := 0
	for i := range mpt.mapping.Containers {
		cp := jsonpath.New("")
		if err := cp.Parse(fmt.Sprintf("{%s}", mpt.mapping.Containers[i].Path)); err != nil {
			return err
		}
		cr, err := cp.FindResults(u)
		if err != nil {
			// errors are expected if a path is not found
			continue
		}
		for _, cv := range cr[0] {
			if mpt.mapping.Containers[i].Name != "" && mpt.Containers[ci].Name != nil {
				if err := mpt.setAt(mpt.mapping.Containers[i].Name, mpt.Containers[ci].Name, cv); err != nil {
					return err
				}
			}
			if err := mpt.setAt(mpt.mapping.Containers[i].Env, &mpt.Containers[ci].Env, cv); err != nil {
				return err
			}
			if err := mpt.setAt(mpt.mapping.Containers[i].VolumeMounts, &mpt.Containers[ci].VolumeMounts, cv); err != nil {
				return err
			}

			ci++
		}
	}
	if err := mpt.setAt(mpt.mapping.Volumes, &mpt.Volumes, uv); err != nil {
		return err
	}

	// mutate workload with update content from unstructured
	return runtime.DefaultUnstructuredConverter.FromUnstructured(u, mpt.workload)
}

func (mpt *metaPodTemplate) getAt(ptr string, source reflect.Value, target interface{}) error {
	parent := reflect.ValueOf(nil)
	createIfNil := false
	keys, err := mpt.keys(ptr)
	if err != nil {
		return err
	}
	v, _, _, err := mpt.find(source, parent, keys, "", createIfNil)
	if err != nil {
		return err
	}
	if !v.IsValid() || v.IsNil() {
		return nil
	}
	b, err := json.Marshal(v.Interface())
	if err != nil {
		return err
	}
	return json.Unmarshal(b, target)
}

func (mpt *metaPodTemplate) setAt(ptr string, value interface{}, target reflect.Value) error {
	keys, err := mpt.keys(ptr)
	if err != nil {
		return err
	}
	parent := reflect.ValueOf(nil)
	createIfNil := true
	_, vp, lk, err := mpt.find(target, parent, keys, "", createIfNil)
	if err != nil {
		return err
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var out interface{}
	switch reflect.ValueOf(value).Elem().Kind() {
	case reflect.Map:
		out = map[string]interface{}{}
	case reflect.Slice:
		out = []interface{}{}
	case reflect.String:
		out = ""
	default:
		return fmt.Errorf("unsupported kind %s", reflect.ValueOf(value).Kind())
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return err
	}
	vp.SetMapIndex(reflect.ValueOf(lk), reflect.ValueOf(out))
	return nil
}

func (mpt *metaPodTemplate) keys(ptr string) ([]string, error) {
	p, err := jsonpath.Parse("", fmt.Sprintf("{%s}", ptr))
	if err != nil {
		return nil, err
	}
	return mpt.fieldKeys(p.Root)
}

func (mpt *metaPodTemplate) fieldKeys(node jsonpath.Node) ([]string, error) {
	switch node.Type() {
	case jsonpath.NodeList:
		list := node.(*jsonpath.ListNode)
		paths := []string{}
		for i := range list.Nodes {
			nestedpaths, err := mpt.fieldKeys(list.Nodes[i])
			if err != nil {
				return nil, err
			}
			paths = append(paths, nestedpaths...)
		}
		return paths, nil
	case jsonpath.NodeField:
		field := node.(*jsonpath.FieldNode)
		return []string{field.Value}, nil
	default:
		return nil, fmt.Errorf("unsupported node type %q found", node.Type())
	}
}

func (mpt *metaPodTemplate) find(value, parent reflect.Value, keys []string, lastKey string, createIfNil bool) (reflect.Value, reflect.Value, string, error) {
	if !value.IsValid() || value.IsNil() {
		if !createIfNil {
			return reflect.ValueOf(nil), reflect.ValueOf(nil), "", nil
		}
		value = reflect.ValueOf(make(map[string]interface{}))
		parent.SetMapIndex(reflect.ValueOf(lastKey), value)
	}
	if len(keys) == 0 {
		return value, parent, lastKey, nil
	}
	switch value.Kind() {
	case reflect.Map:
		lastKey = keys[0]
		keys = keys[1:]
		parent = value
		value = value.MapIndex(reflect.ValueOf(lastKey))
		return mpt.find(value, parent, keys, lastKey, createIfNil)
	case reflect.Interface:
		parent = value
		value = value.Elem()
		return mpt.find(value, parent, keys, lastKey, createIfNil)
	default:
		return reflect.ValueOf(nil), parent, lastKey, fmt.Errorf("unhandled kind %q", value.Kind())
	}
}

/*
Copyright 2019 The KubeSphere Authors.

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

package edgeappset

import (
	"reflect"
	"testing"

	appsv1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/apps/v1alpha1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestReconciler_getExpectedUniqNames(t *testing.T) {
	type fields struct {
		Client                  client.Client
		Logger                  logr.Logger
		Recorder                record.EventRecorder
		MaxConcurrentReconciles int
	}
	type args struct {
		instance *appsv1alpha1.EdgeAppSet
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   map[string]struct {
			Status   bool
			Selector *appsv1alpha1.NodeSelector
		}
	}{
		{
			name: "test1",
			fields: fields{
				Client:                  nil,
				Logger:                  logr.Logger{},
				Recorder:                nil,
				MaxConcurrentReconciles: 0,
			},
			args: args{
				instance: &appsv1alpha1.EdgeAppSet{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: appsv1alpha1.EdgeAppSetSpec{
						AppTemplateName:    "",
						Version:            "",
						DeploymentTemplate: appsv1alpha1.DeploymentTemplate{},
						NodeSelectors: []appsv1alpha1.NodeSelector{{
							Project:   "projectA",
							NodeGroup: "groupA",
							NodeName:  "node1",
						}, {
							Project:   "projectA",
							NodeGroup: "groupA",
							NodeName:  "",
						}, {
							Project:   "projectA",
							NodeGroup: "groupA",
							NodeName:  "",
						}, {
							Project:   "projectB",
							NodeGroup: "groupA",
							NodeName:  "",
						}},
					},
					Status: appsv1alpha1.EdgeAppSetStatus{},
				},
			},
			want: map[string]struct {
				Status   bool
				Selector *appsv1alpha1.NodeSelector
			}{
				"projectA-groupA-node1": {
					Status: false,
					Selector: &appsv1alpha1.NodeSelector{
						Project:   "projectA",
						NodeGroup: "groupA",
						NodeName:  "node1",
					}},
				"projectA-groupA-": {
					Status: false,
					Selector: &appsv1alpha1.NodeSelector{
						Project:   "projectA",
						NodeGroup: "groupA",
						NodeName:  "",
					}},
				"projectB-groupA-": {
					Status: false,
					Selector: &appsv1alpha1.NodeSelector{
						Project:   "projectB",
						NodeGroup: "groupA",
						NodeName:  "",
					}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reconciler{
				Client:                  tt.fields.Client,
				Logger:                  tt.fields.Logger,
				Recorder:                tt.fields.Recorder,
				MaxConcurrentReconciles: tt.fields.MaxConcurrentReconciles,
			}
			if got := r.getExpectedUniqNames(tt.args.instance); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Reconciler.getExpectedUniqNames() = %v, want %v", got, tt.want)
			}
		})
	}
}

/*
Copyright 2020 The KubeSphere Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/apps/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeInferModelDeployments implements InferModelDeploymentInterface
type FakeInferModelDeployments struct {
	Fake *FakeAppsV1alpha1
	ns   string
}

var infermodeldeploymentsResource = schema.GroupVersionResource{Group: "apps.edgewize.io", Version: "v1alpha1", Resource: "infermodeldeployments"}

var infermodeldeploymentsKind = schema.GroupVersionKind{Group: "apps.edgewize.io", Version: "v1alpha1", Kind: "InferModelDeployment"}

// Get takes name of the inferModelDeployment, and returns the corresponding inferModelDeployment object, and an error if there is any.
func (c *FakeInferModelDeployments) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.InferModelDeployment, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(infermodeldeploymentsResource, c.ns, name), &v1alpha1.InferModelDeployment{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.InferModelDeployment), err
}

// List takes label and field selectors, and returns the list of InferModelDeployments that match those selectors.
func (c *FakeInferModelDeployments) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.InferModelDeploymentList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(infermodeldeploymentsResource, infermodeldeploymentsKind, c.ns, opts), &v1alpha1.InferModelDeploymentList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.InferModelDeploymentList{ListMeta: obj.(*v1alpha1.InferModelDeploymentList).ListMeta}
	for _, item := range obj.(*v1alpha1.InferModelDeploymentList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested inferModelDeployments.
func (c *FakeInferModelDeployments) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(infermodeldeploymentsResource, c.ns, opts))

}

// Create takes the representation of a inferModelDeployment and creates it.  Returns the server's representation of the inferModelDeployment, and an error, if there is any.
func (c *FakeInferModelDeployments) Create(ctx context.Context, inferModelDeployment *v1alpha1.InferModelDeployment, opts v1.CreateOptions) (result *v1alpha1.InferModelDeployment, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(infermodeldeploymentsResource, c.ns, inferModelDeployment), &v1alpha1.InferModelDeployment{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.InferModelDeployment), err
}

// Update takes the representation of a inferModelDeployment and updates it. Returns the server's representation of the inferModelDeployment, and an error, if there is any.
func (c *FakeInferModelDeployments) Update(ctx context.Context, inferModelDeployment *v1alpha1.InferModelDeployment, opts v1.UpdateOptions) (result *v1alpha1.InferModelDeployment, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(infermodeldeploymentsResource, c.ns, inferModelDeployment), &v1alpha1.InferModelDeployment{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.InferModelDeployment), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeInferModelDeployments) UpdateStatus(ctx context.Context, inferModelDeployment *v1alpha1.InferModelDeployment, opts v1.UpdateOptions) (*v1alpha1.InferModelDeployment, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(infermodeldeploymentsResource, "status", c.ns, inferModelDeployment), &v1alpha1.InferModelDeployment{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.InferModelDeployment), err
}

// Delete takes name of the inferModelDeployment and deletes it. Returns an error if one occurs.
func (c *FakeInferModelDeployments) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(infermodeldeploymentsResource, c.ns, name, opts), &v1alpha1.InferModelDeployment{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeInferModelDeployments) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(infermodeldeploymentsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.InferModelDeploymentList{})
	return err
}

// Patch applies the patch and returns the patched inferModelDeployment.
func (c *FakeInferModelDeployments) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.InferModelDeployment, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(infermodeldeploymentsResource, c.ns, name, pt, data, subresources...), &v1alpha1.InferModelDeployment{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.InferModelDeployment), err
}
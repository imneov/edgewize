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

	v1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/infra/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeVClusterNamespaces implements VClusterNamespaceInterface
type FakeVClusterNamespaces struct {
	Fake *FakeInfraV1alpha1
	ns   string
}

var vclusternamespacesResource = schema.GroupVersionResource{Group: "infra.edgewize.io", Version: "v1alpha1", Resource: "vclusternamespaces"}

var vclusternamespacesKind = schema.GroupVersionKind{Group: "infra.edgewize.io", Version: "v1alpha1", Kind: "VClusterNamespace"}

// Get takes name of the vClusterNamespace, and returns the corresponding vClusterNamespace object, and an error if there is any.
func (c *FakeVClusterNamespaces) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.VClusterNamespace, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(vclusternamespacesResource, c.ns, name), &v1alpha1.VClusterNamespace{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.VClusterNamespace), err
}

// List takes label and field selectors, and returns the list of VClusterNamespaces that match those selectors.
func (c *FakeVClusterNamespaces) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.VClusterNamespaceList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(vclusternamespacesResource, vclusternamespacesKind, c.ns, opts), &v1alpha1.VClusterNamespaceList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.VClusterNamespaceList{ListMeta: obj.(*v1alpha1.VClusterNamespaceList).ListMeta}
	for _, item := range obj.(*v1alpha1.VClusterNamespaceList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested vClusterNamespaces.
func (c *FakeVClusterNamespaces) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(vclusternamespacesResource, c.ns, opts))

}

// Create takes the representation of a vClusterNamespace and creates it.  Returns the server's representation of the vClusterNamespace, and an error, if there is any.
func (c *FakeVClusterNamespaces) Create(ctx context.Context, vClusterNamespace *v1alpha1.VClusterNamespace, opts v1.CreateOptions) (result *v1alpha1.VClusterNamespace, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(vclusternamespacesResource, c.ns, vClusterNamespace), &v1alpha1.VClusterNamespace{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.VClusterNamespace), err
}

// Update takes the representation of a vClusterNamespace and updates it. Returns the server's representation of the vClusterNamespace, and an error, if there is any.
func (c *FakeVClusterNamespaces) Update(ctx context.Context, vClusterNamespace *v1alpha1.VClusterNamespace, opts v1.UpdateOptions) (result *v1alpha1.VClusterNamespace, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(vclusternamespacesResource, c.ns, vClusterNamespace), &v1alpha1.VClusterNamespace{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.VClusterNamespace), err
}

// Delete takes name of the vClusterNamespace and deletes it. Returns an error if one occurs.
func (c *FakeVClusterNamespaces) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(vclusternamespacesResource, c.ns, name, opts), &v1alpha1.VClusterNamespace{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeVClusterNamespaces) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(vclusternamespacesResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.VClusterNamespaceList{})
	return err
}

// Patch applies the patch and returns the patched vClusterNamespace.
func (c *FakeVClusterNamespaces) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.VClusterNamespace, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(vclusternamespacesResource, c.ns, name, pt, data, subresources...), &v1alpha1.VClusterNamespace{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.VClusterNamespace), err
}
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

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
	v1alpha1 "kubesphere.io/kubesphere/pkg/apis/infra/v1alpha1"
)

// FakeEdgeClusters implements EdgeClusterInterface
type FakeEdgeClusters struct {
	Fake *FakeInfraV1alpha1
	ns   string
}

var edgeclustersResource = schema.GroupVersionResource{Group: "infra.edgewize.io", Version: "v1alpha1", Resource: "edgeclusters"}

var edgeclustersKind = schema.GroupVersionKind{Group: "infra.edgewize.io", Version: "v1alpha1", Kind: "EdgeCluster"}

// Get takes name of the edgeCluster, and returns the corresponding edgeCluster object, and an error if there is any.
func (c *FakeEdgeClusters) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.EdgeCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(edgeclustersResource, c.ns, name), &v1alpha1.EdgeCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.EdgeCluster), err
}

// List takes label and field selectors, and returns the list of EdgeClusters that match those selectors.
func (c *FakeEdgeClusters) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.EdgeClusterList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(edgeclustersResource, edgeclustersKind, c.ns, opts), &v1alpha1.EdgeClusterList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.EdgeClusterList{ListMeta: obj.(*v1alpha1.EdgeClusterList).ListMeta}
	for _, item := range obj.(*v1alpha1.EdgeClusterList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested edgeClusters.
func (c *FakeEdgeClusters) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(edgeclustersResource, c.ns, opts))

}

// Create takes the representation of a edgeCluster and creates it.  Returns the server's representation of the edgeCluster, and an error, if there is any.
func (c *FakeEdgeClusters) Create(ctx context.Context, edgeCluster *v1alpha1.EdgeCluster, opts v1.CreateOptions) (result *v1alpha1.EdgeCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(edgeclustersResource, c.ns, edgeCluster), &v1alpha1.EdgeCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.EdgeCluster), err
}

// Update takes the representation of a edgeCluster and updates it. Returns the server's representation of the edgeCluster, and an error, if there is any.
func (c *FakeEdgeClusters) Update(ctx context.Context, edgeCluster *v1alpha1.EdgeCluster, opts v1.UpdateOptions) (result *v1alpha1.EdgeCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(edgeclustersResource, c.ns, edgeCluster), &v1alpha1.EdgeCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.EdgeCluster), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeEdgeClusters) UpdateStatus(ctx context.Context, edgeCluster *v1alpha1.EdgeCluster, opts v1.UpdateOptions) (*v1alpha1.EdgeCluster, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(edgeclustersResource, "status", c.ns, edgeCluster), &v1alpha1.EdgeCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.EdgeCluster), err
}

// Delete takes name of the edgeCluster and deletes it. Returns an error if one occurs.
func (c *FakeEdgeClusters) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(edgeclustersResource, c.ns, name), &v1alpha1.EdgeCluster{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeEdgeClusters) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(edgeclustersResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.EdgeClusterList{})
	return err
}

// Patch applies the patch and returns the patched edgeCluster.
func (c *FakeEdgeClusters) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.EdgeCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(edgeclustersResource, c.ns, name, pt, data, subresources...), &v1alpha1.EdgeCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.EdgeCluster), err
}
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

package v1alpha1

import (
	"context"
	"time"

	v1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/apps/v1alpha1"
	scheme "github.com/edgewize-io/edgewize/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// AppTemplateVersionsGetter has a method to return a AppTemplateVersionInterface.
// A group's client should implement this interface.
type AppTemplateVersionsGetter interface {
	AppTemplateVersions() AppTemplateVersionInterface
}

// AppTemplateVersionInterface has methods to work with AppTemplateVersion resources.
type AppTemplateVersionInterface interface {
	Create(ctx context.Context, appTemplateVersion *v1alpha1.AppTemplateVersion, opts v1.CreateOptions) (*v1alpha1.AppTemplateVersion, error)
	Update(ctx context.Context, appTemplateVersion *v1alpha1.AppTemplateVersion, opts v1.UpdateOptions) (*v1alpha1.AppTemplateVersion, error)
	UpdateStatus(ctx context.Context, appTemplateVersion *v1alpha1.AppTemplateVersion, opts v1.UpdateOptions) (*v1alpha1.AppTemplateVersion, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.AppTemplateVersion, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.AppTemplateVersionList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.AppTemplateVersion, err error)
	AppTemplateVersionExpansion
}

// appTemplateVersions implements AppTemplateVersionInterface
type appTemplateVersions struct {
	client rest.Interface
}

// newAppTemplateVersions returns a AppTemplateVersions
func newAppTemplateVersions(c *AppsV1alpha1Client) *appTemplateVersions {
	return &appTemplateVersions{
		client: c.RESTClient(),
	}
}

// Get takes name of the appTemplateVersion, and returns the corresponding appTemplateVersion object, and an error if there is any.
func (c *appTemplateVersions) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.AppTemplateVersion, err error) {
	result = &v1alpha1.AppTemplateVersion{}
	err = c.client.Get().
		Resource("apptemplateversions").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of AppTemplateVersions that match those selectors.
func (c *appTemplateVersions) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.AppTemplateVersionList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.AppTemplateVersionList{}
	err = c.client.Get().
		Resource("apptemplateversions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested appTemplateVersions.
func (c *appTemplateVersions) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("apptemplateversions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a appTemplateVersion and creates it.  Returns the server's representation of the appTemplateVersion, and an error, if there is any.
func (c *appTemplateVersions) Create(ctx context.Context, appTemplateVersion *v1alpha1.AppTemplateVersion, opts v1.CreateOptions) (result *v1alpha1.AppTemplateVersion, err error) {
	result = &v1alpha1.AppTemplateVersion{}
	err = c.client.Post().
		Resource("apptemplateversions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(appTemplateVersion).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a appTemplateVersion and updates it. Returns the server's representation of the appTemplateVersion, and an error, if there is any.
func (c *appTemplateVersions) Update(ctx context.Context, appTemplateVersion *v1alpha1.AppTemplateVersion, opts v1.UpdateOptions) (result *v1alpha1.AppTemplateVersion, err error) {
	result = &v1alpha1.AppTemplateVersion{}
	err = c.client.Put().
		Resource("apptemplateversions").
		Name(appTemplateVersion.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(appTemplateVersion).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *appTemplateVersions) UpdateStatus(ctx context.Context, appTemplateVersion *v1alpha1.AppTemplateVersion, opts v1.UpdateOptions) (result *v1alpha1.AppTemplateVersion, err error) {
	result = &v1alpha1.AppTemplateVersion{}
	err = c.client.Put().
		Resource("apptemplateversions").
		Name(appTemplateVersion.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(appTemplateVersion).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the appTemplateVersion and deletes it. Returns an error if one occurs.
func (c *appTemplateVersions) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("apptemplateversions").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *appTemplateVersions) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("apptemplateversions").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched appTemplateVersion.
func (c *appTemplateVersions) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.AppTemplateVersion, err error) {
	result = &v1alpha1.AppTemplateVersion{}
	err = c.client.Patch(pt).
		Resource("apptemplateversions").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}

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

package vclusternamespace

import (
	"context"
	"fmt"

	infrav1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/infra/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/utils/sliceutil"
	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/pkg/apis/rbac"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

const (
	controllerName = "vcluster-namespace-controller"
	Finalizer      = "vcluster-namespace.finalizer.edgewize.io"
)

// Reconciler reconciles a Workspace object
type Reconciler struct {
	client.Client
	Logger                  logr.Logger
	Recorder                record.EventRecorder
	MaxConcurrentReconciles int
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.Logger.GetSink() == nil {
		r.Logger = ctrl.Log.WithName("controllers").WithName(controllerName)
	}
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor(controllerName)
	}
	if r.MaxConcurrentReconciles <= 0 {
		r.MaxConcurrentReconciles = 1
	}
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: r.MaxConcurrentReconciles,
		}).
		For(&infrav1alpha1.VClusterNamespace{}).
		Complete(r)
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Logger.WithValues("VClusterNamespace", req.NamespacedName)
	logger.V(7).Info("receive request", "req", req)
	rootCtx := context.Background()
	instance := &infrav1alpha1.VClusterNamespace{}
	if err := r.Get(rootCtx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !sliceutil.HasString(instance.ObjectMeta.Finalizers, Finalizer) {
			logger.V(4).Info("edge cluster is created, add finalizer and update", "req", req, "finalizer", Finalizer)
			if err := r.UpdateInstance(rootCtx, req.NamespacedName, func(_instance *infrav1alpha1.VClusterNamespace) error {
				_instance.ObjectMeta.Finalizers = append(_instance.ObjectMeta.Finalizers, Finalizer)
				return r.Update(rootCtx, _instance)
			}); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if sliceutil.HasString(instance.ObjectMeta.Finalizers, Finalizer) {
			if _, err := r.undoReconcile(ctx, instance); err != nil {
				logger.Error(err, "undoReconcile failed", "instance", instance.Name)
			}
			if err := r.UpdateInstance(rootCtx, req.NamespacedName, func(_instance *infrav1alpha1.VClusterNamespace) error {
				// remove our finalizer from the list and update it.
				_instance.ObjectMeta.Finalizers = sliceutil.RemoveString(_instance.ObjectMeta.Finalizers, func(item string) bool {
					return item == Finalizer
				})
				logger.V(4).Info("update edge cluster")
				return r.Update(rootCtx, _instance)
			}); err != nil {
				logger.Error(err, "update edge cluster failed")
				return ctrl.Result{}, err
			}
		}
		// Our finalizer has finished, so the reconciler can do nothing.
		return ctrl.Result{}, nil
	}
	return r.doReconcile(ctx, instance)
}

func (r *Reconciler) undoReconcile(ctx context.Context, instance *infrav1alpha1.VClusterNamespace) (ctrl.Result, error) {
	// do nothing in current version
	return ctrl.Result{}, nil
}

func (r *Reconciler) doReconcile(ctx context.Context, instance *infrav1alpha1.VClusterNamespace) (ctrl.Result, error) {
	logger := r.Logger.WithName("doReconcile")
	logger.V(3).Info("start reconcile", "instance", instance.Name)

	ns := &corev1.Namespace{}
	err := r.Get(ctx, types.NamespacedName{Name: instance.Name}, ns)
	if client.IgnoreNotFound(err) != nil {
		logger.Error(err, "get namespace failed", "namespace", instance.Name)
		return ctrl.Result{}, err
	} else if err == nil {
		if ns.Labels == nil {
			ns.Labels = map[string]string{}
		}
		if val, ok := ns.Labels["vcluster-namespace.edgewize.io/create-by"]; ok && val == "edgewize" {
			logger.V(4).Info("namespace already exists", "namespace", instance.Name)
			return ctrl.Result{}, nil
		}
		logger.V(4).Info("namespace already exists, but not created by edgewize, skip", "namespace", instance.Name)
		r.Recorder.Event(instance, corev1.EventTypeWarning, "NamespaceExisted", "namespace already exists, but not created by edgewize")
		return ctrl.Result{}, nil
	}
	ns.Name = instance.Name
	if ns.Labels == nil {
		ns.Labels = map[string]string{}
	}
	ns.Labels["vcluster-namespace.edgewize.io/create-by"] = "edgewize"
	if err := r.Create(ctx, ns); err != nil {
		logger.Error(err, "create namespace failed", "namespace", instance.Name)
		return ctrl.Result{}, err
	}
	err = r.createAdminServiceAccount(instance.Name)
	if err != nil {
		logger.Error(err, "create admin service account failed", "namespace", instance.Name)
		return ctrl.Result{}, err
	}
	kubeconfig, err := r.GetAdminKubeConfig(instance.Name, "admin")
	if err != nil {
		logger.Error(err, "get admin kubeconfig failed", "namespace", instance.Name)
		return ctrl.Result{}, err
	}
	err = r.UpdateEdgeWizeNamespaceConfig(instance.Name, kubeconfig)
	if err != nil {
		logger.Error(err, "update edgewize-namespaces-config failed", "namespace", instance.Name)
		return ctrl.Result{}, err
	}

	r.Recorder.Event(instance, corev1.EventTypeNormal, "Succeed", fmt.Sprintf("namespace %s created", instance.Name))
	return ctrl.Result{}, nil
}

func (r *Reconciler) UpdateInstance(ctx context.Context, nn types.NamespacedName, updateFunc func(deployer *infrav1alpha1.VClusterNamespace) error) error {
	logger := r.Logger.WithName("UpdateInstance")
	return retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		instance := &infrav1alpha1.VClusterNamespace{}
		if err := r.Get(ctx, nn, instance); err != nil {
			logger.Error(err, "get instance failed")
			return client.IgnoreNotFound(err)
		}
		return updateFunc(instance)
	})
}

func (r *Reconciler) createAdminServiceAccount(namespace string) error {
	logger := r.Logger.WithName("createAdminServiceAccount")
	ctx := context.Background()
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "admin",
			Namespace: namespace,
		},
	}
	err := r.Create(ctx, sa)
	if err != nil {
		logger.Error(err, "create service account failed")
		return err
	}
	role := &rbac.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "admin",
			Namespace: namespace,
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
		},
	}
	err = r.Create(ctx, role)
	if err != nil {
		logger.Error(err, "create role failed")
		return err
	}
	rolebinding := &rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "admin",
			Namespace: namespace,
		},
		Subjects: []rbac.Subject{
			{
				Kind: "ServiceAccount",
				Name: "admin",
			},
		},
		RoleRef: rbac.RoleRef{
			Kind:     "Role",
			Name:     "admin",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	err = r.Create(ctx, rolebinding)
	if err != nil {
		logger.Error(err, "create rolebinding failed")
		return err
	}
	return nil
}

func (r *Reconciler) GetAdminKubeConfig(namespace string, serviceAccountName string) (string, error) {
	logger := r.Logger.WithName("GetAdminKubeConfig")
	sa := &corev1.ServiceAccount{}
	err := r.Get(context.Background(), types.NamespacedName{namespace, serviceAccountName}, sa)
	if err != nil {
		logger.Error(err, "get service account failed")
		return "", err
	}
	secret := &corev1.Secret{}
	err = r.Get(context.Background(), types.NamespacedName{namespace, sa.Secrets[0].Name}, secret)
	if err != nil {
		logger.Error(err, "get secret failed")
		return "", err
	}

	user := clientcmdapi.NewAuthInfo()
	user.Token = string(secret.Data["token"])

	// 设置 kubeconfig 的当前上下文
	ctx := clientcmdapi.NewContext()
	ctx.Cluster = namespace
	ctx.AuthInfo = fmt.Sprintf("%s-admin", namespace)

	// 设置集群和上下文信息
	cluster := clientcmdapi.NewCluster()
	cluster.Server = "https://kubernetes.default.svc:443"
	cluster.CertificateAuthorityData = secret.Data["ca.crt"]

	kubeconfig := &clientcmdapi.Config{
		APIVersion: "v1",
		Kind:       "Config",
		Preferences: clientcmdapi.Preferences{
			Colors: true,
		},
		Clusters: map[string]*clientcmdapi.Cluster{
			"admin": cluster,
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"admin": user,
		},
		Contexts: map[string]*clientcmdapi.Context{
			"admin": ctx,
		},
	}

	data, err := yaml.Marshal(kubeconfig)
	if err != nil {
		logger.Error(err, "marshal kubeconfig failed")
		return "", err
	}
	return string(data), nil
}

func (r *Reconciler) UpdateEdgeWizeNamespaceConfig(namespace string, kubeconfig string) error {
	logger := r.Logger.WithName("UpdateEdgeWizeNamespaceConfig")
	ctx := context.Background()
	cm := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Namespace: "edgewize-system", Name: "edgewize-namespaces-config"}, cm)
	if err != nil {
		logger.Error(err, "get edgewize-namespaces-config failed")
		return nil
	}
	cm.Data[namespace] = kubeconfig
	err = r.Update(ctx, cm)
	if err != nil {
		logger.Error(err, "update edgewize-namespaces-config failed")
		return err
	}
	return nil
}

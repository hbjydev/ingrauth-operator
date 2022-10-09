/*
Copyright 2022.

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

package controllers

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ingressv1alpha1 "github.com/hbjydev/ingrauth-operator/api/v1alpha1"
)

// IngressReconciler reconciles a Ingress object
type IngressReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=ingress.ingrauth.h4n.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ingress.ingrauth.h4n.io,resources=ingresses/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ingress.ingrauth.h4n.io,resources=ingresses/finalizers,verbs=update

//+kubebuilder:rbac:groups=ingress.ingrauth.h4n.io,resources=users,verbs=get;list
//+kubebuilder:rbac:groups=ingress.ingrauth.h4n.io,resources=users/status,verbs=get;update;patch

//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	var ingrauth ingressv1alpha1.Ingress
	if err := r.Get(ctx, req.NamespacedName, &ingrauth); err != nil {
		l.Error(err, "unable to fetch ingrauth ingress")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var users ingressv1alpha1.UserList
	listOpts := []client.ListOption{
		client.InNamespace(ingrauth.Namespace),
		client.MatchingLabels(ingrauth.Spec.UserSelector),
	}

	if err := r.List(ctx, &users, listOpts...); err != nil {
		return ctrl.Result{}, err
	}

	var childSecret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%v-basic", ingrauth.Name), Namespace: ingrauth.Namespace}, &childSecret); err != nil {
		if errors.IsNotFound(err) {
			secret, err := r.secretForResource(ctx, &ingrauth, users)
			if err != nil {
				l.Error(err, "error generating secret object")
				return ctrl.Result{Requeue: false}, nil
			}

			if err := r.Create(ctx, secret); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	userNames := getUserNames(users.Items)
	if !reflect.DeepEqual(userNames, ingrauth.Status.Users) {
		l.Info("differing user versions, updating auth secret and status", "ingress.name", ingrauth.Name, "ingress.namespace", ingrauth.Namespace)

		vals, err := r.getBasicAuthSlice(ctx, users.Items)
		if err != nil {
			return ctrl.Result{}, err
		}

		childSecret.StringData = map[string]string{
			"auth": strings.Join(vals, "\n"),
		}
		if err := r.Update(ctx, &childSecret); err != nil {
			return ctrl.Result{}, err
		}

		ingrauth.Status.Users = userNames
		if err := r.Status().Update(ctx, &ingrauth); err != nil {
			l.Error(err, "failed to update ingrauth ingress", "ingress.namespace", ingrauth.Namespace, "ingress.name", ingrauth.Name)
			return ctrl.Result{}, err
		}

		return ctrl.Result{Requeue: true}, nil
	}

	var childIngress networkingv1.Ingress
	if err := r.Get(ctx, types.NamespacedName{Name: ingrauth.Name, Namespace: ingrauth.Namespace}, &childIngress); err != nil {
		if errors.IsNotFound(err) {
			ingress := r.ingressForResource(&ingrauth)
			l.Info("creating ingress resource", "ingress.namespace", req.Namespace, "ingress.name", req.Name)
			if err := r.Create(ctx, ingress); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	l.Info("found generated ingress", "ingress.name", childIngress.Name)
	if !reflect.DeepEqual(ingrauth.Spec.Template.Metadata.Labels, childIngress.Labels) {
		childIngress.ObjectMeta.Labels = ingrauth.Spec.Template.Metadata.Labels
		if err := r.Update(ctx, &childIngress); err != nil {
			l.Error(err, "failed to update ingress", "ingress.namespace", childIngress.Namespace, "ingress.name", childIngress.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if !reflect.DeepEqual(ingrauth.Spec.Template.Metadata.Annotations, childIngress.Annotations) {
		childIngress.ObjectMeta.Annotations = ingrauth.Spec.Template.Metadata.Annotations
		if err := r.Update(ctx, &childIngress); err != nil {
			l.Error(err, "failed to update ingress", "ingress.namespace", childIngress.Namespace, "ingress.name", childIngress.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if !reflect.DeepEqual(ingrauth.Spec.Template.Spec, childIngress.Spec) {
		childIngress.Spec = ingrauth.Spec.Template.Spec
		if err := r.Update(ctx, &childIngress); err != nil {
			l.Error(err, "failed to update ingress", "ingress.namespace", childIngress.Namespace, "ingress.name", childIngress.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if ingrauth.Status.IngressResource != childIngress.Name {
		ingrauth.Status.IngressResource = childIngress.Name
		if err := r.Status().Update(ctx, &ingrauth); err != nil {
			l.Error(err, "failed to update ingress", "ingress.namespace", childIngress.Namespace, "ingress.name", childIngress.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if ingrauth.Status.AuthSecret != childSecret.Name {
		ingrauth.Status.AuthSecret = childSecret.Name
		if err := r.Status().Update(ctx, &ingrauth); err != nil {
			l.Error(err, "failed to update ingress", "ingress.namespace", childIngress.Namespace, "ingress.name", childIngress.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

func (r *IngressReconciler) ingressForResource(i *ingressv1alpha1.Ingress) *networkingv1.Ingress {
	ingress := networkingv1.Ingress{
		ObjectMeta: v1.ObjectMeta{
			Name:        i.Name,
			Namespace:   i.Namespace,
			Labels:      i.Spec.Template.Metadata.Labels,
			Annotations: i.Spec.Template.Metadata.Annotations,
		},
		Spec: i.Spec.Template.Spec,
	}

	ctrl.SetControllerReference(i, &ingress, r.Scheme)

	return &ingress
}

func (r *IngressReconciler) secretForResource(ctx context.Context, i *ingressv1alpha1.Ingress, users ingressv1alpha1.UserList) (*corev1.Secret, error) {
	basicAuthUsers, err := r.getBasicAuthSlice(ctx, users.Items)
	if err != nil {
		return nil, err
	}

	secret := corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:        fmt.Sprintf("%v-basic", i.Name),
			Namespace:   i.Namespace,
			Labels:      i.Spec.Template.Metadata.Labels,
			Annotations: i.Spec.Template.Metadata.Annotations,
		},
		StringData: map[string]string{
			"auth": strings.Join(basicAuthUsers, "\n"),
		},
	}

	ctrl.SetControllerReference(i, &secret, r.Scheme)

	return &secret, nil
}

func (r *IngressReconciler) secretsUpdated(ctx context.Context, status *ingressv1alpha1.IngressStatus, items []ingressv1alpha1.User) (bool, error) {
	for _, v := range items {
		name := v.Name
		secretName := v.Status.UserSecret

		var secret corev1.Secret
		if err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: v.Namespace}, &secret); err != nil {
			return false, err
		}

		if secret.ObjectMeta.ResourceVersion != status.Users[name] {
			return true, nil
		}
	}

	return false, nil
}

func (r *IngressReconciler) getBasicAuthSlice(ctx context.Context, items []ingressv1alpha1.User) ([]string, error) {
	basicAuthUsers := []string{}

	for _, v := range items {
		name := v.Name

		secretName := v.Status.UserSecret
		var secret corev1.Secret
		if err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: v.Namespace}, &secret); err != nil {
			return nil, err
		}

		hash, err := bcrypt.GenerateFromPassword(secret.Data["password"], bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}

		log.Log.Info("thing happened, generated it all", "value", string(hash))

		str := fmt.Sprintf("%v:%v", name, string(hash))
		basicAuthUsers = append(basicAuthUsers, str)
	}

	return basicAuthUsers, nil
}

func getUserNames(items []ingressv1alpha1.User) map[string]string {
	names := map[string]string{}
	for _, v := range items {
		names[v.Name] = v.ResourceVersion
	}
	return names
}

// SetupWithManager sets up the controller with the Manager.
func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ingressv1alpha1.Ingress{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

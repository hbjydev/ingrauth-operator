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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	ingressv1alpha1 "github.com/hbjydev/ingrauth-operator/api/v1alpha1"
	"github.com/hbjydev/ingrauth-operator/util"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=ingress.ingrauth.h4n.io,resources=users,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ingress.ingrauth.h4n.io,resources=users/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ingress.ingrauth.h4n.io,resources=users/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	user := &ingressv1alpha1.User{}
	if err := r.Get(ctx, req.NamespacedName, user); err != nil {
		l.Error(err, "unable to fetch user")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	l.Info("found user object", "labels", user.Labels)

	childSecret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: user.Status.UserSecret, Namespace: user.Namespace}, childSecret); err != nil {
		if errors.IsNotFound(err) {
			return r.createNewSecretForUser(ctx, l, req, user)
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *UserReconciler) secretForUser(u *ingressv1alpha1.User) *corev1.Secret {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%v-%v", u.Name, util.RandLowerRunes(5)),
			Namespace:   u.Namespace,
			Labels:      u.Labels,
			Annotations: u.Annotations,
		},

		StringData: map[string]string{
			"password": "lol u thought this would be generated, fukkin imagine",
		},
	}

	ctrl.SetControllerReference(u, &secret, r.Scheme)

	return &secret
}

func (r *UserReconciler) createNewSecretForUser(ctx context.Context, l logr.Logger, req ctrl.Request, user *ingressv1alpha1.User) (ctrl.Result, error) {
	secret := r.secretForUser(user)
	user.Status.UserSecret = secret.Name
	if err := r.Status().Update(ctx, user); err != nil {
		return ctrl.Result{}, err
	}

	l.Info("creating secret", "secret.namespace", req.Namespace, "secret.name", req.Name)
	if err := r.Create(ctx, secret); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ingressv1alpha1.User{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

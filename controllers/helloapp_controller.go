/*
Copyright 2023.

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

	apps "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"encoding/json"

	"github.com/go-logr/logr"
	appsv1 "github.com/scorta-d/operator.git/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HelloAppReconciler reconciles a HelloApp object
type HelloAppReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func indent(obj interface{}) string {
	var result string = ""
	data, err2 := json.MarshalIndent(obj, "", "   ")
	if err2 == nil {
		result = string(data)
	}
	return result
}

func inPrintf(fs string, obj interface{}) string {
	return fmt.Sprintf(fs, indent(obj))
}

func (recons *HelloAppReconciler) resizeDeployment(
	deployment *apps.Deployment,
	size int32, ctx context.Context, log logr.Logger,
) bool {
	var change bool = false
	var repl = *deployment.Spec.Replicas
	log.Info(fmt.Sprintf("spec.replicas = %v", repl))
	if repl != size {
		log.Info(fmt.Sprintf("Resize is required: %d vs %d requested", repl, size))
		*deployment.Spec.Replicas = size
		change = true
		log.Info("Resize done")
	} else {
		log.Info("Resize is not required")
	}
	return change
}
func (recons *HelloAppReconciler) reimageDeployment(
	deployment *apps.Deployment,
	image string, ctx context.Context, log logr.Logger,
) bool {
	var change bool = false
	for i := range deployment.Spec.Template.Spec.Containers {
		var container = &deployment.Spec.Template.Spec.Containers[i]
		if image != container.Image {
			container.Image = image
			log.Info(
				fmt.Sprintf(
					"Images: \n %v\n %v\n %v\n",
					image,
					deployment.Spec.Template.Spec.Containers[i].Image,
					container.Image,
				),
			)
			change = true
		}
	}
	return change
}

func (recons *HelloAppReconciler) createDeployment(
	deployment *apps.Deployment, hello *appsv1.HelloApp,
	size int32, image string, args []string, ctx context.Context,
) error {
	var log = log.FromContext(ctx)
	log.Info(fmt.Sprintf("Create deployment with %d replicas", size))
	var err error = nil
	labels := map[string]string{"a": "b"}

	deployment.ObjectMeta = metav1.ObjectMeta{
		Name:      hello.Name,
		Namespace: hello.Namespace,
	}

	deployment.Spec = apps.DeploymentSpec{
		Replicas: &size,
		Selector: &metav1.LabelSelector{
			MatchLabels: labels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Image: image,
					Name:  hello.Name,
					Args:  args,
				}},
			},
		},
	}
	err = ctrl.SetControllerReference(hello, deployment, recons.Scheme)
	if err == nil {
		cli := recons.Client
		err = cli.Create(ctx, deployment)
	}
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelloAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.HelloApp{}).
		Complete(r)
}

//+kubebuilder:rbac:groups=apps.dz,resources=helloapps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.dz,resources=helloapps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.dz,resources=helloapps/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the HelloApp object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (recons *HelloAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	var log = log.FromContext(ctx)
	log.Info("--- Process begin ---")

	var hello = &appsv1.HelloApp{}
	var cli = recons.Client
	err = cli.Get(ctx, req.NamespacedName, hello)
	if err == nil {
		var size = hello.Spec.Size
		var image = hello.Spec.Image
		var args = hello.Spec.Args
		log.Info(inPrintf("Request = %s", req))
		log.Info(fmt.Sprintf("Required size = %d, Image: %s, args: %v", size, image, args))
		log.Info(inPrintf("Spec: %s", hello.Spec))

		var deployment = &apps.Deployment{}
		var nsName = types.NamespacedName{
			Name:      hello.Name,
			Namespace: hello.Namespace,
		}
		log.Info(fmt.Sprintf("Try to get: %v", deployment))
		err = cli.Get(ctx, nsName, deployment)
		log.Info(fmt.Sprintf("Get err: %v", err))

		if err == nil {
			log.Info(inPrintf("Deployment exists: %s", deployment))
			var change bool = false
			change = change || recons.resizeDeployment(deployment, size, ctx, log)
			change = change || recons.reimageDeployment(deployment, image, ctx, log)
			if change {
				log.Info("Image change to be applied")
				var cli client.Client = recons.Client
				err = cli.Update(ctx, deployment)
				log.Info(fmt.Sprintf("Deployment updated: %v", err))
			}
		} else if errors.IsNotFound(err) {
			log.Info("Not found any deployment")

			err = recons.createDeployment(deployment, hello, size, image, args, ctx)
		}
	} else if errors.IsNotFound(err) {
		log.Info("CR Not found")
	} else {
		log.Error(err, "Something is wrong")
	}
	log.Info("--- Process end ---")
	return ctrl.Result{}, err
}

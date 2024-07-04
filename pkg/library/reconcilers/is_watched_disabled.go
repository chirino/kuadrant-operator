//go:build !integration

package reconcilers

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

// Complete is similar to the controller-runtime's builder.Complete method.  It takes
// the build result and captures the controller as field of the base reconciler.
func (b *BaseReconciler) Complete(controller controller.Controller, err error) error {
	return err
}

// Client returns a split client that reads objects from
// the cache and writes to the Kubernetes APIServer
func (b *BaseReconciler) Client() client.Client {
	return b.client
}

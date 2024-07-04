//go:build integration

package reconcilers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func init() {
	// replace the default controller builder with our own
	builder.NewController = newWatchedController
}

// Complete is similar to the controller-runtime's builder.Complete method.  It takes
// the build result and captures the controller as field of the base reconciler.
func (b *BaseReconciler) Complete(controller controller.Controller, err error) error {
	if err != nil {
		return err
	}
	isWatchedController := controller.(*isWatchedController)
	b.IsWatched = isWatchedController.IsWatched
	return nil
}

// Client returns a split client that reads objects from
// the cache and writes to the Kubernetes APIServer
func (b *BaseReconciler) Client() client.Client {
	if b.IsWatched != nil {
		return newIsWatchedClient(b.client, b.logger, b.IsWatched)
	}
	return b.client
}

type watch struct {
	Predicates []predicate.Predicate
	Type       client.Object
}

type sourceKind interface {
	ObjectType() client.Object
}

func newWatchedController(name string, mgr manager.Manager, options controller.Options) (controller.Controller, error) {
	c, err := controller.New(name, mgr, options)
	if err != nil {
		return nil, err
	}

	return &isWatchedController{
		Controller: c,
	}, nil
}

type isWatchedController struct {
	controller.Controller
	watches []watch
}

func (c *isWatchedController) Watch(src source.Source, eventhandler handler.EventHandler, predicates ...predicate.Predicate) error {
	switch src := src.(type) {
	case sourceKind:
		c.watches = append(c.watches, watch{
			Type:       src.ObjectType(),
			Predicates: predicates,
		})
	default:
		fmt.Printf("Unknown source type: %+v\n", src)
	}
	return c.Controller.Watch(src, eventhandler, predicates...)
}

func (c *isWatchedController) IsWatched(o client.Object) bool {
	for _, w := range c.watches {

		// Is this the right way to compare types?
		if reflect.TypeOf(o) != reflect.TypeOf(w.Type) {
			continue
		}

		// bail if one of the predicates does not match...
		for _, p := range w.Predicates {
			if !p.Create(event.CreateEvent{Object: o}) &&
				!p.Delete(event.DeleteEvent{Object: o}) &&
				!p.Generic(event.GenericEvent{Object: o}) &&
				!p.Update(event.UpdateEvent{
					// this might not be good enough
					ObjectOld: o,
					ObjectNew: o,
				}) {
				return false
			}
		}

		return true
	}
	return false
}

func newIsWatchedClient(client client.Client, logger logr.Logger, isWatched func(o client.Object) bool) client.Client {
	return &isWatchedClient{
		Client:    client,
		isWatched: isWatched,
		logger:    logger,
	}
}

type isWatchedClient struct {
	client.Client
	logger    logr.Logger
	isWatched func(o client.Object) bool
}

func (c *isWatchedClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	err := c.Client.Get(ctx, key, obj, opts...)
	if err == nil {
		if !c.isWatched(obj) {
			// let's be really loud about this so that we see these problems quickly, this code only happens in tests... so I think it's fine
			c.logger.V(0).Info("Get without watch", "gvk", obj.GetObjectKind().GroupVersionKind(), "namespace", obj.GetNamespace(), "name", obj.GetName())
			// kinda wish we could include the reconciler's name here
			fmt.Printf("Get without watch: %s=%+v, %s=%s, %s=%s\n", "gvk", obj.GetObjectKind().GroupVersionKind(), "namespace", obj.GetNamespace(), "name", obj.GetName())
		}
	}
	return err
}

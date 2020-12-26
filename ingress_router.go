package ingressutil

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/glog"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type ingressRouter struct {
	mutex         sync.Mutex
	ingressMap    map[string]*v1beta1.Ingress
	reloadChannel chan struct{}
	routemap      atomic.Value
}

// NewIngressRouter is used to create an IngressRouter
func NewIngressRouter() IngressRouter {
	return &ingressRouter{
		ingressMap:    make(map[string]*v1beta1.Ingress),
		reloadChannel: make(chan struct{}, 1000),
	}
}

func (ir *ingressRouter) StartAutoUpdate(ctx context.Context, kubeClient *kubernetes.Clientset) func() {
	factory := informers.NewSharedInformerFactory(kubeClient, time.Minute)
	informer := factory.Extensions().V1beta1().Ingresses().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ir.addIngress,
		UpdateFunc: ir.updateIngress,
		DeleteFunc: ir.removeIngress,
	})
	go informer.Run(ctx.Done())

	go func() {
		for !informer.HasSynced() {
			ir.waitForUpdates()
		}
		ir.updateRouteMap()
		ir.reloadPeriodically(ctx)
	}()

	return func() {
		for ir.getRouteMap() == nil {
			time.Sleep(25 * time.Millisecond)
		}
	}
}

func (ir *ingressRouter) MatchRequest(r *http.Request) (string, string, string, bool) {
	hostname := GetHostname(r)
	routeMap := ir.getRouteMap()
	if routeMap == nil {
		glog.Errorln("Received a request to MatchRequest before routemap was filled")
		return "", "", "", false
	}
	return routeMap.match(hostname, r.URL.Path)
}

func (ir *ingressRouter) reloadPeriodically(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ir.reloadChannel:
			if !ok {
				return
			}
			ir.waitForUpdates()
			ir.updateRouteMap()
		}
	}
}

func (ir *ingressRouter) waitForUpdates() {
	timer := time.NewTimer(50 * time.Millisecond)
	for {
		select {
		case <-timer.C:
			return
		case _, ok := <-ir.reloadChannel:
			if !ok {
				return
			}
		}
	}
}

func (ir *ingressRouter) updateRouteMap() {
	glog.Info("Rebuilding the route map")
	ir.mutex.Lock()
	defer ir.mutex.Unlock()

	routeMap := newRouteMap()

	for _, ingress := range ir.ingressMap {
		routeMap.addIngress(ingress)
	}

	routeMap.sort()
	ir.routemap.Store(routeMap)
}

func (ir *ingressRouter) addIngress(obj interface{}) {
	ingress, ok := obj.(*v1beta1.Ingress)
	if !ok {
		glog.Errorf("Expect an Ingress, got an object of type %T \n", obj)
		return
	}

	ir.addIngressToMap(ingress)
}

func (ir *ingressRouter) addIngressToMap(ingress *v1beta1.Ingress) {
	ingressKey := ingress.Namespace + "/" + ingress.Name
	glog.Info("Adding Ingress", ingressKey)
	ir.mutex.Lock()
	ir.ingressMap[ingressKey] = ingress
	ir.mutex.Unlock()

	ir.reloadChannel <- struct{}{}
}

func (ir *ingressRouter) updateIngress(oldObj, newObj interface{}) {
	oldIngress, ok := oldObj.(*v1beta1.Ingress)
	if !ok {
		glog.Errorf("Expect an Ingress, got an object of type %T \n", oldObj)
		return
	}

	newIngress, ok := newObj.(*v1beta1.Ingress)
	if !ok {
		glog.Errorf("Expect an Ingress, got an object of type %T \n", newObj)
		return
	}

	if areIngressesTheSame(oldIngress, newIngress) {
		return
	}

	ir.addIngressToMap(newIngress)
}

func (ir *ingressRouter) removeIngress(obj interface{}) {
	ingress, ok := obj.(*v1beta1.Ingress)
	if !ok {
		glog.Errorf("Expect an Ingress, got an object of type %T \n", obj)
		return
	}

	ir.removeIngressFromMap(ingress)
}

func (ir *ingressRouter) removeIngressFromMap(ingress *v1beta1.Ingress) {
	ingressKey := ingress.Namespace + "/" + ingress.Name
	glog.Info("Removing Ingress", ingressKey)
	ir.mutex.Lock()
	delete(ir.ingressMap, ingressKey)
	ir.mutex.Unlock()
	ir.reloadChannel <- struct{}{}
}

func (ir *ingressRouter) getRouteMap() *routeMap {
	routemap, ok := ir.routemap.Load().(*routeMap)
	if !ok {
		return nil
	}
	return routemap
}

func areIngressesTheSame(ig1, ig2 *v1beta1.Ingress) bool {
	if len(ig1.Spec.Rules) != len(ig2.Spec.Rules) {
		return false
	}

	for i := range ig1.Spec.Rules {
		rule1 := ig1.Spec.Rules[i]
		rule2 := ig2.Spec.Rules[i]

		// Skip non http rules
		if rule1.HTTP == nil && rule2.HTTP == nil {
			continue
		}

		if rule1.HTTP == nil || rule2.HTTP == nil || rule1.Host != rule2.Host || len(rule1.HTTP.Paths) != len(rule2.HTTP.Paths) {
			return false
		}

		for j := range rule1.HTTP.Paths {
			path1 := rule1.HTTP.Paths[j]
			path2 := rule2.HTTP.Paths[j]

			if path1.Path != path2.Path || path1.Backend.ServiceName != path2.Backend.ServiceName || path1.Backend.ServicePort != path2.Backend.ServicePort {
				return false
			}
		}
	}

	return true
}

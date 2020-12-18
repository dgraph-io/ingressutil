package ingressutil

import (
	"sort"
	"strings"

	"k8s.io/api/extensions/v1beta1"
)

type routeMapEntry struct {
	ingress *v1beta1.Ingress
	path    v1beta1.HTTPIngressPath
}

type routeMap struct {
	hostMap map[string][]routeMapEntry
}

func newRouteMap() *routeMap {
	return &routeMap{
		hostMap: make(map[string][]routeMapEntry),
	}
}

func (rm *routeMap) addIngress(ingress *v1beta1.Ingress) {
	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			rm.hostMap[rule.Host] = append(rm.hostMap[rule.Host], routeMapEntry{ingress, path})
		}
	}
}

func (rm *routeMap) sort() {
	for _, paths := range rm.hostMap {
		sort.Slice(paths, func(i, j int) bool {
			return len(paths[i].path.Path) > len(paths[j].path.Path)
		})
	}
}

func (rm *routeMap) match(host, path string) (string, string, string, bool) {
	for _, entry := range rm.hostMap[host] {
		if strings.HasPrefix(path, entry.path.Path) {
			return entry.ingress.Namespace, entry.ingress.Name, entry.path.Backend.ServiceName + "." + entry.ingress.Namespace + ".svc:" + entry.path.Backend.ServicePort.String(), true
		}
	}
	return "", "", "", false
}

// Package ingressutil provides utilities for building a kubernetes ingress controller
package ingressutil

import (
	"context"
	"net/http"
	"strings"

	"k8s.io/client-go/kubernetes"
)

// IngressRouter monitors updates to kube ingresses, and maintains a routing table based on hostname
// and path. NOTE: This does not currently support wildcard domains, or regex in paths
type IngressRouter interface {
	// MatchRequest matches an HTTP Request to the namespace, service and upstream address with port to forward the HTTP request to.
	MatchRequest(r *http.Request) (namespace, name, addr string, found bool)

	// StartAutoUpdate starts listening to kube for updates. It returns a function which can be called to block until all ingresses have been read
	StartAutoUpdate(ctx context.Context, kubeClient *kubernetes.Clientset) (waitTillReady func())
}

func GetHostname(r *http.Request) string {
	hostname := r.Header.Get("Host")
	if hostname == "" {
		hostname = r.Host
	}
	if idx := strings.IndexByte(hostname, ':'); idx >= 0 {
		hostname = hostname[:idx]
	}

	return hostname
}

var _ IngressRouter = (*ingressRouter)(nil)
var _ IngressRouter = (*IngressRouterStub)(nil)

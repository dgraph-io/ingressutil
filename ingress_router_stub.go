package ingressutil

import (
	"context"
	"net/http"

	"k8s.io/client-go/kubernetes"
)

// IngressRouterStub can be used to create an ingress router for tests or local dev. It simply returns the fields in it's properties
type IngressRouterStub struct {
	Namespace string
	Name      string
	Upstream  string
}

func (ir *IngressRouterStub) StartAutoUpdate(ctx context.Context, kubeClient *kubernetes.Clientset) func() {
	return func() {}
}

func (ir *IngressRouterStub) MatchRequest(r *http.Request) (string, string, string, bool) {
	return ir.Namespace, ir.Name, ir.Upstream, true
}

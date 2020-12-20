package caddyingressutil

import (
	"context"
	"net/http"
	"sync"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"github.com/dgraph-io/ingressutil"
	"github.com/golang/glog"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func init() {
  caddy.RegisterModule(IngressRouter{})
}

type IngressRouter struct {
	Router       ingressutil.IngressRouter
	BuildHandler func(string, string, string) caddyhttp.MiddlewareHandler

	proxyMap      sync.Map
	caddyContext  caddy.Context
	cleanupRouter func()
}

// CaddyModule returns the Caddy module information.
func (ir IngressRouter) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "http.handlers.ingress_router",
		New: func() caddy.Module {
			return &IngressRouter{}
		},
	}
}

func (ir *IngressRouter) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	proxy, ok := ir.getUpstreamProxy(r)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
		return nil
	}

	return proxy.ServeHTTP(w, r, next)
}

func (ir *IngressRouter) Provision(ctx caddy.Context) error {
	ir.caddyContext = ctx
	if ir.Router == nil {
		ir.Router, ir.cleanupRouter = ProvisionIngressRouter()
	}
	return nil
}

func (ir *IngressRouter) Cleanup() error {
	if ir.cleanupRouter != nil {
		ir.cleanupRouter()
	}
	ir.proxyMap.Range(func(_, obj interface{}) bool {
		proxy, ok := obj.(caddy.CleanerUpper)
		if !ok {
			return true
		}
		proxy.Cleanup()
		return true
	})
	return nil
}

func (ir *IngressRouter) getUpstreamProxy(r *http.Request) (caddyhttp.MiddlewareHandler, bool) {
	namespace, service, upstream, ok := ir.Router.MatchRequest(r)
	if !ok {
		return nil, false
	}

	if proxy, ok := ir.proxyMap.Load(upstream); ok {
		return proxy.(caddyhttp.MiddlewareHandler), true
	}

	proxy := ir.buildHandler(namespace, service, upstream)
	if provisioner, ok := proxy.(caddy.Provisioner); ok {
		provisioner.Provision(ir.caddyContext)
	}
	proxyInMap, loaded := ir.proxyMap.LoadOrStore(upstream, proxy)
	if loaded {
		if cleaner, ok := proxy.(caddy.CleanerUpper); ok {
			cleaner.Cleanup()
		}
	}
	return proxyInMap.(caddyhttp.MiddlewareHandler), true
}

func (ir *IngressRouter) buildHandler(namespace, service, upstreamAddress string) caddyhttp.MiddlewareHandler {
	if ir.BuildHandler != nil {
		return ir.BuildHandler(namespace, service, upstreamAddress)
	}
	return &reverseproxy.Handler{
		Upstreams: reverseproxy.UpstreamPool{&reverseproxy.Upstream{Dial: upstreamAddress}},
	}
}

var routerStruct = struct {
	init          sync.Once
	ingressRouter ingressutil.IngressRouter
	wg            sync.WaitGroup
}{
	init: sync.Once{},
}

func ProvisionIngressRouter() (ingressutil.IngressRouter, func()) {
	routerStruct.wg.Add(1)
	routerStruct.init.Do(func() {
		ctx, stop := context.WithCancel(context.Background())

		go func() {
			routerStruct.wg.Wait()
			stop()
		}()

		ingressRouter := ingressutil.NewIngressRouter()

		cfg, err := rest.InClusterConfig()
		if err != nil {
			glog.Fatalln(err)
		}

		kubeClient, err := kubernetes.NewForConfig(cfg)
		if err != nil {
			glog.Fatalln(err)
		}
		ingressRouter.StartAutoUpdate(ctx, kubeClient)()

		routerStruct.ingressRouter = ingressRouter
	})
	return routerStruct.ingressRouter, func() { routerStruct.wg.Done() }
}

// Interface guards
var (
	_ caddy.Provisioner           = (*IngressRouter)(nil)
	_ caddyhttp.MiddlewareHandler = (*IngressRouter)(nil)
	_ caddy.CleanerUpper          = (*IngressRouter)(nil)
)

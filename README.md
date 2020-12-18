# ingressutil

Utilities for writing an ingress controller.

Currently, this package only exposes one type.

## [IngressRouter](https://godoc.org/github.com/dgraph-io/ingressutil#IngressRouter)

* This type can be used to monitor for changes to an ingress in Kubernetes. Use StartAutoUpdate() to trigger this.
* IngressRouter can match an HTTP Request to a namespace, service and upstream address using `MatchRequest`
* Currently, this only does exact hostname match, and path prefix. This means that wildcard hosts and path regexes aren't supported

## Blog Post

Coming Soon!

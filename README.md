# ingressutil

Utilities for writing an ingress controller.

Currently, this package only exposes one type.

## IngressRouter

* This type can be used to monitor for changes to an ingress in Kubernetes
* Currently, this only does exact hostname match, and path prefix. This means that wildcard hosts and path regexes aren't supported

## Blog Post

Coming Soon!

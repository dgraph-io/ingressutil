package ingressutil

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/api/extensions/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestIngressRouter(t *testing.T) {
	ingress1 := &v1beta1.Ingress{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "ns1",
			Name:      "name1",
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: "foo.com",
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								{
									Path: "/",
									Backend: v1beta1.IngressBackend{
										ServiceName: "svc11",
										ServicePort: intstr.FromInt(80),
									},
								},
							},
						},
					},
				},
				{
					Host: "bar.com",
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								{
									Path: "/subpath",
									Backend: v1beta1.IngressBackend{
										ServiceName: "svc12",
										ServicePort: intstr.FromInt(80),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	ingress2 := &v1beta1.Ingress{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "ns2",
			Name:      "name2",
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: "bar.com",
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								{
									Path: "/",
									Backend: v1beta1.IngressBackend{
										ServiceName: "svc2",
										ServicePort: intstr.FromInt(80),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	ingress3 := &v1beta1.Ingress{
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: "foo.com",
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								{
									Path: "/",
									Backend: v1beta1.IngressBackend{
										ServiceName: "svc1",
										ServicePort: intstr.FromInt(80),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	t.Run("It matches routes successfully", func(t *testing.T) {
		ir := NewIngressRouter().(*ingressRouter)

		ir.addIngress(ingress1)
		ir.addIngress(ingress2)
		ir.addIngress(ingress3)

		ir.updateRouteMap()

		t.Run("It matches on the first ingress", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "https://foo.com/path1", nil)
			require.NoError(t, err)
			namespace, ingress, svc, ok := ir.MatchRequest(req)
			require.True(t, ok)
			require.Equal(t, "ns1", namespace)
			require.Equal(t, "name1", ingress)
			require.Equal(t, "svc11.ns1.svc:80", svc)
		})

		t.Run("It matches subsequent ingresses", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "https://bar.com/some-path", nil)
			require.NoError(t, err)
			namespace, ingress, svc, ok := ir.MatchRequest(req)
			require.True(t, ok)
			require.Equal(t, "ns2", namespace)
			require.Equal(t, "name2", ingress)
			require.Equal(t, "svc2.ns2.svc:80", svc)
		})

		t.Run("It always matches the longer path", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "https://bar.com/subpath", nil)
			require.NoError(t, err)
			namespace, ingress, svc, ok := ir.MatchRequest(req)
			require.True(t, ok)
			require.Equal(t, "ns1", namespace)
			require.Equal(t, "name1", ingress)
			require.Equal(t, "svc12.ns1.svc:80", svc)
		})

		t.Run("It returns false for a host not found", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "https://unknown/subpath", nil)
			require.NoError(t, err)
			namespace, ingress, svc, ok := ir.MatchRequest(req)
			require.False(t, ok)
			require.Equal(t, "", namespace)
			require.Equal(t, "", ingress)
			require.Equal(t, "", svc)
		})

		t.Run("It returns false for a path not found", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "https://baz.com/other-path", nil)
			require.NoError(t, err)
			namespace, ingress, svc, ok := ir.MatchRequest(req)
			require.False(t, ok)
			require.Equal(t, "", namespace)
			require.Equal(t, "", ingress)
			require.Equal(t, "", svc)
		})
	})

	t.Run("It can delete an ingress sucessfully", func(t *testing.T) {
		ir := NewIngressRouter().(*ingressRouter)
		ir.addIngress(ingress1)
		ir.updateRouteMap()
		ir.removeIngress(ingress1)
		ir.updateRouteMap()

		req, err := http.NewRequest(http.MethodGet, "https://foo.com/path1", nil)
		require.NoError(t, err)
		namespace, ingress, svc, ok := ir.MatchRequest(req)
		require.False(t, ok)
		require.Equal(t, "", namespace)
		require.Equal(t, "", ingress)
		require.Equal(t, "", svc)
	})
}

func TestAreIngressesTheSame(t *testing.T) {
	ingress := &v1beta1.Ingress{
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: "foo.com",
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								{
									Path: "/",
									Backend: v1beta1.IngressBackend{
										ServiceName: "svc1",
										ServicePort: intstr.FromInt(80),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	t.Run("It knows the same object is the same", func(t *testing.T) {
		require.True(t, areIngressesTheSame(ingress, ingress))
	})

	t.Run("It knows a duplicate is the same", func(t *testing.T) {
		require.True(t, areIngressesTheSame(ingress, &v1beta1.Ingress{
			Spec: v1beta1.IngressSpec{
				Rules: []v1beta1.IngressRule{
					{
						Host: "foo.com",
						IngressRuleValue: v1beta1.IngressRuleValue{
							HTTP: &v1beta1.HTTPIngressRuleValue{
								Paths: []v1beta1.HTTPIngressPath{
									{
										Path: "/",
										Backend: v1beta1.IngressBackend{
											ServiceName: "svc1",
											ServicePort: intstr.FromInt(80),
										},
									},
								},
							},
						},
					},
				},
			},
		}))
	})

	t.Run("It knows an ingress without rules", func(t *testing.T) {
		require.False(t, areIngressesTheSame(ingress, &v1beta1.Ingress{
			Spec: v1beta1.IngressSpec{
				Rules: []v1beta1.IngressRule{},
			},
		}))
	})

	t.Run("It knows an ingress without http is different", func(t *testing.T) {
		require.False(t, areIngressesTheSame(ingress, &v1beta1.Ingress{
			Spec: v1beta1.IngressSpec{
				Rules: []v1beta1.IngressRule{
					{
						Host:             "foo.com",
						IngressRuleValue: v1beta1.IngressRuleValue{},
					},
				},
			},
		}))
	})

	t.Run("It knows the ingress is different if the host has changed", func(t *testing.T) {
		require.False(t, areIngressesTheSame(ingress, &v1beta1.Ingress{
			Spec: v1beta1.IngressSpec{
				Rules: []v1beta1.IngressRule{
					{
						Host: "bar.com",
						IngressRuleValue: v1beta1.IngressRuleValue{
							HTTP: &v1beta1.HTTPIngressRuleValue{
								Paths: []v1beta1.HTTPIngressPath{
									{
										Path: "/",
										Backend: v1beta1.IngressBackend{
											ServiceName: "svc1",
											ServicePort: intstr.FromInt(80),
										},
									},
								},
							},
						},
					},
				},
			},
		}))
	})

	t.Run("It knows an ingress with a different path is different", func(t *testing.T) {
		require.False(t, areIngressesTheSame(ingress, &v1beta1.Ingress{
			Spec: v1beta1.IngressSpec{
				Rules: []v1beta1.IngressRule{
					{
						Host: "foo.com",
						IngressRuleValue: v1beta1.IngressRuleValue{
							HTTP: &v1beta1.HTTPIngressRuleValue{
								Paths: []v1beta1.HTTPIngressPath{
									{
										Path: "/another-path",
										Backend: v1beta1.IngressBackend{
											ServiceName: "svc1",
											ServicePort: intstr.FromInt(80),
										},
									},
								},
							},
						},
					},
				},
			},
		}))
	})

	t.Run("It knows an ingress with a different service", func(t *testing.T) {
		require.False(t, areIngressesTheSame(ingress, &v1beta1.Ingress{
			Spec: v1beta1.IngressSpec{
				Rules: []v1beta1.IngressRule{
					{
						Host: "foo.com",
						IngressRuleValue: v1beta1.IngressRuleValue{
							HTTP: &v1beta1.HTTPIngressRuleValue{
								Paths: []v1beta1.HTTPIngressPath{
									{
										Path: "/",
										Backend: v1beta1.IngressBackend{
											ServiceName: "svc2",
											ServicePort: intstr.FromInt(80),
										},
									},
								},
							},
						},
					},
				},
			},
		}))
	})
}

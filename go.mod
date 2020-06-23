module github.com/vmware-labs/reconciler-runtime

go 1.14

require (
	github.com/go-logr/logr v0.1.0
	github.com/google/go-cmp v0.4.0
	golang.org/x/net v0.0.0-20191004110552-13f9640d40b9
	k8s.io/api v0.18.2
	k8s.io/apimachinery v0.18.2
	k8s.io/client-go v0.18.2
	sigs.k8s.io/controller-runtime v0.6.0
	sigs.k8s.io/structured-merge-diff v0.0.0-20190525122527-15d366b2352e // indirect
)

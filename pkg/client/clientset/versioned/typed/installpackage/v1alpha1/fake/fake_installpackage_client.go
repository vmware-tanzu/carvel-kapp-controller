// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "github.com/vmware-tanzu/carvel-kapp-controller/pkg/client/clientset/versioned/typed/installpackage/v1alpha1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeInstallV1alpha1 struct {
	*testing.Fake
}

func (c *FakeInstallV1alpha1) InstalledPackages(namespace string) v1alpha1.InstalledPackageInterface {
	return &FakeInstalledPackages{c, namespace}
}

func (c *FakeInstallV1alpha1) PackageRepositories() v1alpha1.PackageRepositoryInterface {
	return &FakePackageRepositories{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeInstallV1alpha1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}

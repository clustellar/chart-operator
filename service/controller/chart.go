package controller

import (
	"github.com/giantswarm/apiextensions/pkg/apis/core/v1alpha1"
	"github.com/giantswarm/apiextensions/pkg/clientset/versioned"
	"github.com/giantswarm/apprclient"
	"github.com/giantswarm/helmclient"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/giantswarm/operatorkit/client/k8scrdclient"
	"github.com/giantswarm/operatorkit/framework"
	"github.com/giantswarm/operatorkit/informer"
	"github.com/spf13/afero"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"

	"github.com/giantswarm/chart-operator/service/controller/v1"
)

type ChartConfig struct {
	ApprClient   apprclient.Interface
	Fs           afero.Fs
	G8sClient    versioned.Interface
	HelmClient   helmclient.Interface
	K8sClient    kubernetes.Interface
	K8sExtClient apiextensionsclient.Interface
	Logger       micrologger.Logger

	ProjectName    string
	WatchNamespace string
}

type Chart struct {
	*framework.Framework
}

func NewChart(config ChartConfig) (*Chart, error) {
	var err error

	if config.G8sClient == nil {
		return nil, microerror.Maskf(invalidConfigError, "config.G8sClient must not be empty")
	}
	if config.K8sClient == nil {
		return nil, microerror.Maskf(invalidConfigError, "config.K8sClient must not be empty")
	}
	if config.K8sExtClient == nil {
		return nil, microerror.Maskf(invalidConfigError, "config.K8sExtClient must not be empty")
	}
	if config.Logger == nil {
		return nil, microerror.Maskf(invalidConfigError, "config.Logger must not be empty")
	}

	if config.ProjectName == "" {
		return nil, microerror.Maskf(invalidConfigError, "config.ProjectName must not be empty")
	}

	var crdClient *k8scrdclient.CRDClient
	{
		c := k8scrdclient.Config{
			K8sExtClient: config.K8sExtClient,
			Logger:       config.Logger,
		}

		crdClient, err = k8scrdclient.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	resourceRouter, err := newChartResourceRouter(config)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	var newInformer *informer.Informer
	{
		c := informer.Config{
			Watcher: config.G8sClient.CoreV1alpha1().ChartConfigs(config.WatchNamespace),

			RateWait:     informer.DefaultRateWait,
			ResyncPeriod: informer.DefaultResyncPeriod,
		}

		newInformer, err = informer.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var crdFramework *framework.Framework
	{
		c := framework.Config{
			CRD:            v1alpha1.NewChartConfigCRD(),
			CRDClient:      crdClient,
			Informer:       newInformer,
			K8sClient:      config.K8sClient,
			Logger:         config.Logger,
			ResourceRouter: resourceRouter,

			Name: config.ProjectName,
		}

		crdFramework, err = framework.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	c := &Chart{
		Framework: crdFramework,
	}

	return c, nil
}

func newChartResourceRouter(config ChartConfig) (*framework.ResourceRouter, error) {
	var err error

	var resourceSetV1 *framework.ResourceSet
	{
		c := v1.ResourceSetConfig{
			ApprClient:  config.ApprClient,
			Fs:          config.Fs,
			HelmClient:  config.HelmClient,
			K8sClient:   config.K8sClient,
			Logger:      config.Logger,
			ProjectName: config.ProjectName,
		}

		resourceSetV1, err = v1.NewResourceSet(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var resourceRouter *framework.ResourceRouter
	{
		c := framework.ResourceRouterConfig{
			Logger: config.Logger,
			ResourceSets: []*framework.ResourceSet{
				resourceSetV1,
			},
		}

		resourceRouter, err = framework.NewResourceRouter(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	return resourceRouter, nil
}
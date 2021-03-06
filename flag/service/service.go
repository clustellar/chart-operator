package service

import (
	"github.com/giantswarm/chart-operator/flag/service/cnr"
	"github.com/giantswarm/chart-operator/flag/service/helm"
	"github.com/giantswarm/chart-operator/flag/service/kubernetes"
)

// Service is an intermediate data structure for command line configuration flags.
type Service struct {
	CNR        cnr.CNR
	Helm       helm.Helm
	Kubernetes kubernetes.Kubernetes
}

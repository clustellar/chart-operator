package chart

import (
	"context"
	"reflect"
	"testing"

	"github.com/giantswarm/apiextensions/pkg/apis/core/v1alpha1"
	"github.com/giantswarm/micrologger/microloggertest"
	"github.com/spf13/afero"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_DesiredState(t *testing.T) {
	testCases := []struct {
		name          string
		obj           *v1alpha1.ChartConfig
		configMap     *apiv1.ConfigMap
		expectedState ChartState
		errorMatcher  func(error) bool
	}{
		{
			name: "case 0: basic match",
			obj: &v1alpha1.ChartConfig{
				Spec: v1alpha1.ChartConfigSpec{
					Chart: v1alpha1.ChartConfigSpecChart{
						Name:    "chart-operator-chart",
						Channel: "0-1-beta",
						Release: "chart-operator",
					},
				},
			},
			expectedState: ChartState{
				ChartName:      "chart-operator-chart",
				ChartValues:    map[string]interface{}{},
				ChannelName:    "0-1-beta",
				ReleaseName:    "chart-operator",
				ReleaseVersion: "0.1.2",
			},
		},
		{
			name: "case 1: basic match with empty config map",
			obj: &v1alpha1.ChartConfig{
				Spec: v1alpha1.ChartConfigSpec{
					Chart: v1alpha1.ChartConfigSpecChart{
						Name: "chart-operator-chart",
						ConfigMap: v1alpha1.ChartConfigSpecConfigMap{
							Name:      "chart-operator-values-configmap",
							Namespace: "giantswarm",
						},
						Channel: "0.1-beta",
						Release: "chart-operator",
					},
				},
			},
			configMap: &apiv1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "chart-operator-values-configmap",
					Namespace: "giantswarm",
				},
				Data: map[string]string{},
			},
			expectedState: ChartState{
				ChartName:      "chart-operator-chart",
				ChartValues:    map[string]interface{}{},
				ChannelName:    "0.1-beta",
				ReleaseName:    "chart-operator",
				ReleaseVersion: "0.1.2",
			},
		},
		{
			name: "case 2: basic match with config map value",
			obj: &v1alpha1.ChartConfig{
				Spec: v1alpha1.ChartConfigSpec{
					Chart: v1alpha1.ChartConfigSpecChart{
						Name: "chart-operator-chart",
						ConfigMap: v1alpha1.ChartConfigSpecConfigMap{
							Name:      "chart-operator-values-configmap",
							Namespace: "giantswarm",
						},
						Channel: "0-1-beta",
						Release: "chart-operator",
					},
				},
			},
			configMap: &apiv1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "chart-operator-values-configmap",
					Namespace: "giantswarm",
				},
				Data: map[string]string{
					"values.json": `{ "test": "test" }`,
				},
			},
			expectedState: ChartState{
				ChartName: "chart-operator-chart",
				ChartValues: map[string]interface{}{
					"test": "test",
				},
				ChannelName:    "0-1-beta",
				ReleaseName:    "chart-operator",
				ReleaseVersion: "0.1.2",
			},
		},
		{
			name: "case 3: config map with multiple values",
			obj: &v1alpha1.ChartConfig{
				Spec: v1alpha1.ChartConfigSpec{
					Chart: v1alpha1.ChartConfigSpecChart{
						Name: "chart-operator-chart",
						ConfigMap: v1alpha1.ChartConfigSpecConfigMap{
							Name:      "chart-operator-values-configmap",
							Namespace: "giantswarm",
						},
						Channel: "0-1-beta",
						Release: "chart-operator",
					},
				},
			},
			configMap: &apiv1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "chart-operator-values-configmap",
					Namespace: "giantswarm",
				},
				Data: map[string]string{
					"values.json": `{ "provider": "azure", "replicas": 2 }`,
				},
			},
			expectedState: ChartState{
				ChartName: "chart-operator-chart",
				ChartValues: map[string]interface{}{
					"provider": "azure",
					// Numeric values in JSON will be deserialized to a float64.
					"replicas": float64(2),
				},
				ChannelName:    "0-1-beta",
				ReleaseName:    "chart-operator",
				ReleaseVersion: "0.1.2",
			},
		},
		{
			name: "case 4: config map not found",
			obj: &v1alpha1.ChartConfig{
				Spec: v1alpha1.ChartConfigSpec{
					Chart: v1alpha1.ChartConfigSpecChart{
						Name: "chart-operator-chart",
						ConfigMap: v1alpha1.ChartConfigSpecConfigMap{
							Name:      "chart-operator-values-configmap",
							Namespace: "giantswarm",
						},
						Channel: "0-1-beta",
						Release: "chart-operator",
					},
				},
			},
			configMap: &apiv1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "missing-values-configmap",
					Namespace: "giantswarm",
				},
			},
			errorMatcher: IsNotFound,
		},
	}

	apprClient := &apprMock{
		defaultReleaseVersion: "0.1.2",
	}
	helmClient := &helmMock{
		defaultError: nil,
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			objs := make([]runtime.Object, 0, 0)
			if tc.configMap != nil {
				objs = append(objs, tc.configMap)
			}

			c := Config{
				ApprClient: apprClient,
				Fs:         afero.NewMemMapFs(),
				HelmClient: helmClient,
				K8sClient:  fake.NewSimpleClientset(objs...),
				Logger:     microloggertest.New(),
			}
			r, err := New(c)
			if err != nil {
				t.Fatalf("error == %#v, want nil", err)
			}

			result, err := r.GetDesiredState(context.TODO(), tc.obj)
			switch {
			case err != nil && tc.errorMatcher == nil:
				t.Fatalf("error == %#v, want nil", err)
			case err == nil && tc.errorMatcher != nil:
				t.Fatalf("error == nil, want non-nil")
			case err != nil && !tc.errorMatcher(err):
				t.Fatalf("error == %#v, want matching", err)
			}

			chartState, err := toChartState(result)
			if err != nil {
				t.Fatalf("error == %#v, want nil", err)
			}

			if !reflect.DeepEqual(chartState, tc.expectedState) {
				t.Fatalf("ChartState == %#v, want %#v", chartState, tc.expectedState)
			}
		})
	}

}

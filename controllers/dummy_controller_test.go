package controllers

import (
	"context"
	interviewcomv1alpha1 "github.com/AgentNemo00/operator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"testing"
)

func initClient(t *testing.T) client.Client {
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}
	testEnvCfg, err := testEnv.Start()
	if err != nil {
		t.Errorf(err.Error())
		return nil
	}
	err = interviewcomv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		t.Errorf(err.Error())
		return nil
	}
	k8sClient, err = client.New(testEnvCfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		t.Errorf(err.Error())
		return nil
	}
	return k8sClient
}

func examplePod(name string, namespace string) *v1.Pod {
	return &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1.PodSpec{Containers: []v1.Container{{Name: "example", Image: "nginx"}}},
	}
}

func TestDummyReconciler_deletePod(t *testing.T) {
	type fields struct {
		Client client.Client
	}
	type args struct {
		ctx       context.Context
		name      string
		namespace string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "success",
			fields: fields{
				Client: func(t *testing.T) client.Client {
					c := initClient(t)
					if c == nil {
						return c
					}
					err := c.Create(context.Background(), examplePod("example-pod", "default"))
					if err != nil {
						t.Errorf(err.Error())
						return nil
					}
					return c
				}(t),
			},
			args: args{
				ctx:       context.Background(),
				name:      "example-pod",
				namespace: "default",
			},
			wantErr: false,
		},
		{
			name: "not found",
			fields: fields{
				Client: func(t *testing.T) client.Client {
					return initClient(t)
				}(t),
			},
			args: args{
				ctx:       context.Background(),
				name:      "example-pod",
				namespace: "default",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &DummyReconciler{
				Client: tt.fields.Client,
			}
			if err := r.deletePod(tt.args.ctx, tt.args.name, tt.args.namespace); (err != nil) != tt.wantErr {
				t.Errorf("deletePod() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

package gcp

import (
	"context"
	"errors"
	"testing"

	runpb "cloud.google.com/go/run/apiv2/runpb"
	"github.com/googleapis/gax-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRunServiceClient struct {
	getServiceFn    func(context.Context, *runpb.GetServiceRequest, ...gax.CallOption) (*runpb.Service, error)
	updateServiceFn func(context.Context, *runpb.UpdateServiceRequest, ...gax.CallOption) (runUpdateOperation, error)
	closeFn         func() error
}

func (f *fakeRunServiceClient) Close() error {
	if f.closeFn != nil {
		return f.closeFn()
	}
	return nil
}

func (f *fakeRunServiceClient) GetService(ctx context.Context, req *runpb.GetServiceRequest, opts ...gax.CallOption) (*runpb.Service, error) {
	return f.getServiceFn(ctx, req, opts...)
}

func (f *fakeRunServiceClient) UpdateService(ctx context.Context, req *runpb.UpdateServiceRequest, opts ...gax.CallOption) (runUpdateOperation, error) {
	return f.updateServiceFn(ctx, req, opts...)
}

type fakeRunUpdateOperation struct {
	waitFn func(context.Context, ...gax.CallOption) (*runpb.Service, error)
}

func (f *fakeRunUpdateOperation) Wait(ctx context.Context, opts ...gax.CallOption) (*runpb.Service, error) {
	return f.waitFn(ctx, opts...)
}

func TestNewRunClient(t *testing.T) {
	t.Parallel()

	originalFactory := newRunServiceClient
	t.Cleanup(func() {
		newRunServiceClient = originalFactory
	})

	t.Run("success", func(t *testing.T) {
		fakeClient := &fakeRunServiceClient{}
		newRunServiceClient = func(context.Context) (runServiceAPI, error) {
			return fakeClient, nil
		}

		client, err := NewRunClient(context.Background())
		require.NoError(t, err)
		require.NotNil(t, client)
		assert.Same(t, fakeClient, client.client)
	})

	t.Run("error", func(t *testing.T) {
		newRunServiceClient = func(context.Context) (runServiceAPI, error) {
			return nil, errors.New("boom")
		}

		client, err := NewRunClient(context.Background())
		require.Error(t, err)
		assert.Nil(t, client)
		assert.ErrorContains(t, err, "create cloud run client")
	})
}

func TestNewRunClientSequential(t *testing.T) {
	originalFactory := newRunServiceClient
	t.Cleanup(func() {
		newRunServiceClient = originalFactory
	})

	newRunServiceClient = func(context.Context) (runServiceAPI, error) {
		return &fakeRunServiceClient{}, nil
	}

	client, err := NewRunClient(context.Background())
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestRunClientClose(t *testing.T) {
	t.Parallel()

	client := &RunClient{
		client: &fakeRunServiceClient{
			closeFn: func() error { return errors.New("boom") },
		},
	}

	err := client.Close()
	require.Error(t, err)
	assert.ErrorContains(t, err, "boom")
}

func TestRunClientRollbackTraffic(t *testing.T) {
	t.Parallel()

	client := &RunClient{
		client: &fakeRunServiceClient{
			getServiceFn: func(_ context.Context, req *runpb.GetServiceRequest, _ ...gax.CallOption) (*runpb.Service, error) {
				assert.Equal(t, "projects/project/locations/us-central1/services/api", req.Name)
				return &runpb.Service{}, nil
			},
			updateServiceFn: func(_ context.Context, req *runpb.UpdateServiceRequest, _ ...gax.CallOption) (runUpdateOperation, error) {
				require.NotNil(t, req.Service)
				require.Len(t, req.Service.Traffic, 1)
				assert.Equal(t, int32(100), req.Service.Traffic[0].Percent)
				assert.Equal(t, "rev-2", req.Service.Traffic[0].Revision)
				assert.Equal(t, []string{"traffic"}, req.UpdateMask.Paths)
				return &fakeRunUpdateOperation{
					waitFn: func(context.Context, ...gax.CallOption) (*runpb.Service, error) {
						return &runpb.Service{}, nil
					},
				}, nil
			},
		},
	}

	require.NoError(t, client.RollbackTraffic(context.Background(), "project", "us-central1", "api", "rev-2"))
}

func TestRunClientRollbackTrafficErrors(t *testing.T) {
	t.Parallel()

	t.Run("get service", func(t *testing.T) {
		t.Parallel()
		client := &RunClient{
			client: &fakeRunServiceClient{
				getServiceFn: func(context.Context, *runpb.GetServiceRequest, ...gax.CallOption) (*runpb.Service, error) {
					return nil, errors.New("boom")
				},
				updateServiceFn: func(context.Context, *runpb.UpdateServiceRequest, ...gax.CallOption) (runUpdateOperation, error) {
					t.Fatal("unexpected update")
					return nil, nil
				},
			},
		}
		err := client.RollbackTraffic(context.Background(), "project", "region", "api", "rev")
		require.Error(t, err)
		assert.ErrorContains(t, err, "get cloud run service")
	})

	t.Run("update service", func(t *testing.T) {
		t.Parallel()
		client := &RunClient{
			client: &fakeRunServiceClient{
				getServiceFn: func(context.Context, *runpb.GetServiceRequest, ...gax.CallOption) (*runpb.Service, error) {
					return &runpb.Service{}, nil
				},
				updateServiceFn: func(context.Context, *runpb.UpdateServiceRequest, ...gax.CallOption) (runUpdateOperation, error) {
					return nil, errors.New("boom")
				},
			},
		}
		err := client.RollbackTraffic(context.Background(), "project", "region", "api", "rev")
		require.Error(t, err)
		assert.ErrorContains(t, err, "update cloud run traffic")
	})

	t.Run("wait", func(t *testing.T) {
		t.Parallel()
		client := &RunClient{
			client: &fakeRunServiceClient{
				getServiceFn: func(context.Context, *runpb.GetServiceRequest, ...gax.CallOption) (*runpb.Service, error) {
					return &runpb.Service{}, nil
				},
				updateServiceFn: func(context.Context, *runpb.UpdateServiceRequest, ...gax.CallOption) (runUpdateOperation, error) {
					return &fakeRunUpdateOperation{
						waitFn: func(context.Context, ...gax.CallOption) (*runpb.Service, error) {
							return nil, errors.New("boom")
						},
					}, nil
				},
			},
		}
		err := client.RollbackTraffic(context.Background(), "project", "region", "api", "rev")
		require.Error(t, err)
		assert.ErrorContains(t, err, "wait for cloud run rollback")
	})
}

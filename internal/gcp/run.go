package gcp

import (
	"context"
	"fmt"

	run "cloud.google.com/go/run/apiv2"
	runpb "cloud.google.com/go/run/apiv2/runpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type RunClient struct {
	client *run.ServicesClient
}

func NewRunClient(ctx context.Context) (*RunClient, error) {
	client, err := run.NewServicesClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create cloud run client: %w", err)
	}

	return &RunClient{client: client}, nil
}

func (c *RunClient) Close() error {
	return c.client.Close()
}

func (c *RunClient) RollbackTraffic(ctx context.Context, projectID, region, serviceName, revision string) error {
	resourceName := fmt.Sprintf("projects/%s/locations/%s/services/%s", projectID, region, serviceName)
	service, err := c.client.GetService(ctx, &runpb.GetServiceRequest{Name: resourceName})
	if err != nil {
		return fmt.Errorf("get cloud run service: %w", err)
	}

	service.Traffic = []*runpb.TrafficTarget{
		{
			Type:     runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_REVISION,
			Percent:  100,
			Revision: revision,
		},
	}

	op, err := c.client.UpdateService(ctx, &runpb.UpdateServiceRequest{
		Service: service,
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: []string{"traffic"},
		},
	})
	if err != nil {
		return fmt.Errorf("update cloud run traffic: %w", err)
	}

	if _, err := op.Wait(ctx); err != nil {
		return fmt.Errorf("wait for cloud run rollback: %w", err)
	}

	return nil
}

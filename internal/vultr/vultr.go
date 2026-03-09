package vultrlayer

import (
	"context"
	"fmt"

	"github.com/vultr/govultr/v3"
	"golang.org/x/oauth2"
)

const SingleServerLabel = "SingleCoreKeeperServer"

type VultrLayer struct {
	vultrClient *govultr.Client
}

func New(ctx context.Context, apiKey string) *VultrLayer {
	config := &oauth2.Config{}
	tokenSrc := config.TokenSource(ctx, &oauth2.Token{AccessToken: apiKey})
	vultrClient := govultr.NewClient(oauth2.NewClient(ctx, tokenSrc))
	vultrClient.SetRateLimit(500)

	return &VultrLayer{vultrClient: vultrClient}
}

func (vl *VultrLayer) StartInstance(ctx context.Context, instanceID string) error {
	return vl.vultrClient.Instance.Start(ctx, instanceID)
}

func (vl *VultrLayer) StopInstance(ctx context.Context, instanceID string) error {
	return vl.vultrClient.Instance.Halt(ctx, instanceID)
}

func (vl *VultrLayer) GetInstance(ctx context.Context, instanceID string) (instance *govultr.Instance, err error) {
	instance, httpResponse, err := vl.vultrClient.Instance.Get(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	httpResponse.Body.Close() // why do we even get the raw response back from the SDK?
	return instance, nil
}

func (vl *VultrLayer) ListInstances(ctx context.Context) ([]govultr.Instance, error) {
	instances, meta, resp, err := vl.vultrClient.Instance.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	_ = meta
	return instances, nil
}

func (vl *VultrLayer) GetSingleServerInstanceByLabel(ctx context.Context, label string) (*govultr.Instance, error) {
	instances, meta, resp, err := vl.vultrClient.Instance.List(ctx, &govultr.ListOptions{Label: label})
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	_ = meta
	if len(instances) > 1 {
		return nil, fmt.Errorf("more than one instance found for label %s", label)
	}
	return &instances[0], nil
}

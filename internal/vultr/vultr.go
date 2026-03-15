package vultrlayer

import (
	"4dmiral/discordServerManager/internal/secrets"
	"context"
	"encoding/base64"
	"fmt"

	"github.com/vultr/govultr/v3"
	"golang.org/x/oauth2"
)

const (
	MaxServerCount = 3
	InstancePlan   = "vx1-g-2c-8g-120s"
	InstanceRegion = "sea"
	InstanceOSID   = 2284 // Ubuntu 24.04
)

var sshKeyID string

type VultrLayer struct {
	vultrClient *govultr.Client
}

func New(ctx context.Context, apiKey string) (*VultrLayer, error) {
	config := &oauth2.Config{}
	tokenSrc := config.TokenSource(ctx, &oauth2.Token{AccessToken: apiKey})
	vultrClient := govultr.NewClient(oauth2.NewClient(ctx, tokenSrc))
	vultrClient.SetRateLimit(500)

	sshKey, _, err := vultrClient.SSHKey.Get(ctx, secrets.Secrets.VultrSSHKeyID)
	if err != nil {
		return nil, err
	}
	sshKeyID = sshKey.ID

	return &VultrLayer{vultrClient: vultrClient}, nil
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

func (vl *VultrLayer) GetInstanceByLabel(ctx context.Context, label string) (*govultr.Instance, error) {
	instances, err := vl.ListInstances(ctx)
	if err != nil {
		return nil, err
	}
	for i := range instances {
		if instances[i].Label == label {
			return &instances[i], nil
		}
	}
	return nil, fmt.Errorf("no instance found with label %q", label)
}

func (vl *VultrLayer) CreateInstance(ctx context.Context, label, startupScript string) (*govultr.Instance, error) {
	osID := InstanceOSID
	req := &govultr.InstanceCreateReq{
		Region:   InstanceRegion,
		Plan:     InstancePlan,
		OsID:     osID,
		Label:    label,
		Hostname: label,
		UserData: base64.StdEncoding.EncodeToString([]byte(startupScript)),
		SSHKeys:  []string{sshKeyID},
	}
	instance, resp, err := vl.vultrClient.Instance.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create instance: %w", err)
	}
	resp.Body.Close()
	return instance, nil
}

func (vl *VultrLayer) DestroyInstance(ctx context.Context, instanceID string) error {
	return vl.vultrClient.Instance.Delete(ctx, instanceID)
}

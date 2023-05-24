package custompollers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/go-azure-sdk/resource-manager/resources/2022-09-01/providers"
	"github.com/hashicorp/go-azure-sdk/sdk/client/pollers"
)

var _ pollers.PollerType = &resourceProviderUnregistrationPoller{}

func NewResourceProviderUnregistrationPoller(client *providers.ProvidersClient, id providers.SubscriptionProviderId) *resourceProviderUnregistrationPoller {
	return &resourceProviderUnregistrationPoller{
		client: client,
		id:     id,
	}
}

type resourceProviderUnregistrationPoller struct {
	client *providers.ProvidersClient
	id     providers.SubscriptionProviderId
}

func (p *resourceProviderUnregistrationPoller) Poll(ctx context.Context) (*pollers.PollResult, error) {
	resp, err := p.client.Get(ctx, p.id, providers.DefaultGetOperationOptions())
	if err != nil {
		return nil, fmt.Errorf("retrieving %s: %+v", p.id, err)
	}

	registrationState := ""
	if model := resp.Model; model != nil && model.RegistrationState != nil {
		registrationState = *model.RegistrationState
	}

	if strings.EqualFold(registrationState, "Unregistered") {
		return &pollers.PollResult{
			Status:       pollers.PollingStatusSucceeded,
			PollInterval: 10 * time.Second,
		}, nil
	}

	// Processing
	return &pollers.PollResult{
		Status:       pollers.PollingStatusInProgress,
		PollInterval: 10 * time.Second,
	}, nil
}

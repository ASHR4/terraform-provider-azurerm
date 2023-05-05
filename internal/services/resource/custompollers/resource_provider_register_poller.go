package custompollers

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/go-azure-sdk/resource-manager/resources/2022-09-01/providers"
	"github.com/hashicorp/go-azure-sdk/sdk/client/pollers"
)

var _ pollers.PollerType = &resourceProviderRegistrationPoller{}

func NewResourceProviderRegistrationPoller(client *providers.ProvidersClient, id providers.SubscriptionProviderId) *resourceProviderRegistrationPoller {
	return &resourceProviderRegistrationPoller{
		client: client,
		id:     id,
	}
}

type resourceProviderRegistrationPoller struct {
	client *providers.ProvidersClient
	id     providers.SubscriptionProviderId
}

func (p *resourceProviderRegistrationPoller) Poll(ctx context.Context) (*pollers.PollResult, error) {
	resp, err := p.client.Get(ctx, p.id, providers.DefaultGetOperationOptions())
	if err != nil {
		return nil, fmt.Errorf("retrieving %s: %+v", err)
	}

	registrationState := ""
	if model := resp.Model; model != nil && model.RegistrationState != nil {
		registrationState = *model.RegistrationState
	}

	if strings.EqualFold(registrationState, "Registered") {
		return &pollers.PollResult{
			Status: pollers.PollingStatusSucceeded,
		}, nil
	}

	// Processing
	return &pollers.PollResult{
		Status: pollers.PollingStatusInProgress,
	}, nil
}

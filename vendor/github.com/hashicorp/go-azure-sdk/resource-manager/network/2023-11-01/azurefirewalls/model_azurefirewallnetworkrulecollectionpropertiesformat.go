package azurefirewalls

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License. See NOTICE.txt in the project root for license information.

type AzureFirewallNetworkRuleCollectionPropertiesFormat struct {
	Action            *AzureFirewallRCAction      `json:"action,omitempty"`
	Priority          *int64                      `json:"priority,omitempty"`
	ProvisioningState *ProvisioningState          `json:"provisioningState,omitempty"`
	Rules             *[]AzureFirewallNetworkRule `json:"rules,omitempty"`
}

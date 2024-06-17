package applicationgateways

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License. See NOTICE.txt in the project root for license information.

type ApplicationGatewaySslPredefinedPolicyPropertiesFormat struct {
	CipherSuites       *[]ApplicationGatewaySslCipherSuite `json:"cipherSuites,omitempty"`
	MinProtocolVersion *ApplicationGatewaySslProtocol      `json:"minProtocolVersion,omitempty"`
}

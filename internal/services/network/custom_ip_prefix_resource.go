package network

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonschema"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/location"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/network/parse"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/network/validate"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tags"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
	"github.com/hashicorp/terraform-provider-azurerm/utils"
	"github.com/tombuildsstuff/kermit/sdk/network/2022-07-01/network"
)

type CustomIpPrefixModel struct {
	CIDR                        string            `tfschema:"cidr"`
	CommissioningEnabled        bool              `tfschema:"commissioning_enabled"`
	InternetAdvertisingDisabled bool              `tfschema:"internet_advertising_disabled"`
	Location                    string            `tfschema:"location"`
	Name                        string            `tfschema:"name"`
	ParentCustomIPPrefixID      string            `tfschema:"parent_custom_ip_prefix_id"`
	ROAValidityEndDate          string            `tfschema:"roa_validity_end_date"`
	ResourceGroupName           string            `tfschema:"resource_group_name"`
	Tags                        map[string]string `tfschema:"tags"`
	WANValidationSignedMessage  string            `tfschema:"wan_validation_signed_message"`
	Zones                       []string          `tfschema:"zones"`
}

var (
	_ sdk.ResourceWithUpdate = CustomIpPrefixResource{}
)

type CustomIpPrefixResource struct {
	client *network.CustomIPPrefixesClient
}

func (CustomIpPrefixResource) ResourceType() string {
	return "azurerm_custom_ip_prefix"
}

func (CustomIpPrefixResource) ModelObject() interface{} {
	return &CustomIpPrefixModel{}
}

func (CustomIpPrefixResource) IDValidationFunc() pluginsdk.SchemaValidateFunc {
	return validate.CustomIpPrefixID
}
func (r CustomIpPrefixResource) Arguments() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"name": {
			Type:         pluginsdk.TypeString,
			Required:     true,
			ForceNew:     true,
			ValidateFunc: validate.CustomIpPrefixName,
		},

		"location": commonschema.Location(),

		"resource_group_name": commonschema.ResourceGroupName(),

		"cidr": {
			Type:     pluginsdk.TypeString,
			Required: true,
			ForceNew: true,
			ValidateFunc: func(i interface{}, k string) (warnings []string, errors []error) {
				v, ok := i.(string)
				if !ok {
					errors = append(errors, fmt.Errorf("expected type of %s to be string", k))
					return
				}

				if _, _, err := net.ParseCIDR(v); err != nil {
					errors = append(errors, fmt.Errorf("expected %q to be a valid IPv4 or IPv6 network, got %v: %v", k, i, err))
				}

				return
			},
		},

		"parent_custom_ip_prefix_id": {
			Type:         pluginsdk.TypeString,
			Optional:     true,
			ForceNew:     true,
			ValidateFunc: validate.CustomIpPrefixID,
		},

		"roa_validity_end_date": {
			Type:     pluginsdk.TypeString,
			Optional: true,
			ForceNew: true,
			ValidateFunc: func(i interface{}, k string) (warnings []string, errors []error) {
				v, ok := i.(string)
				if !ok {
					errors = append(errors, fmt.Errorf("expected type of %q to be string", k))
					return warnings, errors
				}

				if _, err := time.Parse("2006-01-02", v); err != nil {
					errors = append(errors, fmt.Errorf("expected %q to be a valid date in the format YYYY-MM-DD, got %q: %+v", k, i, err))
				}

				return warnings, errors
			},
		},

		"wan_validation_signed_message": {
			Type:         pluginsdk.TypeString,
			Optional:     true,
			ForceNew:     true,
			ValidateFunc: validation.StringIsNotEmpty,
		},

		"commissioning_enabled": {
			Type:     pluginsdk.TypeBool,
			Optional: true,
			Default:  false,
		},

		"internet_advertising_disabled": {
			Type:     pluginsdk.TypeBool,
			Optional: true,
			Default:  false,
		},

		"tags": commonschema.Tags(),

		"zones": commonschema.ZonesMultipleOptionalForceNew(),
	}
}

func (r CustomIpPrefixResource) Attributes() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{}
}

func (r CustomIpPrefixResource) Create() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 9 * time.Hour,

		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			r.client = metadata.Client.Network.CustomIPPrefixesClient
			subscriptionId := metadata.Client.Account.SubscriptionId

			deadline, ok := ctx.Deadline()
			if !ok {
				return fmt.Errorf("internal-error: context has no deadline")
			}

			var model CustomIpPrefixModel
			if err := metadata.Decode(&model); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}

			id := parse.NewCustomIpPrefixID(subscriptionId, model.ResourceGroupName, model.Name)

			existing, err := r.client.Get(ctx, id.ResourceGroup, id.Name, "")
			if err != nil {
				if !utils.ResponseWasNotFound(existing.Response) {
					return fmt.Errorf("checking for presence of existing %s: %+v", id, err)
				}
			}

			if !utils.ResponseWasNotFound(existing.Response) {
				return metadata.ResourceRequiresImport(r.ResourceType(), id)
			}

			_, cidr, err := net.ParseCIDR(model.CIDR)
			if err != nil {
				return fmt.Errorf("parsing `cidr`: %+v", err)
			}

			results, err := r.client.ListAll(ctx)
			if err != nil {
				return fmt.Errorf("listing existing %s: %+v", id, err)
			}

			// Check for an existing CIDR
			for results.NotDone() {
				for _, prefix := range results.Values() {
					if prefix.CustomIPPrefixPropertiesFormat != nil && prefix.CustomIPPrefixPropertiesFormat.Cidr != nil {
						_, netw, err := net.ParseCIDR(*prefix.CustomIPPrefixPropertiesFormat.Cidr)
						if err != nil {
							// couldn't parse the existing custom prefix, so skip it
							continue
						}
						if cidr == netw {
							return metadata.ResourceRequiresImport(r.ResourceType(), id)
						}
					}
				}

				if err = results.NextWithContext(ctx); err != nil {
					return fmt.Errorf("listing next page of %s: %+v", id, err)
				}
			}

			properties := network.CustomIPPrefix{
				Name:             &model.Name,
				Location:         pointer.To(location.Normalize(model.Location)),
				Tags:             tags.FromTypedObject(model.Tags),
				ExtendedLocation: nil,
				CustomIPPrefixPropertiesFormat: &network.CustomIPPrefixPropertiesFormat{
					Cidr:              &model.CIDR,
					CommissionedState: network.CommissionedStateProvisioning,
				},
			}

			if model.ParentCustomIPPrefixID != "" {
				properties.CustomIPPrefixPropertiesFormat.CustomIPPrefixParent = &network.SubResource{
					ID: &model.ParentCustomIPPrefixID,
				}
			}

			if model.WANValidationSignedMessage != "" {
				properties.CustomIPPrefixPropertiesFormat.SignedMessage = &model.WANValidationSignedMessage
			}

			if model.ROAValidityEndDate != "" {
				roaValidityEndDate, err := time.Parse("2006-01-02", model.ROAValidityEndDate)
				if err != nil {
					return err
				}
				authorizationMessage := fmt.Sprintf("%s|%s|%s", subscriptionId, model.CIDR, roaValidityEndDate.Format("20060102"))
				properties.CustomIPPrefixPropertiesFormat.AuthorizationMessage = &authorizationMessage
			}

			if len(model.Zones) > 0 {
				properties.Zones = &model.Zones
			}

			future, err := r.client.CreateOrUpdate(ctx, id.ResourceGroup, id.Name, properties)
			if err != nil {
				return fmt.Errorf("creating %s: %+v", id, err)
			}

			if err = future.WaitForCompletionRef(ctx, r.client.Client); err != nil {
				return fmt.Errorf("waiting for creation of %s: %+v", id, err)
			}

			stateConf := &pluginsdk.StateChangeConf{
				Pending:    []string{string(network.ProvisioningStateUpdating)},
				Target:     []string{string(network.ProvisioningStateSucceeded)},
				Refresh:    r.provisioningStateRefreshFunc(ctx, id),
				MinTimeout: 2 * time.Minute,
				Timeout:    time.Until(deadline),
			}
			if _, err = stateConf.WaitForStateContext(ctx); err != nil {
				return fmt.Errorf("waiting for provisioning state of %s: %+v", id, err)
			}

			desiredState := network.CommissionedStateProvisioned
			if model.CommissioningEnabled {
				if model.InternetAdvertisingDisabled {
					desiredState = network.CommissionedStateCommissionedNoInternetAdvertise
				} else {
					desiredState = network.CommissionedStateCommissioned
				}
			}

			if _, err = r.updateCommissionedState(ctx, id, desiredState); err != nil {
				return err
			}

			metadata.SetID(id)

			return nil
		},
	}
}

func (r CustomIpPrefixResource) Update() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 17 * time.Hour,

		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			r.client = metadata.Client.Network.CustomIPPrefixesClient

			id, err := parse.CustomIpPrefixID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			metadata.Logger.Info("Decoding state...")
			var state CustomIpPrefixModel
			if err := metadata.Decode(&state); err != nil {
				return err
			}

			desiredState := network.CommissionedStateProvisioned
			if state.CommissioningEnabled {
				if state.InternetAdvertisingDisabled {
					desiredState = network.CommissionedStateCommissionedNoInternetAdvertise
				} else {
					desiredState = network.CommissionedStateCommissioned
				}
			}

			if _, err = r.updateCommissionedState(ctx, *id, desiredState); err != nil {
				return err
			}

			return nil
		},
	}
}

func (r CustomIpPrefixResource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 5 * time.Minute,

		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			r.client = metadata.Client.Network.CustomIPPrefixesClient

			id, err := parse.CustomIpPrefixID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			existing, err := r.client.Get(ctx, id.ResourceGroup, id.Name, "")
			if err != nil {
				if utils.ResponseWasNotFound(existing.Response) {
					return metadata.MarkAsGone(id)
				}
				return fmt.Errorf("retrieving %s: %+v", id, err)
			}

			model := CustomIpPrefixModel{
				Name:              id.Name,
				ResourceGroupName: id.ResourceGroup,
				Location:          location.NormalizeNilable(existing.Location),
				Tags:              tags.ToTypedObject(existing.Tags),
				Zones:             pointer.From(existing.Zones),
			}

			if props := existing.CustomIPPrefixPropertiesFormat; props != nil {
				model.CIDR = pointer.From(props.Cidr)
				model.InternetAdvertisingDisabled = pointer.From(props.NoInternetAdvertise)
				model.WANValidationSignedMessage = pointer.From(props.SignedMessage)

				if parent := props.CustomIPPrefixParent; parent != nil {
					model.ParentCustomIPPrefixID = pointer.From(parent.ID)
				}

				if props.AuthorizationMessage != nil {
					authMessage := strings.Split(*props.AuthorizationMessage, "|")
					if len(authMessage) == 3 {
						if roaValidityEndDate, err := time.Parse("20060102", authMessage[2]); err == nil {
							model.ROAValidityEndDate = roaValidityEndDate.Format("2006-01-02")
						}
					}
				}

				switch props.CommissionedState {
				case network.CommissionedStateCommissioning, network.CommissionedStateCommissioned, network.CommissionedStateCommissionedNoInternetAdvertise:
					model.CommissioningEnabled = true
				default:
					model.CommissioningEnabled = false
				}
			}

			return metadata.Encode(&model)
		},
	}
}

func (r CustomIpPrefixResource) Delete() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 17 * time.Hour,

		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			r.client = metadata.Client.Network.CustomIPPrefixesClient

			id, err := parse.CustomIpPrefixID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			// Must be de-provisioned before deleting
			if _, err = r.updateCommissionedState(ctx, *id, network.CommissionedStateDeprovisioned); err != nil {
				return err
			}

			future, err := r.client.Delete(ctx, id.ResourceGroup, id.Name)
			if err != nil {
				return fmt.Errorf("deleting %s: %+v", id, err)
			}

			if err = future.WaitForCompletionRef(ctx, r.client.Client); err != nil {
				return fmt.Errorf("waiting for deletion of %s: %+v", *id, err)
			}

			return nil
		},
	}
}

type commissionedStates []network.CommissionedState

func (t commissionedStates) contains(i network.CommissionedState) bool {
	for _, s := range t {
		if i == s {
			return true
		}
	}
	return false
}

func (t commissionedStates) strings() (out []string) {
	for _, s := range t {
		out = append(out, string(s))
	}
	return
}

func (r CustomIpPrefixResource) updateCommissionedState(ctx context.Context, id parse.CustomIpPrefixId, desiredState network.CommissionedState) (*network.CommissionedState, error) {
	existing, err := r.client.Get(ctx, id.ResourceGroup, id.Name, "")
	if err != nil {
		return nil, fmt.Errorf("retrieving existing %s: %+v", id, err)
	}
	if existing.CustomIPPrefixPropertiesFormat == nil {
		return nil, fmt.Errorf("retrieving existing %s: `properties` was nil", id)
	}

	initialState := existing.CustomIPPrefixPropertiesFormat.CommissionedState

	// stateTree is a map of desired state, to a map of current state, to the list of transition states needed to get there
	stateTree := map[network.CommissionedState]map[network.CommissionedState][]network.CommissionedState{
		network.CommissionedStateDeprovisioned: {
			network.CommissionedStateProvisioned:                     {network.CommissionedStateDeprovisioning},
			network.CommissionedStateCommissioned:                    {network.CommissionedStateDecommissioning, network.CommissionedStateDeprovisioning},
			network.CommissionedStateCommissionedNoInternetAdvertise: {network.CommissionedStateDecommissioning, network.CommissionedStateDeprovisioning},
		},
		network.CommissionedStateProvisioned: {
			network.CommissionedStateDeprovisioned:                   {network.CommissionedStateProvisioning},
			network.CommissionedStateCommissioned:                    {network.CommissionedStateDecommissioning},
			network.CommissionedStateCommissionedNoInternetAdvertise: {network.CommissionedStateDecommissioning},
		},
		network.CommissionedStateCommissioned: {
			network.CommissionedStateDeprovisioned:                   {network.CommissionedStateProvisioning, network.CommissionedStateCommissioning},
			network.CommissionedStateProvisioned:                     {network.CommissionedStateCommissioning},
			network.CommissionedStateCommissionedNoInternetAdvertise: {network.CommissionedStateDecommissioning, network.CommissionedStateCommissioning},
		},
		network.CommissionedStateCommissionedNoInternetAdvertise: {
			network.CommissionedStateDeprovisioned: {network.CommissionedStateProvisioning, network.CommissionedStateCommissioning},
			network.CommissionedStateProvisioned:   {network.CommissionedStateCommissioning},
			network.CommissionedStateCommissioned:  {network.CommissionedStateDecommissioning, network.CommissionedStateCommissioning},
		},
	}

	// transitioningStatesFor returns the known transitioning states for the desired goal state
	transitioningStatesFor := func(finalState network.CommissionedState) (out commissionedStates) {
		switch finalState {
		case network.CommissionedStateProvisioned:
			out = commissionedStates{network.CommissionedStateProvisioning, network.CommissionedStateDecommissioning}
		case network.CommissionedStateDeprovisioned:
			out = commissionedStates{network.CommissionedStateDeprovisioning}
		case network.CommissionedStateCommissioned:
			out = commissionedStates{network.CommissionedStateCommissioning}
		}
		return
	}

	// finalStatesFor returns the known final states for the current transitioning state
	finalStatesFor := func(transitioningState network.CommissionedState) (out commissionedStates) {
		switch transitioningState {
		case network.CommissionedStateProvisioning:
			out = commissionedStates{network.CommissionedStateProvisioned}
		case network.CommissionedStateDeprovisioning:
			out = commissionedStates{network.CommissionedStateDeprovisioned}
		case network.CommissionedStateCommissioning:
			out = commissionedStates{network.CommissionedStateCommissioned, network.CommissionedStateCommissionedNoInternetAdvertise}
		case network.CommissionedStateDecommissioning:
			out = commissionedStates{network.CommissionedStateProvisioned}
		}
		return
	}

	// shouldNotAdvertise determines whether to set the noInternetAdvertise flag, which can only be set at the point of transitioning to `Commissioning`
	shouldNotAdvertise := func(steppingState network.CommissionedState) *bool {
		if steppingState == network.CommissionedStateCommissioning {
			switch desiredState {
			case network.CommissionedStateCommissioned:
				return pointer.To(false)
			case network.CommissionedStateCommissionedNoInternetAdvertise:
				return pointer.To(true)
			}
		}
		return nil
	}

	if plan, ok := stateTree[desiredState]; ok {
		lastKnownState := pointer.To(initialState)

		if transitioningStatesFor(desiredState).contains(initialState) {
			if lastKnownState, err = r.waitForCommissionedState(ctx, id, transitioningStatesFor(desiredState), commissionedStates{desiredState}); err != nil {
				return lastKnownState, err
			}
		}

		if initialState == desiredState {
			return lastKnownState, nil
		}

		for startingState, path := range plan {
			if initialState == startingState || transitioningStatesFor(startingState).contains(initialState) {
				if lastKnownState, err = r.waitForCommissionedState(ctx, id, transitioningStatesFor(startingState), commissionedStates{startingState}); err != nil {
					return lastKnownState, err
				}

				retries := 0
				for i := 0; i < len(path); i++ {
					steppingState := path[i]

					if err = r.setCommissionedState(ctx, id, steppingState, shouldNotAdvertise(steppingState)); err != nil {
						return lastKnownState, err
					}

					latestState, err := r.waitForCommissionedState(ctx, id, commissionedStates{steppingState}, finalStatesFor(steppingState))
					if err != nil {
						// Known issue where the previous commissioningState was reported prematurely by the API, so we reattempt up to 2 times
						if lastKnownState != nil && latestState != nil && *latestState == *lastKnownState && retries < 2 {
							retries++
							i--
							continue
						}

						return lastKnownState, err
					}

					lastKnownState = latestState
				}

				return r.waitForCommissionedState(ctx, id, transitioningStatesFor(desiredState), commissionedStates{desiredState})
			}
		}
	} else {
		return nil, fmt.Errorf("unsupported state %q", desiredState)
	}

	return nil, nil
}

func (r CustomIpPrefixResource) setCommissionedState(ctx context.Context, id parse.CustomIpPrefixId, desiredState network.CommissionedState, noInternetAdvertise *bool) error {
	existing, err := r.client.Get(ctx, id.ResourceGroup, id.Name, "")
	if err != nil {
		return fmt.Errorf("retrieving existing %s: %+v", id, err)
	}
	if existing.CustomIPPrefixPropertiesFormat == nil {
		return fmt.Errorf("retrieving existing %s: `properties` was nil", id)
	}

	existing.CustomIPPrefixPropertiesFormat.CommissionedState = desiredState
	existing.CustomIPPrefixPropertiesFormat.NoInternetAdvertise = noInternetAdvertise

	log.Printf("[DEBUG] Updating the CommissionedState field to %q for %s..", string(desiredState), id)
	future, err := r.client.CreateOrUpdate(ctx, id.ResourceGroup, id.Name, existing)
	if err != nil {
		return fmt.Errorf("updating CommissionedState to %q for %s: %+v", string(desiredState), id, err)
	}

	if err := future.WaitForCompletionRef(ctx, r.client.Client); err != nil {
		return fmt.Errorf("waiting for the update of CommissionedState to %q for %s: %+v", string(desiredState), id, err)
	}

	return nil
}

func (r CustomIpPrefixResource) waitForCommissionedState(ctx context.Context, id parse.CustomIpPrefixId, pendingStates, targetStates commissionedStates) (*network.CommissionedState, error) {
	log.Printf("[DEBUG] Polling for the CommissionedState field for %s..", id)
	timeout, _ := ctx.Deadline()

	stateConf := &pluginsdk.StateChangeConf{
		Pending:      pendingStates.strings(),
		Target:       targetStates.strings(),
		Refresh:      r.commissionedStateRefreshFunc(ctx, id),
		PollInterval: 5 * time.Minute,
		Timeout:      time.Until(timeout),

		// Provisioned is known to flip-flop
		ContinuousTargetOccurence: 3,
	}

	result, err := stateConf.WaitForStateContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("waiting for commissioned state of %s: %+v", id, err)
	}

	prefix, ok := result.(network.CustomIPPrefix)
	if !ok {
		return nil, fmt.Errorf("retrieving %s: response was not a valid Custom IP Prefix", id)
	}

	if prefix.CustomIPPrefixPropertiesFormat == nil {
		return nil, fmt.Errorf("retrieving %s: `properties` was nil", id)
	}

	return &prefix.CustomIPPrefixPropertiesFormat.CommissionedState, nil
}

func (r CustomIpPrefixResource) commissionedStateRefreshFunc(ctx context.Context, id parse.CustomIpPrefixId) pluginsdk.StateRefreshFunc {
	return func() (interface{}, string, error) {
		res, err := r.client.Get(ctx, id.ResourceGroup, id.Name, "")
		if err != nil {
			return nil, "", fmt.Errorf("polling for %s: %+v", id.String(), err)
		}

		return res, string(res.CommissionedState), nil
	}
}

func (r CustomIpPrefixResource) provisioningStateRefreshFunc(ctx context.Context, id parse.CustomIpPrefixId) pluginsdk.StateRefreshFunc {
	return func() (interface{}, string, error) {
		res, err := r.client.Get(ctx, id.ResourceGroup, id.Name, "")
		if err != nil {
			return nil, "", fmt.Errorf("polling for %s: %+v", id.String(), err)
		}

		return res, string(res.ProvisioningState), nil
	}
}

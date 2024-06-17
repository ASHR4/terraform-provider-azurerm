package servicelinker

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/go-azure-sdk/sdk/client"
	"github.com/hashicorp/go-azure-sdk/sdk/odata"
)

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License. See NOTICE.txt in the project root for license information.

type ConnectorListOperationResponse struct {
	HttpResponse *http.Response
	OData        *odata.OData
	Model        *[]LinkerResource
}

type ConnectorListCompleteResult struct {
	LatestHttpResponse *http.Response
	Items              []LinkerResource
}

// ConnectorList ...
func (c ServiceLinkerClient) ConnectorList(ctx context.Context, id LocationId) (result ConnectorListOperationResponse, err error) {
	opts := client.RequestOptions{
		ContentType: "application/json; charset=utf-8",
		ExpectedStatusCodes: []int{
			http.StatusOK,
		},
		HttpMethod: http.MethodGet,
		Path:       fmt.Sprintf("%s/connectors", id.ID()),
	}

	req, err := c.Client.NewRequest(ctx, opts)
	if err != nil {
		return
	}

	var resp *client.Response
	resp, err = req.ExecutePaged(ctx)
	if resp != nil {
		result.OData = resp.OData
		result.HttpResponse = resp.Response
	}
	if err != nil {
		return
	}

	var values struct {
		Values *[]LinkerResource `json:"value"`
	}
	if err = resp.Unmarshal(&values); err != nil {
		return
	}

	result.Model = values.Values

	return
}

// ConnectorListComplete retrieves all the results into a single object
func (c ServiceLinkerClient) ConnectorListComplete(ctx context.Context, id LocationId) (ConnectorListCompleteResult, error) {
	return c.ConnectorListCompleteMatchingPredicate(ctx, id, LinkerResourceOperationPredicate{})
}

// ConnectorListCompleteMatchingPredicate retrieves all the results and then applies the predicate
func (c ServiceLinkerClient) ConnectorListCompleteMatchingPredicate(ctx context.Context, id LocationId, predicate LinkerResourceOperationPredicate) (result ConnectorListCompleteResult, err error) {
	items := make([]LinkerResource, 0)

	resp, err := c.ConnectorList(ctx, id)
	if err != nil {
		result.LatestHttpResponse = resp.HttpResponse
		err = fmt.Errorf("loading results: %+v", err)
		return
	}
	if resp.Model != nil {
		for _, v := range *resp.Model {
			if predicate.Matches(v) {
				items = append(items, v)
			}
		}
	}

	result = ConnectorListCompleteResult{
		LatestHttpResponse: resp.HttpResponse,
		Items:              items,
	}
	return
}

package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func Test_client_fetchNetworkID(t *testing.T) {
	tests := map[string]struct {
		Responder httpmock.Responder
		Result    string
		Error     bool
	}{
		"Request error": {
			Responder: httpmock.NewErrorResponder(assert.AnError),
			Result:    "",
			Error:     true,
		},
		"Successfully fetched": {
			Responder: httpmock.NewStringResponder(http.StatusOK, `{"result":{"node_info": {"network": "test1"}}}`),
			Result:    "test1",
			Error:     false,
		},
	}

	for tname, tcase := range tests {
		t.Run(tname, func(t *testing.T) {
			t.Parallel()

			mt := httpmock.NewMockTransport()
			c := client{
				http:       &http.Client{Transport: mt},
				baseURL:    "http://localhost",
				maxRetries: 1,
			}
			mt.RegisterResponder(http.MethodGet, "http://localhost/status", tcase.Responder)

			res, err := c.fetchNetworkID(context.Background())
			if tcase.Error {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tcase.Result, res)
		})
	}
}

func Test_client_fetchBlock(t *testing.T) {
	tests := map[string]struct {
		Responder httpmock.Responder
		Result    block
		Error     bool
	}{
		"Request error": {
			Responder: httpmock.NewErrorResponder(assert.AnError),
			Result:    block{},
			Error:     true,
		},
		"Successfully fetched": {
			Responder: httpmock.NewStringResponder(http.StatusOK, `{"result":{"block": {"data":{"txs":[{}, {}, {}]}}}}`),
			Result: block{
				NetworkID: "test",
				Height:    123,
				NumTxs:    3,
			},
			Error: false,
		},
	}

	for tname, tcase := range tests {
		t.Run(tname, func(t *testing.T) {
			t.Parallel()

			mt := httpmock.NewMockTransport()
			c := client{
				http:       &http.Client{Transport: mt},
				baseURL:    "http://localhost",
				maxRetries: 1,
			}
			mt.RegisterResponder(http.MethodGet, "http://localhost/block", tcase.Responder)

			res, err := c.fetchBlock(context.Background(), "test", 123)
			if tcase.Error {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tcase.Result, res)
		})
	}
}

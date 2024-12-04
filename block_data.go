package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
)

const (
	retryAfter       = time.Second
	cooldownDuration = time.Second * 5
	maxRetries       = 100
)

var (
	errCooldown = errors.New("429 was received, cooling down...")
)

// blockResponse contains the essential data of the /block response.
// More info: https://www.quicknode.com/docs/cosmos/block
type blockResponse struct {
	Result struct {
		Block struct {
			Data struct {
				Txs []interface{} `json:"txs"`
			} `json:"data"`
		} `json:"block"`
	} `json:"result"`
}

// statusResponse contains the essential data of the /status response.
// More info: https://www.quicknode.com/docs/cosmos/status
type statusResponse struct {
	Result struct {
		NodeInfo struct {
			Network string `json:"network"`
		} `json:"node_info"`
	} `json:"result"`
}

// block contains a single block's metadata.
type block struct {
	Height    int64  `json:"height"`
	NumTxs    int    `json:"num_txs"`
	NetworkID string `json:"network_id"`
}

// client is a structure that handles communication with a chain's RPC
// node.
type client struct {
	http       *http.Client
	baseURL    string
	maxRetries uint64

	cooldownUntilMu sync.RWMutex
	cooldownUntil   time.Time
}

// newClient creates a new instance of client.
func newClient(baseURL string, maxRetries uint64) *client {
	return &client{
		http: &http.Client{
			Timeout: time.Minute,
		},
		baseURL:    baseURL,
		maxRetries: maxRetries,
	}
}

// fetchNetworkID retrieves the network ID of the chain whose base URL
// is used by the client.
func (c *client) fetchNetworkID(ctx context.Context) (string, error) {
	var statusResp statusResponse
	err := c.fetchWithRetry(ctx, fmt.Sprintf("%s/status", c.baseURL), &statusResp)
	if err != nil {
		return "", err
	}

	return statusResp.Result.NodeInfo.Network, nil
}

// fetchBlock retrieves the target block's metadata.
func (c *client) fetchBlock(ctx context.Context, networkID string, height int64) (block, error) {
	var resp blockResponse
	err := c.fetchWithRetry(ctx, fmt.Sprintf("%s/block?height=%d", c.baseURL, height), &resp)
	if err != nil {
		return block{}, err
	}

	return block{
		Height:    height,
		NumTxs:    len(resp.Result.Block.Data.Txs),
		NetworkID: networkID,
	}, nil
}

// fetchWithRetry retrieves the target resource and applies a repeated retry
// strategy if needed.
func (c *client) fetchWithRetry(ctx context.Context, targetURL string, target interface{}) error {
	req, err := http.NewRequest(http.MethodGet, targetURL, http.NoBody)
	if err != nil {
		return err
	}

	return backoff.RetryNotify(func() error {
		c.cooldownUntilMu.RLock()
		if time.Now().Before(c.cooldownUntil) {
			select {
			case <-time.After(c.cooldownUntil.Sub(time.Now())):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		c.cooldownUntilMu.RUnlock()

		resp, err := c.http.Do(req.WithContext(ctx))
		if err != nil {
			return err
		}

		defer resp.Body.Close()

		if resp.StatusCode == 429 {
			c.cooldownUntilMu.Lock()
			c.cooldownUntil = time.Now().Add(cooldownDuration)
			c.cooldownUntilMu.Unlock()

			return errCooldown
		}

		return json.NewDecoder(resp.Body).Decode(target)
	}, backoff.WithContext(
		backoff.WithMaxRetries(
			backoff.NewConstantBackOff(retryAfter),
			c.maxRetries,
		),
		ctx,
	), func(err error, d time.Duration) {
		if !errors.Is(err, errCooldown) {
			log.Printf("Retrying block fetch request in %s (%s)\n", d, err)
		}
	})
}

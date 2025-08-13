package rethexec

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"
)

type rpcClient struct {
	endpoints      []string
	timeout        time.Duration
	jwtSecretBytes []byte
	curIdx         atomic.Int32
	httpClient     *http.Client
}

func newRPCClient(urls []string, timeout time.Duration, jwtSecret []byte) *rpcClient {
	if len(urls) == 0 {
		urls = []string{"http://localhost:8547"}
	}
	return &rpcClient{
		endpoints:      urls,
		timeout:        timeout,
		jwtSecretBytes: jwtSecret,
		httpClient:     &http.Client{Timeout: timeout},
	}
}

type rpcReq struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      uint64      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type rpcResp struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint64          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (c *rpcClient) nextEndpoint() string {
	idx := int(c.curIdx.Add(1)) % len(c.endpoints)
	return c.endpoints[idx]
}

func (c *rpcClient) do(ctx context.Context, method string, params interface{}, out interface{}) error {
	reqBody := rpcReq{
		JSONRPC: "2.0",
		ID:      uint64(time.Now().UnixNano()),
		Method:  method,
		Params:  params,
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := c.nextEndpoint()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	if len(c.jwtSecretBytes) > 0 {
		token, err := jwtForNow(c.jwtSecretBytes)
		if err != nil {
			return fmt.Errorf("jwt signing failed: %w", err)
		}
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var r rpcResp
	if err := json.Unmarshal(respData, &r); err != nil {
		return err
	}
	if r.Error != nil {
		return fmt.Errorf("rpc error %d: %s", r.Error.Code, r.Error.Message)
	}
	if out != nil {
		return json.Unmarshal(r.Result, out)
	}
	return nil
}

func jwtForNow(secret []byte) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	now := time.Now().Unix()
	payload := fmt.Sprintf(`{"iat":%d}`, now)
	payloadEnc := base64.RawURLEncoding.EncodeToString([]byte(payload))
	toSign := header + "." + payloadEnc
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(toSign))
	sig := mac.Sum(nil)
	sigEnc := base64.RawURLEncoding.EncodeToString(sig)
	return toSign + "." + sigEnc, nil
}

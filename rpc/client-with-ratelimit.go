package rpc

import (
	"io"

	"github.com/valyala/fasthttp"

	"github.com/desperatee/solana-go/rpc/jsonrpc"
	"go.uber.org/ratelimit"
)

var _ JSONRPCClient = &clientWithRateLimiting{}

type clientWithRateLimiting struct {
	rpcClient   jsonrpc.RPCClient
	rateLimiter ratelimit.Limiter
}

// NewWithRateLimit creates a new rate-limitted Solana RPC client.
func NewWithRateLimit(
	rpcEndpoint string,
	rps int, // requests per second
) (JSONRPCClient, error) {
	client, err := newHTTP(rpcEndpoint)
	if err != nil {
		return nil, nil
	}
	opts := &jsonrpc.RPCClientOpts{
		HTTPClient: client,
	}

	rpcClient := jsonrpc.NewClientWithOpts(rpcEndpoint, opts)

	return &clientWithRateLimiting{
		rpcClient:   rpcClient,
		rateLimiter: ratelimit.New(rps),
	}, nil
}

func (wr *clientWithRateLimiting) CallForInto(out interface{}, method string, params []interface{}) error {
	wr.rateLimiter.Take()
	return wr.rpcClient.CallForInto(&out, method, params)
}

func (wr *clientWithRateLimiting) CallWithCallback(
	method string,
	params []interface{},
	callback func(*fasthttp.Request, *fasthttp.Response) error,
) error {
	wr.rateLimiter.Take()
	return wr.rpcClient.CallWithCallback(method, params, callback)
}

func (wr *clientWithRateLimiting) CallBatch(
	requests jsonrpc.RPCRequests,
) (jsonrpc.RPCResponses, error) {
	wr.rateLimiter.Take()
	return wr.rpcClient.CallBatch(requests)
}

// Close closes clientWithRateLimiting.
func (cl *clientWithRateLimiting) Close() error {
	if c, ok := cl.rpcClient.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

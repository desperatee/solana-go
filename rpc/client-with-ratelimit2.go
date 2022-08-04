package rpc

import (
	"github.com/desperatee/solana-go/rpc/jsonrpc"
	"github.com/valyala/fasthttp"
	"golang.org/x/time/rate"
	"io"
)

var _ JSONRPCClient = &clientWithLimiter{}

type clientWithLimiter struct {
	rpcClient jsonrpc.RPCClient
	limiter   *rate.Limiter
}

// NewWithLimiter creates a new rate-limitted Solana RPC client.
// Example: NewWithLimiter(URL, rate.Every(time.Second), 1)
func NewWithLimiter(
	rpcEndpoint string,
	every rate.Limit, // time frame
	b int, // number of requests per time frame
) (JSONRPCClient, error) {
	client, err := newHTTP(rpcEndpoint)
	if err != nil {
		return nil, nil
	}
	opts := &jsonrpc.RPCClientOpts{
		HTTPClient: client,
	}

	rpcClient := jsonrpc.NewClientWithOpts(rpcEndpoint, opts)
	rater := rate.NewLimiter(every, b)

	return &clientWithLimiter{
		rpcClient: rpcClient,
		limiter:   rater,
	}, nil
}

func (wr *clientWithLimiter) CallForInto(out interface{}, method string, params []interface{}) error {
	return wr.rpcClient.CallForInto(&out, method, params)
}

func (wr *clientWithLimiter) CallWithCallback(
	method string,
	params []interface{},
	callback func(*fasthttp.Request, *fasthttp.Response) error,
) error {
	return wr.rpcClient.CallWithCallback(method, params, callback)
}

// Close closes clientWithLimiter.
func (cl *clientWithLimiter) Close() error {
	if c, ok := cl.rpcClient.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

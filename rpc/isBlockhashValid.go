package rpc

import (
	"github.com/desperatee/solana-go"
)

// Returns whether a blockhash is still valid or not
//
// **NEW: This method is only available in solana-core v1.9 or newer. Please use
// `getFeeCalculatorForBlockhash` for solana-core v1.8**
func (cl *Client) IsBlockhashValid(
	// Blockhash to be queried. Required.
	blockHash solana.Hash,

	// Commitment requirement. Optional.
	commitment CommitmentType,
) (out *IsValidBlockhashResult, err error) {
	params := []interface{}{blockHash}
	if commitment != "" {
		params = append(params, M{"commitment": string(commitment)})
	}

	err = cl.rpcClient.CallForInto(&out, "isBlockhashValid", params)
	return
}

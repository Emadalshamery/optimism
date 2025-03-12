package contracts

import "github.com/ethereum-optimism/optimism/op-service/sources/batching"

func newCapturingCall(call batching.Call, callback func(result *batching.CallResult) error) batching.Call {
	return &resultCapturingCall{
		call:     call,
		callback: callback,
	}
}

type resultCapturingCall struct {
	call     batching.Call
	callback func(result *batching.CallResult) error
}

func (c *resultCapturingCall) ToBatchElemCreator() (batching.BatchElementCreator, error) {
	return c.call.ToBatchElemCreator()
}

func (c *resultCapturingCall) HandleResult(result interface{}) (*batching.CallResult, error) {
	callResult, err := c.call.HandleResult(result)
	if err != nil {
		return nil, err
	}
	if err := c.callback(callResult); err != nil {
		return nil, err
	}
	return callResult, nil
}

var _ batching.Call = (*resultCapturingCall)(nil)

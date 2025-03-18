package txplan

import (
	"context"
	"crypto/ecdsa"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

type Wallet interface {
	PrivateKey() *ecdsa.PrivateKey
	Client() *sources.EthClient
}

type wallet struct {
	priv   *ecdsa.PrivateKey
	client *sources.EthClient
	ctx    context.Context
}

func newWallet(ctx context.Context, rpcURL, privHex string, clCfg *sources.EthClientConfig, log log.Logger) (*wallet, error) {
	privRaw, err := hexutil.Decode(privHex)
	if err != nil {
		return nil, err
	}
	priv, err := crypto.ToECDSA(privRaw)
	if err != nil {
		return nil, err
	}
	if clCfg == nil {
		clCfg = &sources.EthClientConfig{
			MaxRequestsPerBatch:   10,
			MaxConcurrentRequests: 10,
			ReceiptsCacheSize:     10,
			TransactionsCacheSize: 10,
			HeadersCacheSize:      10,
			PayloadsCacheSize:     10,
			BlockRefsCacheSize:    10,
			TrustRPC:              false,
			MustBePostMerge:       true,
			RPCProviderKind:       sources.RPCKindStandard,
			MethodResetDuration:   time.Minute,
		}
	}
	rpcClient, err := rpc.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, err
	}
	cl, err := sources.NewEthClient(client.NewBaseRPCClient(rpcClient), log, nil, clCfg)
	if err != nil {
		return nil, err
	}
	return &wallet{
		client: cl,
		priv:   priv,
		ctx:    ctx,
	}, nil
}

func (w *wallet) PrivateKey() *ecdsa.PrivateKey {
	return w.priv
}

func (w *wallet) Client() *sources.EthClient {
	return w.client
}

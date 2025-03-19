package system

import (
	"context"
	"crypto/ecdsa"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/retry"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

var (
	_ WalletV2 = (*walletV2)(nil)
)

type walletV2 struct {
	priv   *ecdsa.PrivateKey
	client *sources.EthClient
	ctx    context.Context
}

func NewWalletV2FromWalletAndChain(ctx context.Context, wallet Wallet, chain LowLevelChain) (WalletV2, error) {
	client, err := chain.Client()
	if err != nil {
		return nil, err
	}
	return &walletV2{
		priv:   wallet.PrivateKey(),
		client: client,
		ctx:    ctx,
	}, nil
}

func NewWalletV2(ctx context.Context, rpcURL, privHex string, clCfg *sources.EthClientConfig, log log.Logger) (*walletV2, error) {
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
	return &walletV2{
		client: cl,
		priv:   priv,
		ctx:    ctx,
	}, nil
}

func (w *walletV2) PrivateKey() *ecdsa.PrivateKey {
	return w.priv
}

func (w *walletV2) Client() *sources.EthClient {
	return w.client
}

func (w *walletV2) Ctx() context.Context {
	return w.ctx
}

func DefaultTxSubmitOptions(w WalletV2) txplan.Option {
	return txplan.CombineOptions(
		txplan.WithPrivateKey(w.PrivateKey()),
		txplan.WithChainID(w.Client()),
		txplan.WithAgainstLatestBlock(w.Client()),
		txplan.WithPendingNonce(w.Client()),
		txplan.WithEstimator(w.Client(), false),
		txplan.WithTransactionSubmitter(w.Client()),
	)
}

func DefaultTxInclusionOptions(w WalletV2) txplan.Option {
	return txplan.CombineOptions(
		txplan.WithRetryInclusion(w.Client(), 10, retry.Exponential()),
		txplan.WithBlockInclusionInfo(w.Client()),
	)
}

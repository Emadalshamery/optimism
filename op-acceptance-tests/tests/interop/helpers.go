package interop

import (
	"context"
	"math/big"
	"math/rand"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/bindings"
	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/constants"
	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/systest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/testlib/validators"
	sdktypes "github.com/ethereum-optimism/optimism/devnet-sdk/types"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/stretchr/testify/require"
)

func DefaultOpts(w system.WalletV2) txplan.Option {
	return txplan.CombineOptions(
		system.DefaultTxSubmitOptions(w),
		system.DefaultTxInclusionOptions(w),
	)
}

func GetWalletV2AndOpts(ctx context.Context, walletGetter validators.WalletGetter, chain system.LowLevelL2Chain) (system.WalletV2, txplan.Option, error) {
	wallet, err := system.NewWalletV2FromWalletAndChain(ctx, walletGetter(ctx), chain)
	if err != nil {
		return nil, nil, err
	}
	opts := DefaultOpts(wallet)
	return wallet, opts, nil
}

func DefaultSetup(t systest.T,
	lowLevelSystemGetter validators.LowLevelSystemGetter,
	l2ChainNums int,
	chainIdxs []uint64,
	walletGetters []validators.WalletGetter,
) (context.Context, *rand.Rand, log.Logger, []system.LowLevelChain, []system.WalletV2, []txplan.Option) {
	ctx := t.Context()
	rng := rand.New(rand.NewSource(1234))
	logger := testlog.Logger(t, log.LevelInfo)

	llsys := lowLevelSystemGetter(ctx)
	chains := make([]system.LowLevelChain, l2ChainNums)
	wallets := make([]system.WalletV2, 0)
	opts := make([]txplan.Option, 0)
	for idx := range l2ChainNums {
		chain := llsys.L2s()[chainIdxs[idx]]
		chains = append(chains, chain)
		wallet, opt, err := GetWalletV2AndOpts(ctx, walletGetters[idx], chain)
		require.NoError(t, err)
		wallets = append(wallets, wallet)
		opts = append(opts, opt)
	}
	return ctx, rng, logger, chains, wallets, opts
}

func DeployEventLogger(ctx context.Context, wallet system.WalletV2, logger log.Logger) (common.Address, error) {
	optsFunc := func(w system.WalletV2) txplan.Option {
		opts := txplan.CombineOptions(
			system.DefaultTxSubmitOptions(w),
			system.DefaultTxInclusionOptions(w),
		)
		return opts
	}
	opts := optsFunc(wallet)
	logger.Info("Deploying EventLogger")
	deployCalldata := common.FromHex(bindings.EventloggerBin)
	deployTx := txplan.NewPlannedTx(opts, txplan.WithData(deployCalldata))

	res, err := deployTx.Included.Eval(ctx)
	if err != nil {
		return common.Address{}, err
	}
	eventLoggerAddress := res.ContractAddress
	logger.Info("Deployed EventLogger", "address", eventLoggerAddress)
	return eventLoggerAddress, nil
}

func RandomTopicAndData(rng *rand.Rand, cnt, len int) ([][32]byte, []byte) {
	topics := [][32]byte{}
	for range cnt {
		var topic [32]byte
		copy(topic[:], testutils.RandomData(rng, 32))
		topics = append(topics, topic)
	}
	data := testutils.RandomData(rng, len)
	return topics, data
}

func RandomInitTrigger(rng *rand.Rand, eventLoggerAddress common.Address, cnt, len int) *system.InitTrigger {
	topics, data := RandomTopicAndData(rng, cnt, len)
	return &system.InitTrigger{
		Emitter:    eventLoggerAddress,
		Topics:     topics,
		OpaqueData: data,
	}
}

func SetupDefaultInteropSystemTest(l2ChainNums int) ([]uint64, []validators.WalletGetter, []systest.PreconditionValidator, validators.LowLevelSystemGetter) {
	chainIdxs := make([]uint64, l2ChainNums)
	walletGetters := make([]validators.WalletGetter, l2ChainNums)
	totalValidators := make([]systest.PreconditionValidator, 0)
	for i := range l2ChainNums {
		chainIdxs[i] = uint64(i)
		walletGetter, fundsValidator := validators.AcquireL2WalletWithFunds(
			chainIdxs[i], sdktypes.NewBalance(big.NewInt(1*constants.ETH)),
		)
		walletGetters[i] = walletGetter
		totalValidators = append(totalValidators, fundsValidator)
	}
	lowLevelSystemGetter, lowLevelSystemValidator := validators.AcquireLowLevelSystem()
	totalValidators = append(totalValidators, lowLevelSystemValidator)
	return chainIdxs, walletGetters, totalValidators, lowLevelSystemGetter
}

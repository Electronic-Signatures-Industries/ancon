package ancon

import (
	"bytes"
	"errors"
	"io"
	"math/big"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/fluent"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/basicnode"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/server"
	sdkconfig "github.com/cosmos/cosmos-sdk/server/config"
	sdk "github.com/cosmos/cosmos-sdk/types"

	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/tendermint/tendermint/libs/log"
	tmtypes "github.com/tendermint/tendermint/types"

	"github.com/tharsis/ethermint/ethereum/rpc/backend"
	rpctypes "github.com/tharsis/ethermint/ethereum/rpc/types"
	"github.com/tharsis/ethermint/server/config"
)

// API is the ancon prefixed set of APIs
type API struct {
	ctx       *server.Context
	logger    log.Logger
	clientCtx client.Context
	backend   backend.Backend
}

// NewAnconAPI creates an instance of the Miner API.
func NewAnconAPI(
	ctx *server.Context,
	clientCtx client.Context,
	backend backend.Backend,
) *API {
	return &API{
		ctx:       ctx,
		clientCtx: clientCtx,
		logger:    ctx.Logger.With("api", "ancon"),
		backend:   backend,
	}
}

// Store
func (api *API) Store(
	ctx sdk.Context,
	file interface{},
) ipld.Link {
	lsys := cidlink.DefaultLinkSystem()

	//   you just need a function that conforms to the ipld.BlockWriteOpener interface.
	lsys.StorageWriteOpener = func(lnkCtx ipld.LinkContext) (io.Writer, ipld.BlockWriteCommitter, error) {
		// change prefix
		buf := bytes.Buffer{}
		return &buf, func(lnk ipld.Link) error {
			// store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefix(types.FileIPLDKey))
			// f := types.File{
			// 	Data:               buf.Bytes(),
			// 	Creator:            file.Creator,
			// 	ContentType:        file.ContentType,
			// 	Id:                 file.Id,
			// 	Cid:                lnk.String(),
			// 	StorageNetworkType: "xdv",
			// }
			// value := k.cdc.MustMarshalBinaryBare(&f)
			// store.Set(GetFileCIDBytes(lnk.String()), value)
			return nil
		}, nil
	}

	// Add Document
	// Basic Node
	n := fluent.MustBuildMap(basicnode.Prototype.Map, 1, func(na fluent.MapAssembler) {
		// na.AssembleEntry("index").AssignBytes(file.Data)
	})

	lp := cidlink.LinkPrototype{cid.Prefix{
		Version:  1,
		Codec:    0x71, // dag-cbor
		MhType:   0x13, // sha2-512
		MhLength: 64,   // sha2-512 hash has a 64-byte sum.
	}}

	lnk, err := lsys.Store(
		ipld.LinkContext{}, // The zero value is fine.  Configure it it you want cancellability or other features.
		lp,                 // The LinkPrototype says what codec and hashing to use.
		n,                  // And here's our data.
	)
	if err != nil {
		panic(err)
	}
	return lnk
}

func (api *API) Load(ctx sdk.Context, cid cid.Cid) ipld.Node {
	// Let's say we want to load this link (it's the same one we just created in the example above).
	// cid, _ := cid.Decode("bafyrgqhai26anf3i7pips7q22coa4sz2fr4gk4q4sqdtymvvjyginfzaqewveaeqdh524nsktaq43j65v22xxrybrtertmcfxufdam3da3hbk")
	lnk := cidlink.Link{Cid: cid}

	// Let's get a LinkSystem.  We're going to be working with CID links,
	//  so let's get the default LinkSystem that's ready to work with those.
	// (This is the same as we did in ExampleStoringLink.)
	lsys := cidlink.DefaultLinkSystem()

	// We need somewhere to go looking for any of the data we might want to load!
	//  We'll use an in-memory store for this.  (It's a package scoped variable.)
	//   (This particular memory store was filled with the data we'll load earlier, during ExampleStoringLink.)
	//  You can use any kind of storage system here;
	//   you just need a function that conforms to the ipld.BlockReadOpener interface.
	lsys.StorageReadOpener = func(lnkCtx ipld.LinkContext, lnk ipld.Link) (io.Reader, error) {
		// file := k.GetFile(ctx, lnk.String())
		// return bytes.NewReader(file.Data), NewBuilder
		return nil, nil
	}

	// We'll need to decide what in-memory implementation of ipld.Node we want to use.
	//  Here, we'll use the "basicnode" implementation.  This is a good getting-started choice.
	//   But you could also use other implementations, or even a code-generated type with special features!
	np := basicnode.Prototype.Any

	// Before we use the LinkService, NOTE:
	//  There's a side-effecting import at the top of the file.  It's for the dag-cbor codec.
	//  See the comments in ExampleStoringLink for more discussion of this and why it's important.

	lsys.TrustedStorage = true

	// Choose all the parts.
	decoder, err := lsys.DecoderChooser(lnk)
	if err != nil {
		ctx.Logger().Error("could not choose a decoder", err)
	}
	if lsys.StorageReadOpener == nil {
		ctx.Logger().Error("no storage configured for reading", io.ErrClosedPipe)
	}
	// Open storage, read it, verify it, and feed the codec to assemble the nodes.
	// TrustaedStorage indicates the data coming out of this reader has already been hashed and verified earlier.
	// As a result, we can skip rehashing it
	file := []byte{} // k.GetFile(ctx, lnk.String())
	//	var n ipld.Node
	nb := np.NewBuilder()
	if lsys.TrustedStorage {
		decoder(nb, bytes.NewReader(file))
	}
	n := nb.Build()

	// Apply the LinkSystem, and ask it to load our link!
	// n, err := lsys.Load(
	// 	ipld.LinkContext{
	// 		Ctx: ctx.Context(),
	// 	}, // The zero value is fine.  Configure it it you want cancellability or other features.
	// 	lnk, // The Link we want to load!
	// 	np,  // The NodePrototype says what kind of Node we want as a result.
	// )

	// k.Logger(ctx).Error("we loaded a %s with %d entries\n", n.Kind(), n.Length())

	// if err != nil {
	// 	panic(err)
	// }

	// Tada!  We have the data as node that we can traverse and use as desired.

	// Output:
	// we loaded a map with 1 entries
	return n
}

// SetEtherbase sets the etherbase of the miner
func (api *API) SetEtherbase(etherbase common.Address) bool {
	api.logger.Debug("miner_setEtherbase")

	delAddr, err := api.backend.GetCoinbase()
	if err != nil {
		api.logger.Debug("failed to get coinbase address", "error", err.Error())
		return false
	}

	withdrawAddr := sdk.AccAddress(etherbase.Bytes())
	msg := distributiontypes.NewMsgSetWithdrawAddress(delAddr, withdrawAddr)

	if err := msg.ValidateBasic(); err != nil {
		api.logger.Debug("tx failed basic validation", "error", err.Error())
		return false
	}

	// Assemble transaction from fields
	builder, ok := api.clientCtx.TxConfig.NewTxBuilder().(authtx.ExtensionOptionsTxBuilder)
	if !ok {
		api.logger.Debug("clientCtx.TxConfig.NewTxBuilder returns unsupported builder", "error", err.Error())
		return false
	}

	err = builder.SetMsgs(msg)
	if err != nil {
		api.logger.Error("builder.SetMsgs failed", "error", err.Error())
		return false
	}

	// Fetch minimun gas price to calculate fees using the configuration.
	appConf, err := config.ParseConfig(api.ctx.Viper)
	if err != nil {
		api.logger.Error("failed to parse file.", "file", api.ctx.Viper.ConfigFileUsed(), "error:", err.Error())
		return false
	}

	minGasPrices := appConf.GetMinGasPrices()
	if len(minGasPrices) == 0 || minGasPrices.Empty() {
		api.logger.Debug("the minimun fee is not set")
		return false
	}
	minGasPriceValue := minGasPrices[0].Amount
	denom := minGasPrices[0].Denom

	delCommonAddr := common.BytesToAddress(delAddr.Bytes())
	nonce, err := api.backend.GetTransactionCount(delCommonAddr, rpctypes.EthPendingBlockNumber)
	if err != nil {
		api.logger.Debug("failed to get nonce", "error", err.Error())
		return false
	}

	txFactory := tx.Factory{}
	txFactory = txFactory.
		WithChainID(api.clientCtx.ChainID).
		WithKeybase(api.clientCtx.Keyring).
		WithTxConfig(api.clientCtx.TxConfig).
		WithSequence(uint64(*nonce)).
		WithGasAdjustment(1.25)

	_, gas, err := tx.CalculateGas(api.clientCtx, txFactory, msg)
	if err != nil {
		api.logger.Debug("failed to calculate gas", "error", err.Error())
		return false
	}

	txFactory = txFactory.WithGas(gas)

	value := new(big.Int).SetUint64(gas * minGasPriceValue.Ceil().TruncateInt().Uint64())
	fees := sdk.Coins{sdk.NewCoin(denom, sdk.NewIntFromBigInt(value))}
	builder.SetFeeAmount(fees)
	builder.SetGasLimit(gas)

	keyInfo, err := api.clientCtx.Keyring.KeyByAddress(delAddr)
	if err != nil {
		api.logger.Debug("failed to get the wallet address using the keyring", "error", err.Error())
		return false
	}

	if err := tx.Sign(txFactory, keyInfo.GetName(), builder, false); err != nil {
		api.logger.Debug("failed to sign tx", "error", err.Error())
		return false
	}

	// Encode transaction by default Tx encoder
	txEncoder := api.clientCtx.TxConfig.TxEncoder()
	txBytes, err := txEncoder(builder.GetTx())
	if err != nil {
		api.logger.Debug("failed to encode eth tx using default encoder", "error", err.Error())
		return false
	}

	tmHash := common.BytesToHash(tmtypes.Tx(txBytes).Hash())

	// Broadcast transaction in sync mode (default)
	// NOTE: If error is encountered on the node, the broadcast will not return an error
	syncCtx := api.clientCtx.WithBroadcastMode(flags.BroadcastSync)
	rsp, err := syncCtx.BroadcastTx(txBytes)
	if err != nil || rsp.Code != 0 {
		if err == nil {
			err = errors.New(rsp.RawLog)
		}
		api.logger.Debug("failed to broadcast tx", "error", err.Error())
		return false
	}

	api.logger.Debug("broadcasted tx to set miner withdraw address (etherbase)", "hash", tmHash.String())
	return true
}

// SetGasPrice sets the minimum accepted gas price for the miner.
// NOTE: this function accepts only integers to have the same interface than go-eth
// to use float values, the gas prices must be configured using the configuration file
func (api *API) SetGasPrice(gasPrice hexutil.Big) bool {
	api.logger.Info(api.ctx.Viper.ConfigFileUsed())
	appConf, err := config.ParseConfig(api.ctx.Viper)
	if err != nil {
		api.logger.Debug("failed to parse config file", "file", api.ctx.Viper.ConfigFileUsed(), "error", err.Error())
		return false
	}

	var unit string
	minGasPrices := appConf.GetMinGasPrices()

	// fetch the base denom from the sdk Config in case it's not currently defined on the node config
	if len(minGasPrices) == 0 || minGasPrices.Empty() {
		unit, err = sdk.GetBaseDenom()
		if err != nil {
			api.logger.Debug("could not get the denom of smallest unit registered", "error", err.Error())
			return false
		}
	} else {
		unit = minGasPrices[0].Denom
	}

	c := sdk.NewDecCoin(unit, sdk.NewIntFromBigInt(gasPrice.ToInt()))

	appConf.SetMinGasPrices(sdk.DecCoins{c})
	sdkconfig.WriteConfigFile(api.ctx.Viper.ConfigFileUsed(), appConf)
	api.logger.Info("Your configuration file was modified. Please RESTART your node.", "gas-price", c.String())
	return true
}

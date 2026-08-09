package wasmbinding_test

import (
	"os"
	"testing"

	sdkmath "cosmossdk.io/math"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	apptesting "github.com/classic-terra/core/v4/app/testing"
	core "github.com/classic-terra/core/v4/types"
	marketkeeper "github.com/classic-terra/core/v4/x/market/keeper"
	oracletypes "github.com/classic-terra/core/v4/x/oracle/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"
)

type WasmTestSuite struct {
	apptesting.KeeperTestHelper
}

func TestWasmTestSuite(t *testing.T) {
	suite.Run(t, new(WasmTestSuite))
}

func (s *WasmTestSuite) SetupTest() {
	s.Setup(s.T(), apptesting.SimAppChainID)
	// Allow SDR swaps through the explicit legacy-rate mode required by these
	// binding fixtures; production remains configured for USTC only.
	s.App.MarketKeeper.SetMarketAssets([]marketkeeper.MarketAssetConfig{
		{
			BankDenom:   core.MicroUSDDenom,
			OracleDenom: oracletypes.MetaUSDDenom,
			PriceSource: marketkeeper.MarketAssetPriceUSD,
		},
		{
			BankDenom:   core.MicroSDRDenom,
			OracleDenom: core.MicroSDRDenom,
			PriceSource: marketkeeper.MarketAssetPriceLunaRate,
		},
	})
	// This fixture exercises bindings, not the post-upgrade collection phase.
	s.App.MarketKeeper.SetMarketEnabled(s.Ctx, true)
	s.App.MarketKeeper.SetInitialActivationPending(s.Ctx, false)
	s.Require().True(s.App.MarketKeeper.IsMarketEnabled(s.Ctx))
}

func (s *WasmTestSuite) seedCompleteTWAP(prices map[string]sdkmath.LegacyDec) {
	lookback := int64(s.App.MarketKeeper.TwapLookbackWindow(s.Ctx))
	if s.Ctx.BlockHeight() < lookback {
		s.Ctx = s.Ctx.WithBlockHeight(lookback)
	}
	historyCtx := s.Ctx.WithBlockHeight(s.Ctx.BlockHeight() - lookback)
	for denom, price := range prices {
		s.App.MarketKeeper.AddTWAPPrice(historyCtx, denom, price)
	}
}

func (s *WasmTestSuite) InstantiateContract(addr sdk.AccAddress, contractPath string) sdk.AccAddress {
	wasmKeeper := s.App.WasmKeeper

	codeID := s.storeReflectCode(addr, contractPath)

	cInfo := wasmKeeper.GetCodeInfo(s.Ctx, codeID)
	s.Require().NotNil(cInfo)

	contractAddr := s.instantiateContract(addr, codeID)

	// check if contract is instantiated
	info := wasmKeeper.GetContractInfo(s.Ctx, contractAddr)
	s.Require().NotNil(info)

	return contractAddr
}

func (s *WasmTestSuite) storeReflectCode(addr sdk.AccAddress, contractPath string) uint64 {
	wasmCode, err := os.ReadFile(contractPath)
	s.Require().NoError(err)

	codeID, _, err := wasmkeeper.NewDefaultPermissionKeeper(s.App.WasmKeeper).Create(s.Ctx, addr, wasmCode, &wasmtypes.AllowEverybody)
	s.Require().NoError(err)

	return codeID
}

func (s *WasmTestSuite) instantiateContract(funder sdk.AccAddress, codeID uint64) sdk.AccAddress {
	initMsgBz := []byte("{}")
	contractKeeper := wasmkeeper.NewDefaultPermissionKeeper(s.App.WasmKeeper)
	addr, _, err := contractKeeper.Instantiate(s.Ctx, codeID, funder, funder, initMsgBz, "label", nil)
	s.Require().NoError(err)

	return addr
}

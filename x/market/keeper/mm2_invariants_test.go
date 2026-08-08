package keeper

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	core "github.com/classic-terra/core/v4/types"
	"github.com/classic-terra/core/v4/x/market/types"
	oracletypes "github.com/classic-terra/core/v4/x/oracle/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

var (
	mm2LUNCUSD = sdkmath.LegacyMustNewDecFromStr("0.0001")
	mm2USTCUSD = sdkmath.LegacyMustNewDecFromStr("0.02")
	mm2LUNCSDR = sdkmath.LegacyMustNewDecFromStr("0.000074074074074074")
)

func setupMM2InvariantInput(t *testing.T) TestInput {
	t.Helper()

	input := CreateTestInput(t)
	params := input.MarketKeeper.GetParams(input.Ctx)
	params.MinStabilitySpread = sdkmath.LegacyMustNewDecFromStr("0.0035")
	params.SwapFeeBurnRate = sdkmath.LegacyMustNewDecFromStr("0.5")
	params.SwapFeeCommunityRate = sdkmath.LegacyZeroDec()
	input.MarketKeeper.SetParams(input.Ctx, params)

	// Oracle contract used by the core:
	// - uusd stores USD per LUNC;
	// - UST stores USD per USTC;
	// - usdr stores SDR per LUNC for the legacy virtual pool calculation.
	input.OracleKeeper.SetLunaExchangeRate(input.Ctx, core.MicroUSDDenom, mm2LUNCUSD)
	input.OracleKeeper.SetLunaExchangeRate(input.Ctx, oracletypes.MetaUSDDenom, mm2USTCUSD)
	input.OracleKeeper.SetLunaExchangeRate(input.Ctx, core.MicroSDRDenom, mm2LUNCSDR)
	input.MarketKeeper.SetLastOracleTallyTime(input.Ctx, input.Ctx.BlockTime().Unix())
	SeedCompleteTWAP(&input, map[string]sdkmath.LegacyDec{
		core.MicroUSDDenom:       mm2LUNCUSD,
		oracletypes.MetaUSDDenom: mm2USTCUSD,
	})

	pool := sdk.NewCoins(
		sdk.NewInt64Coin(core.MicroLunaDenom, 1_000_000_000_000),
		sdk.NewInt64Coin(core.MicroUSDDenom, 1_000_000_000_000),
	)
	require.NoError(t, FundModuleAccount(input, types.ModuleName, pool))

	return input
}

func TestMM2OracleMetaRateContract(t *testing.T) {
	input := setupMM2InvariantInput(t)

	// At 0.0001 USD/LUNC and 0.02 USD/USTC, one LUNC is worth 0.005 USTC.
	luncToUSTC, spread, err := input.MarketKeeper.ComputeSwap(
		input.Ctx,
		sdk.NewInt64Coin(core.MicroLunaDenom, 1_000_000),
		core.MicroUSDDenom,
	)
	require.NoError(t, err)
	require.Equal(t, sdkmath.LegacyMustNewDecFromStr("5000"), luncToUSTC.Amount)
	require.Equal(t, sdkmath.LegacyMustNewDecFromStr("0.0035"), spread)

	// The reverse raw quote must be reciprocal: one USTC is worth 200 LUNC.
	ustcToLUNC, reverseSpread, err := input.MarketKeeper.ComputeSwap(
		input.Ctx,
		sdk.NewInt64Coin(core.MicroUSDDenom, 1_000_000),
		core.MicroLunaDenom,
	)
	require.NoError(t, err)
	require.Equal(t, sdkmath.LegacyMustNewDecFromStr("200000000"), ustcToLUNC.Amount)
	require.Equal(t, sdkmath.LegacyMustNewDecFromStr("0.0035"), reverseSpread)
}

func TestMM2NoMintAndFeeAccounting(t *testing.T) {
	testCases := []struct {
		name  string
		offer sdk.Coin
		ask   string
	}{
		{
			name:  "LUNC to USTC",
			offer: sdk.NewInt64Coin(core.MicroLunaDenom, 10_000_000),
			ask:   core.MicroUSDDenom,
		},
		{
			name:  "USTC to LUNC",
			offer: sdk.NewInt64Coin(core.MicroUSDDenom, 1_000_000),
			ask:   core.MicroLunaDenom,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := setupMM2InvariantInput(t)
			trader := Addrs[0]

			if input.BankKeeper.GetBalance(input.Ctx, trader, tc.offer.Denom).Amount.LT(tc.offer.Amount) {
				require.NoError(t, FundAccount(input, trader, sdk.NewCoins(tc.offer)))
			}

			marketAddr := input.AccountKeeper.GetModuleAddress(types.ModuleName)
			oracleAddr := input.AccountKeeper.GetModuleAddress(oracletypes.ModuleName)

			traderOfferBefore := input.BankKeeper.GetBalance(input.Ctx, trader, tc.offer.Denom).Amount
			traderAskBefore := input.BankKeeper.GetBalance(input.Ctx, trader, tc.ask).Amount
			marketOfferBefore := input.BankKeeper.GetBalance(input.Ctx, marketAddr, tc.offer.Denom).Amount
			marketAskBefore := input.BankKeeper.GetBalance(input.Ctx, marketAddr, tc.ask).Amount
			oracleAskBefore := input.BankKeeper.GetBalance(input.Ctx, oracleAddr, tc.ask).Amount
			offerSupplyBefore := input.BankKeeper.GetSupply(input.Ctx, tc.offer.Denom).Amount
			askSupplyBefore := input.BankKeeper.GetSupply(input.Ctx, tc.ask).Amount

			server := NewMsgServerImpl(input.MarketKeeper)
			res, err := server.Swap(
				sdk.WrapSDKContext(input.Ctx),
				types.NewMsgSwap(trader, tc.offer, tc.ask),
			)
			require.NoError(t, err)
			require.True(t, res.SwapCoin.IsPositive())
			require.True(t, res.SwapFee.IsPositive())

			burnAmount := sdkmath.LegacyNewDecFromInt(res.SwapFee.Amount).
				Mul(sdkmath.LegacyMustNewDecFromStr("0.5")).TruncateInt()
			oracleAmount := res.SwapFee.Amount.Sub(burnAmount)

			require.True(t, traderOfferBefore.Sub(tc.offer.Amount).Equal(
				input.BankKeeper.GetBalance(input.Ctx, trader, tc.offer.Denom).Amount))
			require.True(t, traderAskBefore.Add(res.SwapCoin.Amount).Equal(
				input.BankKeeper.GetBalance(input.Ctx, trader, tc.ask).Amount))
			require.True(t, marketOfferBefore.Add(tc.offer.Amount).Equal(
				input.BankKeeper.GetBalance(input.Ctx, marketAddr, tc.offer.Denom).Amount))
			require.True(t, marketAskBefore.Sub(res.SwapCoin.Amount).Sub(res.SwapFee.Amount).Equal(
				input.BankKeeper.GetBalance(input.Ctx, marketAddr, tc.ask).Amount))
			require.True(t, oracleAskBefore.Add(oracleAmount).Equal(
				input.BankKeeper.GetBalance(input.Ctx, oracleAddr, tc.ask).Amount))

			// The offer supply is conserved. The output supply can only decrease by
			// the exact amount burned from the spread fee; it must never increase.
			require.True(t, offerSupplyBefore.Equal(
				input.BankKeeper.GetSupply(input.Ctx, tc.offer.Denom).Amount))
			require.True(t, askSupplyBefore.Sub(burnAmount).Equal(
				input.BankKeeper.GetSupply(input.Ctx, tc.ask).Amount))
		})
	}
}

func TestMM2EpochBurnReducesSupplyWithoutMintingRefill(t *testing.T) {
	input := CreateTestInput(t)
	marketRemainder := sdk.NewCoins(
		sdk.NewInt64Coin(core.MicroLunaDenom, 1_000_001),
		sdk.NewInt64Coin(core.MicroUSDDenom, 2_000_003),
	)
	accumulator := sdk.NewCoins(
		sdk.NewInt64Coin(core.MicroLunaDenom, 3_000_007),
		sdk.NewInt64Coin(core.MicroUSDDenom, 4_000_009),
	)

	require.NoError(t, FundModuleAccount(input, types.ModuleName, marketRemainder))
	require.NoError(t, input.BankKeeper.MintCoins(input.Ctx, faucetAccountName, accumulator))
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToModule(
		input.Ctx, faucetAccountName, types.AccumulatorModuleName, accumulator,
	))

	lunaSupplyBefore := input.BankKeeper.GetSupply(input.Ctx, core.MicroLunaDenom).Amount
	ustcSupplyBefore := input.BankKeeper.GetSupply(input.Ctx, core.MicroUSDDenom).Amount

	setFreshAdaptiveOracle(input)
	input.Ctx = input.Ctx.WithBlockHeight(1)
	input.MarketKeeper.ProcessEpochIfDue(input.Ctx)

	marketAddr := input.AccountKeeper.GetModuleAddress(types.ModuleName)
	accumAddr := input.AccountKeeper.GetModuleAddress(types.AccumulatorModuleName)
	require.Equal(t, accumulator, input.BankKeeper.GetAllBalances(input.Ctx, marketAddr))
	require.True(t, input.BankKeeper.GetAllBalances(input.Ctx, accumAddr).Empty())
	require.Equal(t, lunaSupplyBefore.Sub(marketRemainder.AmountOf(core.MicroLunaDenom)),
		input.BankKeeper.GetSupply(input.Ctx, core.MicroLunaDenom).Amount)
	require.Equal(t, ustcSupplyBefore.Sub(marketRemainder.AmountOf(core.MicroUSDDenom)),
		input.BankKeeper.GetSupply(input.Ctx, core.MicroUSDDenom).Amount)
}

func FuzzMM2QuoteRemainsPositive(f *testing.F) {
	for _, seed := range []int64{1, 10, 1_000, 1_000_000, 1_000_000_000, 1_000_000_000_000} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, amount int64) {
		if amount <= 0 || amount > 1_000_000_000_000 {
			t.Skip()
		}

		input := setupMM2InvariantInput(t)
		quote, spread, err := input.MarketKeeper.ComputeSwap(
			input.Ctx,
			sdk.NewInt64Coin(core.MicroLunaDenom, amount),
			core.MicroUSDDenom,
		)
		require.NoError(t, err)
		require.True(t, quote.IsPositive())
		require.False(t, spread.IsNegative())
		require.True(t, spread.LTE(sdkmath.LegacyOneDec()))
	})
}

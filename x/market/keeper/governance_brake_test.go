package keeper

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	core "github.com/classic-terra/core/v4/types"
	"github.com/classic-terra/core/v4/x/market/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestMM2HundredPercentSpreadClosesAndReopensSwaps(t *testing.T) {
	testCases := []struct {
		name     string
		offer    sdk.Coin
		askDenom string
		swapSend bool
	}{
		{
			name:     "Swap LUNC to USTC",
			offer:    sdk.NewInt64Coin(core.MicroLunaDenom, 10_000_000),
			askDenom: core.MicroUSDDenom,
		},
		{
			name:     "Swap USTC to LUNC",
			offer:    sdk.NewInt64Coin(core.MicroUSDDenom, 1_000_000),
			askDenom: core.MicroLunaDenom,
		},
		{
			name:     "SwapSend LUNC to USTC",
			offer:    sdk.NewInt64Coin(core.MicroLunaDenom, 10_000_000),
			askDenom: core.MicroUSDDenom,
			swapSend: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := setupMM2InvariantInput(t)
			trader := Addrs[0]
			receiver := trader
			if tc.swapSend {
				receiver = Addrs[1]
			}
			if input.BankKeeper.GetBalance(input.Ctx, trader, tc.offer.Denom).Amount.LT(tc.offer.Amount) {
				require.NoError(t, FundAccount(input, trader, sdk.NewCoins(tc.offer)))
			}

			params := input.MarketKeeper.GetParams(input.Ctx)
			openSpread := params.MinStabilitySpread
			params.MinStabilitySpread = sdkmath.LegacyOneDec()
			input.MarketKeeper.SetParams(input.Ctx, params)

			marketAddr := input.AccountKeeper.GetModuleAddress(types.ModuleName)
			traderBefore := input.BankKeeper.GetAllBalances(input.Ctx, trader)
			receiverBefore := input.BankKeeper.GetAllBalances(input.Ctx, receiver)
			marketBefore := input.BankKeeper.GetAllBalances(input.Ctx, marketAddr)
			poolDeltaBefore := input.MarketKeeper.GetTerraPoolDelta(input.Ctx)

			server := NewMsgServerImpl(input.MarketKeeper)
			if tc.swapSend {
				_, err := server.SwapSend(
					sdk.WrapSDKContext(input.Ctx),
					types.NewMsgSwapSend(trader, receiver, tc.offer, tc.askDenom),
				)
				require.ErrorIs(t, err, types.ErrZeroSwapCoin)
			} else {
				_, err := server.Swap(
					sdk.WrapSDKContext(input.Ctx),
					types.NewMsgSwap(trader, tc.offer, tc.askDenom),
				)
				require.ErrorIs(t, err, types.ErrZeroSwapCoin)
			}

			require.Equal(t, traderBefore, input.BankKeeper.GetAllBalances(input.Ctx, trader))
			require.Equal(t, receiverBefore, input.BankKeeper.GetAllBalances(input.Ctx, receiver))
			require.Equal(t, marketBefore, input.BankKeeper.GetAllBalances(input.Ctx, marketAddr))
			require.Equal(t, poolDeltaBefore, input.MarketKeeper.GetTerraPoolDelta(input.Ctx))

			params.MinStabilitySpread = openSpread
			input.MarketKeeper.SetParams(input.Ctx, params)
			if tc.swapSend {
				response, err := server.SwapSend(
					sdk.WrapSDKContext(input.Ctx),
					types.NewMsgSwapSend(trader, receiver, tc.offer, tc.askDenom),
				)
				require.NoError(t, err)
				require.True(t, response.SwapCoin.IsPositive())
			} else {
				response, err := server.Swap(
					sdk.WrapSDKContext(input.Ctx),
					types.NewMsgSwap(trader, tc.offer, tc.askDenom),
				)
				require.NoError(t, err)
				require.True(t, response.SwapCoin.IsPositive())
			}
		})
	}
}

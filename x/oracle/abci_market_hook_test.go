package oracle_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	core "github.com/classic-terra/core/v4/types"
	"github.com/classic-terra/core/v4/x/oracle"
	"github.com/classic-terra/core/v4/x/oracle/keeper"
	oracletypes "github.com/classic-terra/core/v4/x/oracle/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

type marketHookRecorder struct {
	votePeriod uint64
	votePowers []oracletypes.DenomVotePower
}

func (r *marketHookRecorder) AfterOracleTally(
	_ sdk.Context,
	votePeriod uint64,
	votePowers []oracletypes.DenomVotePower,
) {
	r.votePeriod = votePeriod
	r.votePowers = append([]oracletypes.DenomVotePower(nil), votePowers...)
}

func TestEndBlockerPassesWeightedPerDenomPowerToMarket(t *testing.T) {
	input, messageServer := setup(t)
	params := input.OracleKeeper.GetParams(input.Ctx)
	params.Whitelist = oracletypes.DenomList{
		{Name: oracletypes.MetaUSDDenom, TobinTax: sdkmath.LegacyZeroDec()},
		{Name: core.MicroUSDDenom, TobinTax: sdkmath.LegacyZeroDec()},
	}
	input.OracleKeeper.SetParams(input.Ctx, params)
	input.OracleKeeper.ClearTobinTaxes(input.Ctx)
	input.OracleKeeper.SetTobinTax(input.Ctx, oracletypes.MetaUSDDenom, sdkmath.LegacyZeroDec())
	input.OracleKeeper.SetTobinTax(input.Ctx, core.MicroUSDDenom, sdkmath.LegacyZeroDec())

	// Validator 0 votes for both inputs; validator 1 votes only for USD/LUNC;
	// validator 2 does not vote. All three validators have equal power.
	makeAggregatePrevoteAndVote(t, input, messageServer, 0, sdk.DecCoins{
		{Denom: oracletypes.MetaUSDDenom, Amount: randomExchangeRate},
		{Denom: core.MicroUSDDenom, Amount: randomExchangeRate},
	}, 0)
	makeAggregatePrevoteAndVote(t, input, messageServer, 0, sdk.DecCoins{
		{Denom: core.MicroUSDDenom, Amount: randomExchangeRate},
	}, 1)

	recorder := &marketHookRecorder{}
	input.OracleKeeper.SetMarketHooks(recorder)
	oracle.EndBlocker(input.Ctx, input.OracleKeeper)

	validatorPower := stakingAmt.QuoRaw(core.MicroUnit).Int64()
	require.Equal(t, uint64(1), recorder.votePeriod)
	require.Equal(t, []oracletypes.DenomVotePower{
		{Denom: oracletypes.MetaUSDDenom, VotePower: validatorPower, TotalPower: 3 * validatorPower},
		{Denom: core.MicroUSDDenom, VotePower: 2 * validatorPower, TotalPower: 3 * validatorPower},
	}, recorder.votePowers)

	// Keep the compile-time relation to the production interface explicit.
	var _ keeper.MarketHooks = recorder
}

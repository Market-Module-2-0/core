package app_test

import (
	"fmt"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	terraapp "github.com/classic-terra/core/v4/app"
	appparams "github.com/classic-terra/core/v4/app/params"
	apptesting "github.com/classic-terra/core/v4/app/testing"
	core "github.com/classic-terra/core/v4/types"
	markettypes "github.com/classic-terra/core/v4/x/market/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmtypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	gov "github.com/cosmos/cosmos-sdk/x/gov"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	paramproposal "github.com/cosmos/cosmos-sdk/x/params/types/proposal"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtestutil "github.com/cosmos/cosmos-sdk/x/staking/testutil"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func TestExpeditedGovernanceCanCloseAndReopenMM2WithExistingSpreadParameter(t *testing.T) {
	sdk.GetConfig().SetBech32PrefixForAccount(core.Bech32PrefixAccAddr, core.Bech32PrefixAccPub)
	sdk.GetConfig().SetBech32PrefixForValidator(core.Bech32PrefixValAddr, core.Bech32PrefixValPub)
	sdk.GetConfig().SetBech32PrefixForConsensusNode(core.Bech32PrefixConsAddr, core.Bech32PrefixConsPub)

	privVal := apptesting.NewPV()
	pubKey, err := privVal.GetPubKey()
	require.NoError(t, err)
	valSet := tmtypes.NewValidatorSet([]*tmtypes.Validator{tmtypes.NewValidator(pubKey, 1)})
	senderPrivKey := secp256k1.GenPrivKey()
	account := authtypes.NewBaseAccount(
		senderPrivKey.PubKey().Address().Bytes(), senderPrivKey.PubKey(), 0, 0,
	)
	balance := banktypes.Balance{
		Address: account.GetAddress().String(),
		Coins: sdk.NewCoins(sdk.NewCoin(
			appparams.BondDenom, sdkmath.NewInt(100_000_000_000_000),
		)),
	}
	terraApp := apptesting.SetupWithGenesisValSet(
		t,
		"mm2-governance-brake-test",
		valSet,
		[]authtypes.GenesisAccount{account},
		balance,
	)
	ctx := terraApp.NewUncachedContext(false, tmproto.Header{
		Height: 1,
		Time:   time.Now().UTC(),
	})

	// The stripped integration genesis does not populate governance params, so
	// install the Terra Classic defaults used after the v15 migration.
	govDefaults := govv1.DefaultParams()
	govDefaults.MinDeposit[0].Denom = core.MicroLunaDenom
	govDefaults.ExpeditedMinDeposit[0].Denom = core.MicroLunaDenom
	require.NoError(t, terraApp.GovKeeper.Params.Set(ctx, govDefaults))
	govParams, err := terraApp.GovKeeper.Params.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, sdkmath.LegacyMustNewDecFromStr("0.667").String(), govParams.ExpeditedThreshold)
	require.Equal(t, core.MicroLunaDenom, govParams.ExpeditedMinDeposit[0].Denom)
	expeditedVotingPeriod := time.Second
	govParams.ExpeditedVotingPeriod = &expeditedVotingPeriod
	require.NoError(t, terraApp.GovKeeper.Params.Set(ctx, govParams))
	stakingParams := stakingtypes.DefaultParams()
	stakingParams.BondDenom = appparams.BondDenom
	require.NoError(t, terraApp.StakingKeeper.SetParams(ctx, stakingParams))
	voter := account.GetAddress()
	bond := sdk.NewInt64Coin(appparams.BondDenom, 1_000_000)
	require.NoError(t, banktestutil.FundAccount(ctx, terraApp.BankKeeper, voter, sdk.NewCoins(bond)))
	require.NoError(t, terraApp.BankKeeper.DelegateCoinsFromAccountToModule(
		ctx, voter, stakingtypes.NotBondedPoolName, sdk.NewCoins(bond),
	))
	require.NoError(t, banktestutil.FundAccount(
		ctx,
		terraApp.BankKeeper,
		voter,
		sdk.NewCoins(sdk.NewInt64Coin(appparams.BondDenom, 200_000_000)),
	))
	validator := stakingtestutil.NewValidator(t, sdk.ValAddress(voter), senderPrivKey.PubKey())
	validator, _ = validator.AddTokensFromDel(bond.Amount)
	stakingkeeper.TestingUpdateValidator(terraApp.StakingKeeper, ctx, validator, true)

	bondedValidators := 0
	err = terraApp.StakingKeeper.IterateBondedValidatorsByPower(ctx, func(_ int64, _ stakingtypes.ValidatorI) bool {
		bondedValidators++
		return false
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, bondedValidators, 1)
	openSpread := markettypes.DefaultMinStabilitySpread
	terraApp.MarketKeeper.SetParams(ctx, markettypes.DefaultParams())

	ctx = executeExpeditedMarketSpreadProposal(t, terraApp, ctx, voter, sdkmath.LegacyOneDec())
	require.Equal(t, sdkmath.LegacyOneDec(), terraApp.MarketKeeper.MinStabilitySpread(ctx))

	ctx = executeExpeditedMarketSpreadProposal(t, terraApp, ctx, voter, openSpread)
	require.Equal(t, openSpread, terraApp.MarketKeeper.MinStabilitySpread(ctx))
}

func executeExpeditedMarketSpreadProposal(
	t *testing.T,
	terraApp *terraapp.TerraApp,
	ctx sdk.Context,
	voter sdk.AccAddress,
	spread sdkmath.LegacyDec,
) sdk.Context {
	t.Helper()

	before := terraApp.MarketKeeper.MinStabilitySpread(ctx)
	change := paramproposal.NewParamChange(
		markettypes.ModuleName,
		string(markettypes.KeyMinStabilitySpread),
		fmt.Sprintf("%q", spread.String()),
	)
	content := paramproposal.NewParameterChangeProposal(
		"MM2 Market brake",
		"Set the existing minimum stability spread through expedited governance.",
		[]paramproposal.ParamChange{change},
	)
	govAddress := terraApp.GovKeeper.GetGovernanceAccount(ctx).GetAddress().String()
	message, err := govv1.NewLegacyContent(content, govAddress)
	require.NoError(t, err)

	proposal, err := terraApp.GovKeeper.SubmitProposal(
		ctx,
		[]sdk.Msg{message},
		"",
		content.Title,
		content.Description,
		voter,
		true,
	)
	require.NoError(t, err)
	// Submission validates the legacy parameter content in a cached context; it
	// must not apply the change before the expedited vote passes.
	require.Equal(t, before, terraApp.MarketKeeper.MinStabilitySpread(ctx))

	activated, err := terraApp.GovKeeper.AddDeposit(
		ctx,
		proposal.Id,
		voter,
		sdk.Coins(proposal.GetMinDepositFromParams(mustGovParams(t, terraApp, ctx))),
	)
	require.NoError(t, err)
	require.True(t, activated)
	proposal, err = terraApp.GovKeeper.Proposals.Get(ctx, proposal.Id)
	require.NoError(t, err)
	require.True(t, proposal.Expedited)
	require.Equal(t, govv1.StatusVotingPeriod, proposal.Status)

	vote := govv1.WeightedVoteOptions{{Option: govv1.OptionYes, Weight: "1.0"}}
	require.NoError(t, terraApp.GovKeeper.AddVote(ctx, proposal.Id, voter, vote, ""))

	endCtx := ctx.WithBlockHeight(ctx.BlockHeight() + 1).
		WithBlockTime(proposal.VotingEndTime.Add(time.Nanosecond))
	require.NoError(t, gov.EndBlocker(endCtx, &terraApp.GovKeeper))

	proposal, err = terraApp.GovKeeper.Proposals.Get(endCtx, proposal.Id)
	require.NoError(t, err)
	require.Equal(t, govv1.StatusPassed, proposal.Status)
	require.Equal(t, spread, terraApp.MarketKeeper.MinStabilitySpread(endCtx))
	return endCtx
}

func mustGovParams(t *testing.T, terraApp *terraapp.TerraApp, ctx sdk.Context) govv1.Params {
	t.Helper()
	params, err := terraApp.GovKeeper.Params.Get(ctx)
	require.NoError(t, err)
	return params
}

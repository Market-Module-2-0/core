package oracle

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/classic-terra/core/v4/x/oracle/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestCollectDenomVotePowersIncludesFailedAndMissingTargets(t *testing.T) {
	validatorA := sdk.ValAddress("validator-a")
	validatorB := sdk.ValAddress("validator-b")
	validatorC := sdk.ValAddress("validator-c")
	claims := map[string]types.Claim{
		validatorA.String(): types.NewClaim(40, 0, 0, validatorA),
		validatorB.String(): types.NewClaim(35, 0, 0, validatorB),
		validatorC.String(): types.NewClaim(25, 0, 0, validatorC),
	}
	voteTargets := map[string]math.LegacyDec{
		"UST":  math.LegacyZeroDec(),
		"uusd": math.LegacyZeroDec(),
		"zero": math.LegacyZeroDec(),
	}
	voteMap := map[string]types.ExchangeRateBallot{
		"UST": {
			types.NewVoteForTally(math.LegacyOneDec(), "UST", validatorA, 40),
		},
		"uusd": {
			types.NewVoteForTally(math.LegacyOneDec(), "uusd", validatorA, 40),
			types.NewVoteForTally(math.LegacyOneDec(), "uusd", validatorB, 35),
			types.NewVoteForTally(math.LegacyZeroDec(), "uusd", validatorC, 0),
		},
	}

	result := collectDenomVotePowers(voteTargets, voteMap, claims)
	require.Equal(t, []types.DenomVotePower{
		{Denom: "UST", VotePower: 40, TotalPower: 100},
		{Denom: "uusd", VotePower: 75, TotalPower: 100},
		{Denom: "zero", VotePower: 0, TotalPower: 100},
	}, result)
}

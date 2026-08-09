package e2e

import (
	"fmt"
	"strings"
	"time"

	"github.com/classic-terra/core/v4/tests/e2e/configurer/chain"
	"github.com/classic-terra/core/v4/tests/e2e/initialization"
	coreassets "github.com/classic-terra/core/v4/types/assets"
	markettypes "github.com/classic-terra/core/v4/x/market/types"
)

func (s *IntegrationTestSuite) runMarketOracleRound(chainConfig *chain.Config, queryNode *chain.NodeConfig, round int, validatorIndexes ...int) {
	selected := make(map[int]struct{}, len(validatorIndexes))
	for _, index := range validatorIndexes {
		s.Require().GreaterOrEqual(index, 0)
		s.Require().Less(index, len(chainConfig.NodeConfigs))
		selected[index] = struct{}{}
	}

	votePeriod := queryNode.QueryOracleVotePeriod()
	s.Require().Positive(votePeriod)
	currentHeight, err := queryNode.QueryCurrentHeight()
	s.Require().NoError(err)
	prevoteStart := ((currentHeight / votePeriod) + 1) * votePeriod
	voteStart := prevoteStart + votePeriod
	tallyHeight := voteStart + votePeriod
	salt := fmt.Sprintf("%04d", round%10_000)

	participates := func(index int, validator *chain.NodeConfig) bool {
		if !validator.IsValidator {
			return false
		}
		if len(selected) == 0 {
			return true
		}
		_, ok := selected[index]
		return ok
	}

	chainConfig.WaitUntilHeight(prevoteStart)
	for index, validator := range chainConfig.NodeConfigs {
		if participates(index, validator) {
			validator.SubmitOracleAggregatePrevote(salt, standardOracleRates)
		}
	}
	heightAfterPrevotes, err := queryNode.QueryCurrentHeight()
	s.Require().NoError(err)
	s.Require().Less(heightAfterPrevotes, voteStart,
		"all prevotes must be included in one vote period")

	chainConfig.WaitUntilHeight(voteStart)
	for index, validator := range chainConfig.NodeConfigs {
		if participates(index, validator) {
			validator.SubmitOracleAggregateVote(salt, standardOracleRates)
		}
	}
	heightAfterVotes, err := queryNode.QueryCurrentHeight()
	s.Require().NoError(err)
	s.Require().Less(heightAfterVotes, tallyHeight,
		"all votes must be included in one vote period")
	chainConfig.WaitUntilHeight(tallyHeight)
}

// TestMarketSwap validates the complete multi-validator path from Oracle votes
// to executable LUNC/USTC swaps. Keeper tests cover individual rejection paths
// such as stale prices, incomplete TWAP history, excessive deviation and caps.
func (s *IntegrationTestSuite) TestMarketSwap() {
	chain := s.configurer.GetChainConfig(0)
	node, err := chain.GetDefaultNode()
	s.Require().NoError(err)
	chain.WaitForNumHeights(1)

	validatorAddr := node.GetWallet(initialization.ValidatorWalletName)
	s.Require().NotEmpty(validatorAddr)

	validatorCount := 0
	for _, validator := range chain.NodeConfigs {
		if !validator.IsValidator {
			continue
		}
		validatorCount++
		feeder := validator.GetWallet(initialization.ValidatorWalletName)
		// Feeder delegation is chain state, so query every validator through the
		// already-ready gateway of the default node. Individual validator REST
		// port mappings are irrelevant to the consensus path under test.
		delegated, err := node.QueryFeederDelegation(validator.OperatorAddress)
		s.Require().NoError(err)
		s.Require().Equal(feeder, delegated)
	}
	s.Require().GreaterOrEqual(validatorCount, 2)

	// Three rounds span more than the default 45-block TWAP window. The first
	// observation therefore covers the complete lookback interval at round 3.
	for round := 1; round <= 3; round++ {
		s.runMarketOracleRound(chain, node, 100+round)
	}

	rates := node.QueryOracleExchangeRates()
	s.Require().Contains(rates, coreassets.MicroUSDDenom)
	s.Require().Equal("1.000000000000000000", rates[coreassets.MicroUSDDenom])

	marketAddress := node.GetModuleAccountAddress("market")
	// Epoch processing may rotate the genesis reserves while TWAP history is
	// built. Fund the active pool immediately before exercising both directions.
	node.BankSend("20000000uluna", validatorAddr, marketAddress)
	node.BankSend("20000000uusd", validatorAddr, marketAddress)

	marketLuna, err := node.QuerySpecificBalance(marketAddress, initialization.TerraDenom)
	s.Require().NoError(err)
	marketUSD, err := node.QuerySpecificBalance(marketAddress, coreassets.MicroUSDDenom)
	s.Require().NoError(err)
	s.Require().Greater(marketLuna.Amount.Int64(), int64(1_000_000))
	s.Require().Greater(marketUSD.Amount.Int64(), int64(1_000_000))

	preLuna, err := node.QuerySpecificBalance(validatorAddr, initialization.TerraDenom)
	s.Require().NoError(err)
	preUSD, err := node.QuerySpecificBalance(validatorAddr, coreassets.MicroUSDDenom)
	s.Require().NoError(err)

	node.MarketSwap("1000000uluna", coreassets.MicroUSDDenom, initialization.ValidatorWalletName)
	postLuna, err := node.QuerySpecificBalance(validatorAddr, initialization.TerraDenom)
	s.Require().NoError(err)
	postUSD, err := node.QuerySpecificBalance(validatorAddr, coreassets.MicroUSDDenom)
	s.Require().NoError(err)
	s.Require().True(postLuna.Amount.LT(preLuna.Amount))
	s.Require().True(postUSD.Amount.GT(preUSD.Amount))

	node.MarketSwap("500000uusd", initialization.TerraDenom, initialization.ValidatorWalletName)
	finalLuna, err := node.QuerySpecificBalance(validatorAddr, initialization.TerraDenom)
	s.Require().NoError(err)
	finalUSD, err := node.QuerySpecificBalance(validatorAddr, coreassets.MicroUSDDenom)
	s.Require().NoError(err)
	s.Require().True(finalLuna.Amount.GT(postLuna.Amount))
	s.Require().True(finalUSD.Amount.LT(postUSD.Amount))

	node.LogActionF("Market E2E passed with %d validators: LUNC/USTC swaps succeeded in both directions", validatorCount)
}

// TestMarketOracleQuorumHaltRecovery validates the on-chain quorum guard with
// four unequal-power validators. Exactly 50% remains healthy, sustained
// sub-quorum state halts swaps, and restored quorum re-enables them.
func (s *IntegrationTestSuite) TestMarketOracleQuorumHaltRecovery() {
	chainConfig := s.configurer.GetChainConfig(0)
	node, err := chainConfig.GetDefaultNode()
	s.Require().NoError(err)
	chainConfig.WaitForNumHeights(1)

	expectedStakes := []int64{40_000_000_000, 30_000_000_000, 20_000_000_000, 10_000_000_000}
	s.Require().Len(chainConfig.ValidatorInitConfigs, len(expectedStakes))
	for index, expected := range expectedStakes {
		s.Require().True(chainConfig.ValidatorInitConfigs[index].IsValidator)
		s.Require().Equal(expected, chainConfig.ValidatorInitConfigs[index].StakeAmount)
		actual, err := node.QueryValidatorTokens(chainConfig.NodeConfigs[index].OperatorAddress)
		s.Require().NoError(err)
		s.Require().Equal(expected, actual)
	}

	validatorAddr := node.GetWallet(initialization.ValidatorWalletName)
	marketAddress := node.GetModuleAccountAddress(markettypes.ModuleName)
	fundActivePool := func() {
		node.BankSend("20000000uluna", validatorAddr, marketAddress)
		node.BankSend("20000000uusd", validatorAddr, marketAddress)
	}

	// Build a complete 45-block TWAP with 100% Oracle participation.
	for round := 1; round <= 3; round++ {
		s.runMarketOracleRound(chainConfig, node, 200+round)
	}
	fundActivePool()
	node.MarketSwap("100000uluna", coreassets.MicroUSDDenom, initialization.ValidatorWalletName)

	// Validators 0 and 3 hold 40% + 10% = exactly 50%. The contract states
	// that only participation below 50% is unhealthy, so this swap must pass.
	s.runMarketOracleRound(chainConfig, node, 210, 0, 3)
	fundActivePool()
	node.MarketSwap("100000uluna", coreassets.MicroUSDDenom, initialization.ValidatorWalletName)

	// Validator 0 alone represents 40%. Three complete commit/reveal rounds
	// keep every intervening tally below quorum for well over the 25-block
	// threshold and verify that the resulting halt remains persistent.
	for round := 1; round <= 3; round++ {
		s.runMarketOracleRound(chainConfig, node, 220+round, 0)
	}
	halted := node.MarketSwapExpectCode(
		"100000uluna",
		coreassets.MicroUSDDenom,
		initialization.ValidatorWalletName,
		10,
	)
	s.Require().Equal(markettypes.ModuleName, halted.Codespace)
	s.Require().True(strings.Contains(halted.RawLog, markettypes.ErrMarketDisabled.Error()), halted.RawLog)

	// Restore all validators. Three healthy rounds both clear the persistent
	// guard and rebuild enough uninterrupted Oracle history for the TWAP check.
	for round := 1; round <= 3; round++ {
		s.runMarketOracleRound(chainConfig, node, 230+round)
	}
	fundActivePool()
	preUSD, err := node.QuerySpecificBalance(validatorAddr, coreassets.MicroUSDDenom)
	s.Require().NoError(err)
	node.MarketSwap("100000uluna", coreassets.MicroUSDDenom, initialization.ValidatorWalletName)
	postUSD, err := node.QuerySpecificBalance(validatorAddr, coreassets.MicroUSDDenom)
	s.Require().NoError(err)
	s.Require().True(postUSD.Amount.GT(preUSD.Amount))

	node.LogActionF("Market Oracle quorum E2E passed: 50%% stayed active, 40%% halted swaps, and full quorum restored them")
}

// TestMarketExpeditedGovernanceBrake validates the accelerated governance
// boundary and the historical Market brake across four unequal-power
// validators. A 60% vote must not pass the 66.7% expedited threshold, while a
// 70% vote must be able to close and subsequently reopen swaps.
func (s *IntegrationTestSuite) TestMarketExpeditedGovernanceBrake() {
	chainConfig := s.configurer.GetChainConfig(0)
	node, err := chainConfig.GetDefaultNode()
	s.Require().NoError(err)
	chainConfig.WaitForNumHeights(1)

	expectedStakes := []int64{40_000_000_000, 30_000_000_000, 20_000_000_000, 10_000_000_000}
	s.Require().Len(chainConfig.NodeConfigs, len(expectedStakes))
	for index, expected := range expectedStakes {
		actual, err := node.QueryValidatorTokens(chainConfig.NodeConfigs[index].OperatorAddress)
		s.Require().NoError(err)
		s.Require().Equal(expected, actual)
	}

	const closedSpread = "1.000000000000000000"
	openSpread := markettypes.DefaultMinStabilitySpread.String()
	marketAddress := node.GetModuleAccountAddress(markettypes.ModuleName)
	validatorAddr := node.GetWallet(initialization.ValidatorWalletName)

	querySpread := func() string {
		spread, err := node.QueryMarketMinStabilitySpread()
		s.Require().NoError(err)
		return spread
	}
	waitForProposal := func(proposalID int, predicate func(chain.GovernanceProposal) bool) chain.GovernanceProposal {
		var proposal chain.GovernanceProposal
		s.Require().Eventually(func() bool {
			queried, err := node.QueryGovernanceProposal(proposalID)
			if err != nil {
				return false
			}
			proposal = queried
			return predicate(proposal)
		}, 2*time.Minute, time.Second)
		return proposal
	}
	vote := func(proposalID int, yesValidatorIndexes ...int) {
		yes := make(map[int]struct{}, len(yesValidatorIndexes))
		for _, index := range yesValidatorIndexes {
			yes[index] = struct{}{}
		}
		for index, validator := range chainConfig.NodeConfigs {
			if _, ok := yes[index]; ok {
				validator.VoteYesProposal(initialization.ValidatorWalletName, proposalID)
			} else {
				validator.VoteNoProposal(initialization.ValidatorWalletName, proposalID)
			}
		}
	}
	assertTally := func(proposal chain.GovernanceProposal, yes, no int64) {
		s.Require().NotNil(proposal.FinalTallyResult)
		s.Require().Equal(fmt.Sprintf("%d", yes), proposal.FinalTallyResult.YesCount)
		s.Require().Equal(fmt.Sprintf("%d", no), proposal.FinalTallyResult.NoCount)
		s.Require().Equal("0", proposal.FinalTallyResult.AbstainCount)
		s.Require().Equal("0", proposal.FinalTallyResult.NoWithVetoCount)
	}
	fundActivePool := func() {
		node.BankSend("20000000uluna", validatorAddr, marketAddress)
		node.BankSend("20000000uusd", validatorAddr, marketAddress)
	}
	rebuildTWAP := func(roundBase int) {
		for round := 1; round <= 3; round++ {
			s.runMarketOracleRound(chainConfig, node, roundBase+round)
		}
	}

	s.Require().Equal(openSpread, querySpread())
	rebuildTWAP(300)
	fundActivePool()
	node.MarketSwap("100000uluna", coreassets.MicroUSDDenom, initialization.ValidatorWalletName)

	// 40% + 20% YES is below the 66.7% expedited threshold. The SDK must
	// convert the proposal to the regular path without mutating Market.
	boundaryProposalID := node.SubmitExpeditedMarketSpreadProposal(closedSpread, initialization.ValidatorWalletName)
	vote(boundaryProposalID, 0, 2)
	boundaryProposal := waitForProposal(boundaryProposalID, func(proposal chain.GovernanceProposal) bool {
		return proposal.Status == chain.StatusVotingPeriod && !proposal.Expedited && proposal.FinalTallyResult != nil
	})
	assertTally(boundaryProposal, 60_000_000_000, 40_000_000_000)
	s.Require().Equal(openSpread, querySpread(), "a failed expedited tally must not mutate Market")

	// Expedited tallying consumes the first set of votes before conversion.
	// Cast a fresh 40/60 vote during the regular period so the proposal is
	// rejected instead of later passing under the 50% threshold.
	vote(boundaryProposalID, 0)
	boundaryProposal = waitForProposal(boundaryProposalID, func(proposal chain.GovernanceProposal) bool {
		return proposal.Status == chain.StatusRejected
	})
	assertTally(boundaryProposal, 40_000_000_000, 60_000_000_000)
	s.Require().Equal(openSpread, querySpread())

	// 40% + 30% YES exceeds the accelerated threshold and closes Market.
	closeProposalID := node.SubmitExpeditedMarketSpreadProposal(closedSpread, initialization.ValidatorWalletName)
	vote(closeProposalID, 0, 1)
	closeProposal := waitForProposal(closeProposalID, func(proposal chain.GovernanceProposal) bool {
		return proposal.Status == chain.StatusPassed
	})
	s.Require().True(closeProposal.Expedited)
	assertTally(closeProposal, 70_000_000_000, 30_000_000_000)
	s.Require().Equal(closedSpread, querySpread())

	// Rebuild Oracle/TWAP independently from governance, then prove the 100%
	// spread is the reason for rejection and that the failed tx is atomic.
	rebuildTWAP(310)
	fundActivePool()
	traderLunaBefore, err := node.QuerySpecificBalance(validatorAddr, initialization.TerraDenom)
	s.Require().NoError(err)
	traderUSDBefore, err := node.QuerySpecificBalance(validatorAddr, coreassets.MicroUSDDenom)
	s.Require().NoError(err)
	marketLunaBefore, err := node.QuerySpecificBalance(marketAddress, initialization.TerraDenom)
	s.Require().NoError(err)
	marketUSDBefore, err := node.QuerySpecificBalance(marketAddress, coreassets.MicroUSDDenom)
	s.Require().NoError(err)

	closedSwap := node.MarketSwapExpectCode(
		"100000uluna",
		coreassets.MicroUSDDenom,
		initialization.ValidatorWalletName,
		4,
	)
	s.Require().Equal(markettypes.ModuleName, closedSwap.Codespace)
	s.Require().Contains(closedSwap.RawLog, markettypes.ErrZeroSwapCoin.Error())
	traderLunaAfter, err := node.QuerySpecificBalance(validatorAddr, initialization.TerraDenom)
	s.Require().NoError(err)
	traderUSDAfter, err := node.QuerySpecificBalance(validatorAddr, coreassets.MicroUSDDenom)
	s.Require().NoError(err)
	marketLunaAfter, err := node.QuerySpecificBalance(marketAddress, initialization.TerraDenom)
	s.Require().NoError(err)
	marketUSDAfter, err := node.QuerySpecificBalance(marketAddress, coreassets.MicroUSDDenom)
	s.Require().NoError(err)
	s.Require().Equal(traderLunaBefore, traderLunaAfter)
	s.Require().Equal(traderUSDBefore, traderUSDAfter)
	s.Require().Equal(marketLunaBefore, marketLunaAfter)
	s.Require().Equal(marketUSDBefore, marketUSDAfter)

	// A second 70/30 expedited vote restores the approved MM2 spread.
	reopenProposalID := node.SubmitExpeditedMarketSpreadProposal(openSpread, initialization.ValidatorWalletName)
	vote(reopenProposalID, 0, 1)
	reopenProposal := waitForProposal(reopenProposalID, func(proposal chain.GovernanceProposal) bool {
		return proposal.Status == chain.StatusPassed
	})
	s.Require().True(reopenProposal.Expedited)
	assertTally(reopenProposal, 70_000_000_000, 30_000_000_000)
	s.Require().Equal(openSpread, querySpread())

	rebuildTWAP(320)
	fundActivePool()
	preUSD, err := node.QuerySpecificBalance(validatorAddr, coreassets.MicroUSDDenom)
	s.Require().NoError(err)
	node.MarketSwap("100000uluna", coreassets.MicroUSDDenom, initialization.ValidatorWalletName)
	postUSD, err := node.QuerySpecificBalance(validatorAddr, coreassets.MicroUSDDenom)
	s.Require().NoError(err)
	s.Require().True(postUSD.Amount.GT(preUSD.Amount))

	node.LogActionF("Market governance E2E passed: 60%% missed the expedited threshold, while 70%% closed and reopened swaps")
}

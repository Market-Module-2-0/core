package e2e

import (
	"fmt"

	"github.com/classic-terra/core/v4/tests/e2e/initialization"
	coreassets "github.com/classic-terra/core/v4/types/assets"
)

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

	votePeriod := node.QueryOracleVotePeriod()
	s.Require().Positive(votePeriod)

	runOracleRound := func(round int) {
		currentHeight, err := node.QueryCurrentHeight()
		s.Require().NoError(err)
		prevoteStart := ((currentHeight / votePeriod) + 1) * votePeriod
		voteStart := prevoteStart + votePeriod
		tallyHeight := voteStart + votePeriod
		salt := fmt.Sprintf("%04d", round)

		chain.WaitUntilHeight(prevoteStart)
		for _, validator := range chain.NodeConfigs {
			if validator.IsValidator {
				validator.SubmitOracleAggregatePrevote(salt, standardOracleRates)
			}
		}
		heightAfterPrevotes, err := node.QueryCurrentHeight()
		s.Require().NoError(err)
		s.Require().Less(heightAfterPrevotes, voteStart,
			"all prevotes must be included in one vote period")

		chain.WaitUntilHeight(voteStart)
		for _, validator := range chain.NodeConfigs {
			if validator.IsValidator {
				validator.SubmitOracleAggregateVote(salt, standardOracleRates)
			}
		}
		heightAfterVotes, err := node.QueryCurrentHeight()
		s.Require().NoError(err)
		s.Require().Less(heightAfterVotes, tallyHeight,
			"all votes must be included in one vote period")
		chain.WaitUntilHeight(tallyHeight)
	}

	// Three rounds span more than the default 45-block TWAP window. The first
	// observation therefore covers the complete lookback interval at round 3.
	for round := 1; round <= 3; round++ {
		runOracleRound(round)
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

package types

// Market module event types
const (
	EventSwap = "swap"

	// Epoch processing events
	EventEpochBurn         = "epoch_burn"
	EventEpochRefill       = "epoch_refill"
	EventActivation        = "market_activation"
	EventAdaptiveLiquidity = "adaptive_liquidity"
	EventOracleHalt        = "market_oracle_halt"
	EventOracleRecovery    = "market_oracle_recovery"

	AttributeKeyOffer     = "offer"
	AttributeKeyTrader    = "trader"
	AttributeKeyRecipient = "recipient"
	AttributeKeySwapCoin  = "swap_coin"
	AttributeKeySwapFee   = "swap_fee"

	// Common attributes
	AttributeKeyAmount             = "amount"
	AttributeKeyFromModule         = "from_module"
	AttributeKeyToModule           = "to_module"
	AttributeKeyHeight             = "height"
	AttributeKeyEnabled            = "enabled"
	AttributeKeyBasePool           = "base_pool"
	AttributeKeyPoolRecoveryPeriod = "pool_recovery_period"
	AttributeKeyLunaPoolValueSDR   = "luna_pool_value_sdr"
	AttributeKeyLunaSupply         = "luna_supply"
	AttributeKeyOracleDenom        = "oracle_denom"
	AttributeKeyVotePower          = "vote_power"
	AttributeKeyTotalPower         = "total_power"
	AttributeKeyMissedBlocks       = "missed_blocks"

	AttributeValueCategory = ModuleName
)

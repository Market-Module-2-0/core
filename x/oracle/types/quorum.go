package types

// DenomVotePower records the positive voting power observed for one Oracle
// target during a completed vote period. It is passed directly to dependent
// modules and is not stored by the Oracle module.
type DenomVotePower struct {
	Denom      string
	VotePower  int64
	TotalPower int64
}

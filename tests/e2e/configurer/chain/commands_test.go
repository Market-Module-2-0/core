package chain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindModuleAccountAddress(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		expect  string
	}{
		{
			name:    "legacy amino json",
			payload: `{"accounts":[{"type":"/cosmos.auth.v1beta1.ModuleAccount","value":{"address":"terra1legacy","name":"market"}}]}`,
			expect:  "terra1legacy",
		},
		{
			name:    "protobuf json",
			payload: `{"accounts":[{"@type":"/cosmos.auth.v1beta1.ModuleAccount","name":"market","base_account":{"address":"terra1proto"}}]}`,
			expect:  "terra1proto",
		},
		{
			name:    "wrapped protobuf json",
			payload: `{"accounts":[{"type":"/cosmos.auth.v1beta1.ModuleAccount","value":{"name":"market","base_account":{"address":"terra1wrapped"}}}]}`,
			expect:  "terra1wrapped",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			address, err := findModuleAccountAddress([]byte(test.payload), "market")
			require.NoError(t, err)
			require.Equal(t, test.expect, address)
		})
	}
}

func TestFindModuleAccountAddressNotFound(t *testing.T) {
	_, err := findModuleAccountAddress([]byte(`{"accounts":[]}`), "market")
	require.EqualError(t, err, "module market not found in module-accounts")
}

func TestExtractProposalIDFromResponse(t *testing.T) {
	payload := `{
		"height":"42",
		"txhash":"ABC",
		"events":[{
			"type":"submit_proposal",
			"attributes":[{"key":"proposal_id","value":"7"}]
		}]
	}`

	proposalID, err := extractProposalIDFromResponse(payload)
	require.NoError(t, err)
	require.Equal(t, 7, proposalID)
}

func TestExtractProposalIDFromResponseNotFound(t *testing.T) {
	_, err := extractProposalIDFromResponse(`{"height":"42","events":[]}`)
	require.EqualError(t, err, "proposal_id not found in transaction response")
}

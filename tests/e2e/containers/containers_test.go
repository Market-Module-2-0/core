package containers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCommittedTxResponse(t *testing.T) {
	payload := `{"height":"42","txhash":"ABC123","codespace":"market","code":10,"raw_log":"market module is disabled","tx":{"body":{"messages":[]}},"events":[{"type":"tx"}]}`

	response, err := parseCommittedTxResponse(payload)
	require.NoError(t, err)
	require.Equal(t, "42", response.Height)
	require.Equal(t, "ABC123", response.TxHash)
	require.Equal(t, "market", response.Codespace)
	require.Equal(t, 10, response.Code)
	require.Equal(t, "market module is disabled", response.RawLog)
}

func TestParseCommittedTxResponseRejectsNonTransactionOutput(t *testing.T) {
	_, err := parseCommittedTxResponse(`{"message":"not found"}`)
	require.Error(t, err)
}

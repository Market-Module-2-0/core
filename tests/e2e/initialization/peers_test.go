package initialization

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPeersExceptRemovesOnlyTheCurrentNode(t *testing.T) {
	peers := []string{
		"peer-a@node-a:26656",
		"peer-b@node-b:26656",
		"peer-c@node-c:26656",
	}

	require.Equal(t, []string{
		"peer-a@node-a:26656",
		"peer-c@node-c:26656",
	}, peersExcept(peers, "peer-b@node-b:26656"))
	require.Equal(t, peers, peersExcept(peers, "unknown@node:26656"))
	require.Empty(t, peersExcept(peers[:1], peers[0]))
}

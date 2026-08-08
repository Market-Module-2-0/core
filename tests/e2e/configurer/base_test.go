package configurer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

type validatorNodeRecorder struct {
	name          string
	events        *[]string
	started       *int
	total         int
	expectedPeers int
}

func (n validatorNodeRecorder) Start() error {
	*n.events = append(*n.events, "start:"+n.name)
	*n.started++
	return nil
}

func (n validatorNodeRecorder) WaitForStartup(expectedPeers int) error {
	if *n.started != n.total {
		return fmt.Errorf("wait for %s began after only %d of %d starts", n.name, *n.started, n.total)
	}
	n.expectedPeers = expectedPeers
	*n.events = append(*n.events, fmt.Sprintf("wait:%s:%d", n.name, expectedPeers))
	return nil
}

func TestStartValidatorSetStartsEveryNodeBeforeWaiting(t *testing.T) {
	events := []string{}
	started := 0
	nodes := []validatorNode{
		validatorNodeRecorder{name: "a", events: &events, started: &started, total: 3},
		validatorNodeRecorder{name: "b", events: &events, started: &started, total: 3},
		validatorNodeRecorder{name: "c", events: &events, started: &started, total: 3},
	}

	require.NoError(t, startValidatorSet(nodes))
	require.Equal(t, []string{
		"start:a",
		"start:b",
		"start:c",
		"wait:a:2",
		"wait:b:2",
		"wait:c:2",
	}, events)
}

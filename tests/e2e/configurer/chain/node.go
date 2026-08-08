package chain

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/classic-terra/core/v4/tests/e2e/containers"
	"github.com/classic-terra/core/v4/tests/e2e/initialization"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/stretchr/testify/require"
)

type NodeConfig struct {
	Name        string
	ConfigDir   string
	Mnemonic    string
	PublicKey   string
	PeerID      string
	IsValidator bool

	OperatorAddress  string
	ConsensusAddress string // bech32 terravalcons... format
	SnapshotInterval uint64
	chainID          string
	rpcClient        *rpchttp.HTTP
	t                *testing.T
	containerManager *containers.Manager

	// Add this to help with logging / tracking time since start.
	setupTime time.Time
}

// NewNodeConfig returens new initialized NodeConfig.
func NewNodeConfig(t *testing.T, initNode *initialization.Node, initConfig *initialization.NodeConfig, chainID string, containerManager *containers.Manager) *NodeConfig {
	return &NodeConfig{
		Name:             initNode.Name,
		ConfigDir:        initNode.ConfigDir,
		Mnemonic:         initNode.Mnemonic,
		PublicKey:        initNode.PublicKey,
		PeerID:           initNode.PeerID,
		IsValidator:      initNode.IsValidator,
		SnapshotInterval: initConfig.SnapshotInterval,
		chainID:          chainID,
		containerManager: containerManager,
		t:                t,
		setupTime:        time.Now(),
	}
}

// Start creates the node container and RPC client without waiting for blocks.
// Multi-validator chains must start every container before any node waits for
// consensus, otherwise the first validator cannot reach the required quorum.
func (n *NodeConfig) Start() error {
	n.t.Logf("starting node container: %s", n.Name)
	resource, err := n.containerManager.RunNodeResource(n.Name, n.ConfigDir)
	if err != nil {
		return err
	}

	hostPort := resource.GetHostPort("26657/tcp")
	rpcClient, err := rpchttp.New("tcp://"+hostPort, "/websocket")
	if err != nil {
		return err
	}

	n.rpcClient = rpcClient
	return nil
}

// WaitForStartup verifies RPC, P2P connectivity and block production after all
// validators have been started.
func (n *NodeConfig) WaitForStartup(expectedPeers int) error {
	if n.rpcClient == nil {
		return fmt.Errorf("node %s has not been started", n.Name)
	}

	if err := pollNodeCondition(initialization.TwoMin, time.Second, func() (bool, error) {
		_, err := n.QueryCurrentHeight()
		return err == nil, err
	}); err != nil {
		return fmt.Errorf("node %s RPC did not become ready: %w", n.Name, err)
	}

	if expectedPeers > 0 {
		if err := pollNodeCondition(initialization.TwoMin, time.Second, func() (bool, error) {
			netInfo, err := n.rpcClient.NetInfo(context.Background())
			if err != nil {
				return false, err
			}
			return netInfo.NPeers >= expectedPeers, nil
		}); err != nil {
			return fmt.Errorf("node %s did not connect to %d peers: %w", n.Name, expectedPeers, err)
		}
	}

	if err := pollNodeCondition(initialization.TwoMin, time.Second, func() (bool, error) {
		height, err := n.QueryCurrentHeight()
		return err == nil && height > 0, err
	}); err != nil {
		return fmt.Errorf("node %s failed to produce its first block: %w", n.Name, err)
	}

	n.t.Logf("started node container: %s", n.Name)

	// Wait for 2 more blocks to confirm p2p connections are established.
	// Without this, a just-restarted node may not yet have peers and any
	// tx broadcast to it would sit in the local mempool and never be committed.
	firstHeight, _ := n.QueryCurrentHeight()
	if firstHeight > 0 {
		if err := pollNodeCondition(initialization.TwoMin, time.Second, func() (bool, error) {
			height, err := n.QueryCurrentHeight()
			return err == nil && height >= firstHeight+2, err
		}); err != nil {
			return fmt.Errorf("node %s failed to advance two blocks after start: %w", n.Name, err)
		}
	}

	return n.extractOperatorAddressIfValidator()
}

// Run starts and waits for one node. It is retained for nodes restarted after
// the rest of their validator set is already producing blocks.
func (n *NodeConfig) Run() error {
	if err := n.Start(); err != nil {
		return err
	}
	return n.WaitForStartup(1)
}

func pollNodeCondition(timeout, interval time.Duration, condition func() (bool, error)) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		done, err := condition()
		if done {
			return nil
		}
		if err != nil {
			lastErr = err
		}
		time.Sleep(interval)
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("condition not met within %s", timeout)
}

// Stop stops the node from running and removes its container.
func (n *NodeConfig) Stop() error {
	n.t.Logf("stopping node container: %s", n.Name)
	if err := n.containerManager.RemoveNodeResource(n.Name); err != nil {
		return err
	}
	n.t.Logf("stopped node container: %s", n.Name)
	return nil
}

// WaitUntil waits until node reaches doneCondition. Return nil
// if reached, error otherwise.
func (n *NodeConfig) WaitUntil(doneCondition func(syncInfo coretypes.SyncInfo) bool) {
	var latestBlockHeight int64
	for i := 0; i < waitUntilrepeatMax; i++ {
		status, err := n.rpcClient.Status(context.Background())
		require.NoError(n.t, err)
		latestBlockHeight = status.SyncInfo.LatestBlockHeight
		// let the node produce a few blocks
		if !doneCondition(status.SyncInfo) {
			time.Sleep(waitUntilRepeatPauseTime)
			continue
		}
		return
	}
	n.t.Errorf("node %s timed out waiting for condition, latest block height was %d", n.Name, latestBlockHeight)
}

func (n *NodeConfig) extractOperatorAddressIfValidator() error {
	if !n.IsValidator {
		n.t.Logf("node (%s) is not a validator, skipping", n.Name)
		return nil
	}

	cmd := []string{"terrad", "debug", "addr", n.PublicKey}
	n.t.Logf("extracting validator operator addresses for validator: %s", n.Name)
	outBuf, _, err := n.containerManager.ExecCmd(n.t, n.Name, cmd, "", false)
	if err != nil {
		return err
	}
	out := outBuf.String()

	reOper := regexp.MustCompile("terravaloper(.{39})")
	operAddr := fmt.Sprintf("%s\n", reOper.FindString(out))
	n.OperatorAddress = strings.TrimSuffix(operAddr, "\n")

	// The consensus address is derived from the ed25519 consensus key, which is
	// different from the secp256k1 account/operator key fed to "debug addr".
	// Use "comet show-address" to read it directly from the node's local keyfiles.
	showAddrCmd := []string{"terrad", "comet", "show-address"}
	showAddrBuf, _, err := n.containerManager.ExecCmd(n.t, n.Name, showAddrCmd, "", false)
	if err != nil {
		return err
	}
	n.ConsensusAddress = strings.TrimSpace(showAddrBuf.String())

	return nil
}

func (n *NodeConfig) GetHostPort(portID string) (string, error) {
	return n.containerManager.GetHostPort(n.Name, portID)
}

func (n *NodeConfig) WithSetupTime(t time.Time) *NodeConfig {
	n.setupTime = t
	return n
}

func (n *NodeConfig) LogActionF(msg string, args ...interface{}) {
	timeSinceStart := time.Since(n.setupTime).Round(time.Millisecond)
	s := fmt.Sprintf(msg, args...)
	n.t.Logf("[%s] %s. From container %s", timeSinceStart, s, n.Name)
}

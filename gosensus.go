package gosensus

import (
	"context"
	"encoding/hex"
	"errors"
	"go.etcd.io/etcd/v3/clientv3"
	"go.uber.org/zap"
	"sync"
	"time"
)

type Client struct {
	EtcdClient    *clientv3.Client
	Logger        *zap.Logger
	DataDir       string    // the directory in which the node key is stored
	quitElection  chan bool // channel to stop the election loop
	quitKeepAlive chan bool // channel to stop keeping the etcd node entry alive
	nodeId        string    // the node id of this node that is submitted to etcd for the leader election process
	_isLeader     bool      // _isLeader defines if this node is currently a leader. This variable should not be used. Use the concurrency safe IsLeader() method instead
	_isLeaderSync *sync.RWMutex
}

// Start initializes the consensus algorithm
func (c *Client) Start() error {
	if c.DataDir == "" {
		return errors.New("no data dir specified")
	}

	c._isLeader = false
	c._isLeaderSync = new(sync.RWMutex)
	c.quitElection = make(chan bool)
	c.quitKeepAlive = make(chan bool)
	c.Logger.Info("starting gosensus...")

	// check if a node key exists and generate one if not
	keyExists, err := nodeKeyExists(c.DataDir)
	if err != nil {
		return err
	}

	if !keyExists {
		c.Logger.Info("no node key found. Generating a new one...")
		if err := generateNodeKey(c.DataDir); err != nil {
			return err
		}
		c.Logger.Info("successfully generated a new node key")
	}

	if err := registerNode(c); err != nil {
		return err
	}
	go c.leaderElectionLoop()
	return nil
}

// Stop stops gosensus from operating.
// After 5 seconds this node's entry in etcd will expire and this node will be completely removed from the consensus algorithm
func (c *Client) Stop() {
	c.quitElection <- true
	c.quitKeepAlive <- true
}

// NodeId returns the node id of this node
// The node id is based on the node key
func (c *Client) NodeId() string {
	return c.nodeId
}

// registerNode registers the node in the etcd cluster and keeps the entry alive
func registerNode(c *Client) error {
	c.Logger.Info("registering our node in etcd")

	nodeKey, err := getNodeKey(c.DataDir)
	if err != nil {
		return err
	}
	c.nodeId = hex.EncodeToString(nodeKey.PubKey[:16])

	c.Logger.Info("our node id is " + c.nodeId)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	resp, err := c.EtcdClient.Grant(ctx, 5)
	cancel()
	if err != nil {
		return err
	}

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	_, err = c.EtcdClient.Put(ctx, "node:"+c.nodeId, ".", clientv3.WithLease(resp.ID))
	cancel()
	if err != nil {
		return err
	}

	// keep the node id entry alive
	ctx, cancel = context.WithCancel(context.Background())
	ch, kaerr := c.EtcdClient.KeepAlive(ctx, resp.ID)
	if kaerr != nil {
		return err
	}

	// stop keeping the entry alive when the shutdown signal is received
	go func() {
		select {
		case <-c.quitKeepAlive:
			cancel()
		}
	}()

	// discard the keepalive response, make etcd library not complain
	go func() {
		for {
			select {
			case <-ch:
			case <-ctx.Done():
				return
			}
		}
	}()
	c.Logger.Info("registration complete")
	return nil
}

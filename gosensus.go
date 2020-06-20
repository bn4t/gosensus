package gosensus

import (
	"context"
	"encoding/hex"
	"errors"
	"go.etcd.io/etcd/v3/clientv3"
	"go.uber.org/zap"
	"time"
)

// the node id that is submitted to etcd for the leader election process
var NodeId string

type Client struct {
	EtcdClient clientv3.Client
	Logger     zap.Logger
	DataDir    string // the directory in which the node key is stored
	quit       chan bool
}

// Start initializes the consensus algorithm
func (c *Client) Start() error {
	if c.DataDir == "" {
		return errors.New("no data dir specified")
	}

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
	go leaderElectionLoop(c)
	return nil
}

// Stop stops gosensus from operating.
// After 5 seconds this node's entry in etcd will expire and this node will be completely removed from the consensus algorithm
func (c *Client) Stop() error {
	c.quit <- true
	return c.EtcdClient.Close()
}

// registerNode registers the node in the etcd cluster and keeps the entry alive
func registerNode(c *Client) error {
	c.Logger.Info("registering our node in etcd")

	nodeKey, err := getNodeKey(c.DataDir)
	if err != nil {
		return err
	}
	NodeId = hex.EncodeToString(nodeKey.PubKey[:16])

	c.Logger.Info("our node id is " + NodeId)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	resp, err := c.EtcdClient.Grant(ctx, 5)
	cancel()
	if err != nil {
		return err
	}

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	_, err = c.EtcdClient.Put(ctx, "node:"+NodeId, ".", clientv3.WithLease(resp.ID))
	cancel()
	if err != nil {
		return err
	}

	// keep the node id entry alive
	ctx = context.Background()
	ch, kaerr := c.EtcdClient.KeepAlive(ctx, resp.ID)
	if kaerr != nil {
		return err
	}

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

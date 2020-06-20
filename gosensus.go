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
var config Config

// Init initializes the consensus algorithm
func Init(conf Config) error {
	if conf.DataDir == "" {
		return errors.New("no data dir specified")
	}
	if len(conf.EtcdEndpoints) == 0 {
		return errors.New("no etcd endpoint(s) specified")
	}
	config = conf

	if err := initEtcdClient(conf.EtcdEndpoints); err != nil {
		return err
	}
	config.Logger.Info("successfully connected to etcd", zap.String("etcd-endpoint", etcdClient.ActiveConnection().Target()))

	// check if a node key exists and generate one if not
	keyExists, err := nodeKeyExists()
	if err != nil {
		return err
	}

	if !keyExists {
		config.Logger.Info("no node key found. Generating a new one...")
		if err := generateNodeKey(); err != nil {
			return err
		}
		config.Logger.Info("successfully generated a new node key")
	}

	if err := registerNode(); err != nil {
		return err
	}
	go leaderElectionLoop()
	return nil
}

// registerNode registers the node in the etcd cluster and keeps the entry alive
func registerNode() error {
	config.Logger.Info("registering our node in etcd")

	nodeKey, err := getNodeKey()
	if err != nil {
		return err
	}
	NodeId = hex.EncodeToString(nodeKey.PubKey[:16])

	config.Logger.Info("our node id is " + NodeId)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	resp, err := etcdClient.Grant(ctx, 5)
	cancel()
	if err != nil {
		return err
	}

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	_, err = etcdClient.Put(ctx, "node:"+NodeId, ".", clientv3.WithLease(resp.ID))
	cancel()
	if err != nil {
		return err
	}

	// keep the node id entry alive
	ctx = context.Background()
	ch, kaerr := etcdClient.KeepAlive(ctx, resp.ID)
	if kaerr != nil {
		return err
	}

	// discard the keepalive response, make etcd library not to complain
	go func() {
		for {
			select {
			case <-ch:
			case <-ctx.Done():
				return
			}
		}
	}()
	config.Logger.Info("registration complete")
	return nil
}

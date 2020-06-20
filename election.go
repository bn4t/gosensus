package gosensus

import (
	"context"
	"go.etcd.io/etcd/v3/clientv3"
	"go.uber.org/zap"
	"sort"
	"strings"
	"sync"
	"time"
)

// _isLeader defines if this node is currently a leader
// this variable may not be accessed from methods outside of this file
// use the concurrency safe IsLeader() method instead
var _isLeader = false
var _isLeaderSync = new(sync.Mutex)

// IsLeader returns whether the node is currently the leader.
// This method is concurrency safe.
// TODO: possible improvements: https://stackoverflow.com/a/52882045
func IsLeader() (leader bool) {
	_isLeaderSync.Lock()
	leader = _isLeader
	_isLeaderSync.Unlock()
	return
}

// leaderElectionLoop runs the leader election process every 5 seconds
func leaderElectionLoop(c *Client) {
	for {
		nodeIds, err := getAllNodeIds(c)
		if err != nil {
			c.Logger.Error("error while getting all node ids from etcd: ", zap.Error(err))
			return
		}

		// sort the node ids in lexicographical (increasing) order
		// the node id with the lowest lexicographical order, nodeIds[0], is the leader
		sort.Strings(nodeIds)

		_isLeaderSync.Lock()
		prevIsLeader := _isLeader
		if nodeIds[0] == c.nodeId {
			_isLeader = true
		} else {
			_isLeader = false
		}
		if !prevIsLeader == _isLeader {
			c.Logger.Info("leadership status changed.", zap.Bool("leader", _isLeader))
		}
		_isLeaderSync.Unlock()

		// exit loop upon receiving quit signal
		select {
		case <-c.quit:
			return
		default:
		}

		time.Sleep(5 * time.Second)
	}
}

// getAllNodeIds grabs all nodes from etcd and returns all the ids in a slice
// Important: this slice also contains the node id of this node
func getAllNodeIds(c *Client) ([]string, error) {
	nodeIds := make([]string, 0)

	// get all node ids
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	resp, err := c.EtcdClient.Get(ctx, "node:", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend))
	cancel()
	if err != nil {
		return nil, err
	}

	for _, ev := range resp.Kvs {
		if string(ev.Value) != "." {
			continue
		}

		s := strings.Split(string(ev.Key), ":")
		if len(s) != 2 {
			continue
		}

		nodeIds = append(nodeIds, s[1])
	}
	return nodeIds, nil
}

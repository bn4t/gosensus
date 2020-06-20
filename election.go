package gosensus

import (
	"context"
	"go.etcd.io/etcd/v3/clientv3"
	"go.uber.org/zap"
	"sort"
	"strings"
	"time"
)

// IsLeader returns whether the node is currently the leader.
func (c *Client) IsLeader() (leader bool) {
	c._isLeaderSync.RLock()
	leader = c._isLeader
	c._isLeaderSync.RUnlock()
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

		c._isLeaderSync.Lock()
		prevIsLeader := c._isLeader
		if nodeIds[0] == c.nodeId {
			c._isLeader = true
		} else {
			c._isLeader = false
		}
		if !prevIsLeader == c._isLeader {
			c.Logger.Info("leadership status changed.", zap.Bool("leader", c._isLeader))
		}
		c._isLeaderSync.Unlock()

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

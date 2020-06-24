package gosensus

import (
	"go.etcd.io/etcd/v3/clientv3"
	"go.uber.org/zap"
	"log"
	"os"
	"path"
	"testing"
	"time"
)

// This test requires a local etcd node/cluster
func TestGosensus(t *testing.T) {
	etcd, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"http://localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}

	zapLog, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal(err)
	}

	if err := os.MkdirAll(path.Join(os.TempDir(), "g1"), os.FileMode(0755)); err != nil {
		zapLog.Fatal("error while creating directory 'g1'", zap.Error(err))
	}
	if err := os.MkdirAll(path.Join(os.TempDir(), "g2"), os.FileMode(0755)); err != nil {
		cleanupTests()
		zapLog.Fatal("error while creating directory 'g2'", zap.Error(err))
	}

	g1 := Client{
		EtcdClient: etcd,
		Logger:     zapLog.Named("g1"),
		DataDir:    path.Join(os.TempDir(), "g1"),
	}
	g2 := Client{
		EtcdClient: etcd,
		Logger:     zapLog.Named("g2"),
		DataDir:    path.Join(os.TempDir(), "g2"),
	}

	if err := g1.Start(); err != nil {
		cleanupTests()
		zapLog.Fatal("error while starting node 'g1'", zap.Error(err))
	}
	if err := g2.Start(); err != nil {
		g1.Stop()
		cleanupTests()
		zapLog.Fatal("error while starting node 'g2'", zap.Error(err))
	}

	zapLog.Info("waiting for consensus to settle...")
	time.Sleep(10 * time.Second)

	if g1.NodeId() < g2.NodeId() && !g1.IsLeader() {
		g1.Stop()
		g2.Stop()
		cleanupTests()
		zapLog.Fatal("node 1 is not leader despite having the lexicographically lower node id. node id g1: " + g1.NodeId() + ", node id g2: " + g2.NodeId())
	}
	if g2.NodeId() < g1.NodeId() && !g2.IsLeader() {
		g1.Stop()
		g2.Stop()
		cleanupTests()
		zapLog.Fatal("node 2 is not leader despite having the lexicographically lower node id. node id g1: " + g1.NodeId() + ", node id g2: " + g2.NodeId())
	}

	// shutdown the current leader and check if the other node takes over
	if g1.IsLeader() {
		g1.Stop()
		zapLog.Info("shutdown current leader node node g1, waiting for consensus to settle again...")
		time.Sleep(15 * time.Second)

		if !g2.IsLeader() {
			g2.Stop()
			cleanupTests()
			zapLog.Fatal("node 2 is not leader despite being the only node in the cluster")
		}
		g2.Stop()
	} else {
		g2.Stop()
		zapLog.Info("shutdown current leader node node g2, waiting for consensus to settle again...")
		time.Sleep(15 * time.Second)

		if !g1.IsLeader() {
			g1.Stop()
			cleanupTests()
			zapLog.Fatal("node 1 is not leader despite being the only node in the cluster")
		}
		g1.Stop()
	}

	etcd.Close()
	cleanupTests()
}

func cleanupTests() {
	// cleanup
	if err := os.RemoveAll(path.Join(os.TempDir(), "g1")); err != nil {
		log.Print(err)
	}
	if err := os.RemoveAll(path.Join(os.TempDir(), "g2")); err != nil {
		log.Print(err)
	}
}

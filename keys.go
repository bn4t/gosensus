package gosensus

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/pem"
	"io/ioutil"
	"os"
	"path"
)

type nodeKey struct {
	PrivKey ed25519.PrivateKey
	PubKey  ed25519.PublicKey
	Id      string
}

// getNodeKey returns the node key
func getNodeKey(dataDir string) (nodeKey, error) {
	seedFile, err := ioutil.ReadFile(path.Join(dataDir, "gosensus_node_key.pem"))
	if err != nil {
		return nodeKey{}, err
	}

	// decode the pem
	seed, _ := pem.Decode(seedFile)
	privKey := ed25519.NewKeyFromSeed(seed.Bytes)

	return nodeKey{
		PrivKey: privKey,
		PubKey:  privKey.Public().(ed25519.PublicKey),
		Id:      seed.Headers["Key-ID"], // Get the key id from the PEM headers
	}, nil
}

// Generate a random seed with a length of 35 bytes.
// Use the first three bytes as the key-id.
// The other 32 bytes are used as the seed for the ed25519 key.
// 32 bytes is the default seed size of the golang ed25519 implementation.
func generateNodeKey(dataDir string) error {
	var seed [35]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return err
	}

	// open the key file
	keyFile, err := os.OpenFile(path.Join(dataDir, "gosensus_node_key.pem"),
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	// close the key file again
	defer (func() {
		err = keyFile.Close()
	})()
	if err != nil {
		return err
	}

	// encode the seed as pem
	err = pem.Encode(keyFile, &pem.Block{
		Type: "ED25519 PRIVATE KEY",
		Headers: map[string]string{
			// grab the first three bytes from the seed and use them as the key-id
			"Key-ID": "ed25519:" + base64.RawStdEncoding.EncodeToString(seed[:3]),
		},
		Bytes: seed[3:],
	})
	if err != nil {
		return err
	}
	return nil
}

// nodeKeyExists tries to open the node key and returns
func nodeKeyExists(dataDir string) (bool, error) {
	info, err := os.Stat(path.Join(dataDir, "gosensus_node_key.pem"))
	if err == nil {
		return !info.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

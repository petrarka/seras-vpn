package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"

	"seras-protocol/pkg/taiga/msg"
)

func main() {
	genClient := flag.Bool("client", false, "Generate client key pair")
	genNode := flag.Bool("node", false, "Generate node key pair")
	privKeyHex := flag.String("derive", "", "Derive public key from private key (hex)")
	flag.Parse()

	if *privKeyHex != "" {
		// Derive public key from private
		privBytes, err := hex.DecodeString(*privKeyHex)
		if err != nil || len(privBytes) != 32 {
			fmt.Println("Error: private key must be 64 hex characters")
			os.Exit(1)
		}
		var privKey msg.Key
		copy(privKey[:], privBytes)

		pubKey, err := msg.PublicKeyFromPrivate(privKey)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Private: %s\n", hex.EncodeToString(privKey[:]))
		fmt.Printf("Public:  %s\n", hex.EncodeToString(pubKey[:]))
		return
	}

	if *genClient {
		priv, pub, err := msg.GenerateKeyPair()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("# Client keys (add to .env.client)")
		fmt.Printf("PRIVATE_KEY=%s\n", hex.EncodeToString(priv[:]))
		fmt.Println()
		fmt.Println("# Add this to .env.node as CLIENT_PUBLIC_KEY")
		fmt.Printf("CLIENT_PUBLIC_KEY=%s\n", hex.EncodeToString(pub[:]))
		return
	}

	if *genNode {
		priv, pub, err := msg.GenerateKeyPair()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("# Node keys (add to .env.node)")
		fmt.Printf("NODE_PRIVATE_KEY=%s\n", hex.EncodeToString(priv[:]))
		fmt.Printf("NODE_PUBLIC_KEY=%s\n", hex.EncodeToString(pub[:]))
		fmt.Println()
		fmt.Println("# Add NODE_PUBLIC_KEY to .env.client")
		return
	}

	// Default: generate both
	fmt.Println("=== Generating new key pairs ===")
	fmt.Println()

	// Node keys
	nodePriv, nodePub, _ := msg.GenerateKeyPair()
	fmt.Println("# .env.node")
	fmt.Printf("NODE_PRIVATE_KEY=%s\n", hex.EncodeToString(nodePriv[:]))
	fmt.Printf("NODE_PUBLIC_KEY=%s\n", hex.EncodeToString(nodePub[:]))

	// Client keys
	clientPriv, clientPub, _ := msg.GenerateKeyPair()
	fmt.Printf("CLIENT_PUBLIC_KEY=%s\n", hex.EncodeToString(clientPub[:]))
	fmt.Println()

	fmt.Println("# .env.client")
	fmt.Printf("PRIVATE_KEY=%s\n", hex.EncodeToString(clientPriv[:]))
	fmt.Printf("NODE_PUBLIC_KEY=%s\n", hex.EncodeToString(nodePub[:]))
}

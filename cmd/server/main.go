package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"log"
	"os"

	"crypto/x509"

	"happy-place-2/internal/game"
	"happy-place-2/internal/maps"
	"happy-place-2/internal/server"
)

const (
	listenAddr = ":2222"
	hostKeyPath = "host_key"
	mapPath     = "assets/maps/town.json"
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	// Generate host key if it doesn't exist
	if err := ensureHostKey(hostKeyPath); err != nil {
		log.Fatalf("Host key error: %v", err)
	}

	// Load map
	var gameMap *maps.Map
	var err error

	gameMap, err = maps.LoadMap(mapPath)
	if err != nil {
		log.Printf("Could not load %s: %v — using default map", mapPath, err)
		gameMap = maps.DefaultMap()
	}
	log.Printf("Map loaded: %s (%dx%d)", gameMap.Name, gameMap.Width, gameMap.Height)

	// Create game world and loop
	world := game.NewWorld(gameMap)
	gameLoop := game.NewGameLoop(world)

	// Start game loop in background
	go gameLoop.Run()
	defer gameLoop.Stop()

	// Start SSH server (blocks)
	sshServer := server.NewSSHServer(listenAddr, hostKeyPath, gameLoop)
	log.Printf("Starting Happy Place 2 — connect with: ssh -p 2222 YourName@localhost")
	if err := sshServer.Start(); err != nil {
		log.Fatalf("SSH server error: %v", err)
	}
}

func ensureHostKey(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil // key already exists
	}

	log.Println("Generating new host key...")
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}

	keyBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}

	pemBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyBytes,
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	return pem.Encode(f, pemBlock)
}

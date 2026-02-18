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
	"happy-place-2/internal/render"
	"happy-place-2/internal/server"
)

const (
	defaultAddr = ":2222"
	hostKeyPath = "host_key"
	mapsDir     = "assets/maps"
	spritesDir  = "assets/sprites"
	defaultMap  = "Town Square"
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	// Generate host key if it doesn't exist
	if err := ensureHostKey(hostKeyPath); err != nil {
		log.Fatalf("Host key error: %v", err)
	}

	// Load all maps from directory
	allMaps, err := maps.LoadMaps(mapsDir)
	if err != nil {
		log.Printf("Could not load maps from %s: %v — using default map", mapsDir, err)
		dm := maps.DefaultMap()
		allMaps = map[string]*maps.Map{dm.Name: dm}
	}
	for name, m := range allMaps {
		log.Printf("Map loaded: %s (%dx%d, %d portals)", name, m.Width, m.Height, len(m.Portals))
	}

	// Load sprite registry
	sprites, err := render.NewSpriteRegistry(spritesDir)
	if err != nil {
		log.Fatalf("Failed to load sprites from %s: %v", spritesDir, err)
	}

	// Create game world and loop
	world := game.NewWorld(allMaps, defaultMap)
	gameLoop := game.NewGameLoop(world)

	// Start game loop in background
	go gameLoop.Run()
	defer gameLoop.Stop()

	// Start SSH server (blocks)
	listenAddr := defaultAddr
	if port := os.Getenv("PORT"); port != "" {
		listenAddr = ":" + port
	}
	sshServer := server.NewSSHServer(listenAddr, hostKeyPath, gameLoop, sprites)
	log.Printf("Starting Happy Place 2 — connect with: ssh -p %s YourName@localhost", listenAddr[1:])
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

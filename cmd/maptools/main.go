package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"happy-place-2/internal/maps"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "validate":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "Usage: maptools validate <maps-dir>")
			os.Exit(1)
		}
		os.Exit(runValidate(args[0]))
	case "viz":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "Usage: maptools viz <map-file>")
			os.Exit(1)
		}
		runViz(args[0])
	case "stats":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "Usage: maptools stats <map-file>")
			os.Exit(1)
		}
		runStats(args[0])
	case "all":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "Usage: maptools all <maps-dir>")
			os.Exit(1)
		}
		os.Exit(runAll(args[0]))
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: maptools <command> <path>

Commands:
  validate <maps-dir>   Validate all maps in directory
  viz      <map-file>   Render map as colored ASCII art
  stats    <map-file>   Show tile distribution and walkable %
  all      <maps-dir>   Run validate + viz + stats for all maps`)
}

// --- validate ---

func runValidate(dir string) int {
	allMaps, err := maps.LoadMaps(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		return 1
	}

	errors := 0
	for name, m := range allMaps {
		fmt.Printf("Validating %q...\n", name)

		// Check spawn is walkable
		if !m.IsWalkable(m.SpawnX, m.SpawnY) {
			fmt.Printf("  ERROR: spawn (%d,%d) is not walkable\n", m.SpawnX, m.SpawnY)
			errors++
		}

		// Check all tile indices are in legend range
		for y, row := range m.Tiles {
			for x, idx := range row {
				if idx < 0 || idx >= len(m.Legend) {
					fmt.Printf("  ERROR: tile (%d,%d) index %d out of legend range [0..%d]\n", x, y, idx, len(m.Legend)-1)
					errors++
				}
			}
		}

		// Check portal positions are in bounds and walkable
		for _, p := range m.Portals {
			if p.X < 0 || p.X >= m.Width || p.Y < 0 || p.Y >= m.Height {
				fmt.Printf("  ERROR: portal at (%d,%d) is out of bounds\n", p.X, p.Y)
				errors++
			}
			// Check target position is walkable in target map
			if tm, ok := allMaps[p.TargetMap]; ok {
				if p.TargetX < 0 || p.TargetX >= tm.Width || p.TargetY < 0 || p.TargetY >= tm.Height {
					fmt.Printf("  ERROR: portal at (%d,%d) targets out-of-bounds (%d,%d) in %q\n", p.X, p.Y, p.TargetX, p.TargetY, p.TargetMap)
					errors++
				} else if !tm.IsWalkable(p.TargetX, p.TargetY) {
					fmt.Printf("  ERROR: portal at (%d,%d) targets non-walkable tile (%d,%d) in %q\n", p.X, p.Y, p.TargetX, p.TargetY, p.TargetMap)
					errors++
				}
			}
		}

		if errors == 0 {
			fmt.Printf("  OK (%dx%d, %d portals)\n", m.Width, m.Height, len(m.Portals))
		}
	}

	if errors > 0 {
		fmt.Printf("\n%d error(s) found\n", errors)
		return 1
	}
	fmt.Printf("\nAll %d maps valid\n", len(allMaps))
	return 0
}

// --- viz ---

// ansiColor returns the ANSI escape for the given code.
func ansiColor(code int) string {
	return fmt.Sprintf("\033[%dm", code)
}

func runViz(path string) {
	m, err := maps.LoadMap(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s (%dx%d)\n", m.Name, m.Width, m.Height)

	for y := 0; y < m.Height; y++ {
		for x := 0; x < m.Width; x++ {
			tile := m.TileAt(x, y)
			fmt.Print(ansiColor(tile.Fg), string(tile.Char), "\033[0m")
		}
		fmt.Println()
	}

	// Mark spawn and portals in legend
	fmt.Printf("\nSpawn: (%d,%d)\n", m.SpawnX, m.SpawnY)
	for _, p := range m.Portals {
		fmt.Printf("Portal: (%d,%d) → %s (%d,%d)\n", p.X, p.Y, p.TargetMap, p.TargetX, p.TargetY)
	}
}

// --- stats ---

func runStats(path string) {
	m, err := maps.LoadMap(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s (%dx%d = %d tiles)\n\n", m.Name, m.Width, m.Height, m.Width*m.Height)

	// Count by tile name
	counts := make(map[string]int)
	walkable := 0
	total := m.Width * m.Height

	for y := 0; y < m.Height; y++ {
		for x := 0; x < m.Width; x++ {
			tile := m.TileAt(x, y)
			counts[tile.Name]++
			if tile.Walkable {
				walkable++
			}
		}
	}

	// Print sorted by count descending
	type entry struct {
		name  string
		count int
	}
	var sorted []entry
	for name, count := range counts {
		sorted = append(sorted, entry{name, count})
	}
	// Simple insertion sort
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].count > sorted[j-1].count; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

	for _, e := range sorted {
		pct := float64(e.count) / float64(total) * 100
		bar := strings.Repeat("█", int(pct/2))
		fmt.Printf("  %-10s %4d (%5.1f%%) %s\n", e.name, e.count, pct, bar)
	}

	fmt.Printf("\nWalkable: %d/%d (%.1f%%)\n", walkable, total, float64(walkable)/float64(total)*100)
	fmt.Printf("Portals:  %d\n", len(m.Portals))
}

// --- all ---

func runAll(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading directory: %v\n", err)
		return 1
	}

	// Run validate first
	fmt.Println("=== VALIDATE ===")
	code := runValidate(dir)
	if code != 0 {
		return code
	}

	// Then viz + stats for each map
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		fmt.Printf("\n=== VIZ: %s ===\n", entry.Name())
		runViz(path)
		fmt.Printf("\n=== STATS: %s ===\n", entry.Name())
		runStats(path)
	}

	return 0
}

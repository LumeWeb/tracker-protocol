// Command schemas generates JSON Schema files for all urma protocol types.
//
// Usage:
//
//	go run ./cmd/schemas [-out dir]
//
// Default output directory is schemas/.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"go.lumeweb.com/urma/sia"
)

func main() {
	outDir := "schemas"
	if len(os.Args) > 2 && os.Args[1] == "-out" {
		outDir = os.Args[2]
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "create dir: %v\n", err)
		os.Exit(1)
	}

	schemas, err := sia.WriteSchemas()
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate schemas: %v\n", err)
		os.Exit(1)
	}

	for name, data := range schemas {
		filename := filepath.Join(outDir, name+".json")
		if err := os.WriteFile(filename, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", filename, err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s (%d bytes)\n", filename, len(data))
	}
}

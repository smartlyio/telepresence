package main

import (
	"io"
	"log"
	"os"

	"sigs.k8s.io/yaml"
)

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err == nil {
		data, err = yaml.YAMLToJSON(data)
		if err == nil {
			_, err = os.Stdout.Write(data)
		}
	}
	if err != nil {
		log.Fatal(err)
	}
}

package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/josephvusich/zinfer/zfs"
)

func main() {
	pools, err := zfs.ImportedPools()
	if err != nil {
		log.Fatal(err)
	}

	for _, p := range pools {
		for _, d := range p.Datasets.Ordered {
			fmt.Println(strings.Join(d.CreateCommand(), " "))
		}
	}
}

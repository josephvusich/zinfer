package main

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/josephvusich/zinfer/zfs"
	"gopkg.in/alessio/shellescape.v1"
)

func main() {
	pools, err := zfs.ImportedPools()
	if err != nil {
		log.Fatal(err)
	}

	sortedPools := make([]string, 0, len(pools))
	for _, p := range pools {
		sortedPools = append(sortedPools, p.Name)
	}
	sort.Strings(sortedPools)

	for _, poolName := range sortedPools {
		p := pools[poolName]
		cmd, err := p.CreatePoolCommand()
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(joinCommand(cmd))

		for i, d := range p.Datasets.Ordered {
			if i == 0 {
				continue
			}

			cmd, err = p.CreateDatasetCommand(d.Name)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Println(joinCommand(cmd))
		}
	}
}

func joinCommand(cmd []string) string {
	return strings.Join(cmd, " ")
}

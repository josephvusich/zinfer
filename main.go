package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/josephvusich/go-getopt"
	"github.com/josephvusich/zinfer/zfs"
	"gopkg.in/alessio/shellescape.v1"
)

func main() {
	minimalFeatures := flag.Bool("minimal-features", false, "omit enabled pool features that are not currently active")
	help := flag.Bool("help", false, "show this help message")
	if err := getopt.CommandLine.Parse(os.Args[1:]); err != nil {
		log.Fatal(err)
	}

	if *help {
		fmt.Fprintln(flag.CommandLine.Output(), "usage: zinfer [--minimal-features]")
		getopt.PrintDefaults()
		os.Exit(0)
	}

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
		cmd, err := p.CreatePoolCommand(&zfs.FlagOptions{MinimalFeatures: *minimalFeatures})
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(escapeCommand(cmd))

		for i, d := range p.Datasets.Ordered {
			if i == 0 {
				continue
			}

			cmd, err = p.CreateDatasetCommand(d.Name)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Println(escapeCommand(cmd))
		}
	}
}

func escapeCommand(cmd []string) string {
	for i := range cmd {
		cmd[i] = shellescape.Quote(cmd[i])
	}
	return strings.Join(cmd, " ")
}

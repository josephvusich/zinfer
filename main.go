package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/josephvusich/go-getopt"
	"github.com/josephvusich/go-zfs"
	"gopkg.in/alessio/shellescape.v1"
)

func main() {
	log.SetFlags(0)

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

	requested := map[string]struct{}{}
	for _, name := range flag.Args() {
		requested[name] = struct{}{}
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

	printed := 0
	print := func(p *zfs.Pool, name string, isPool bool) {
		if _, ok := requested[name]; !ok && flag.NArg() != 0 {
			return
		} else if ok {
			delete(requested, name)
		}
		if printed != 0 {
			fmt.Print("\n")
		}
		printed++
		var cmd []string
		var err error
		if isPool {
			cmd, err = p.CreatePoolCommand(&zfs.FlagOptions{MinimalFeatures: *minimalFeatures})
		} else {
			cmd, err = p.CreateDatasetCommand(name)
		}
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(escapeCommand(cmd))
	}

	for _, poolName := range sortedPools {
		p := pools[poolName]

		print(p, poolName, true)

		for i, d := range p.Datasets.Ordered {
			if i == 0 {
				continue
			}

			print(p, d.Name, false)
		}
	}

	if len(requested) != 0 {
		if printed != 0 {
			fmt.Print("\n")
		}
		for missing := range requested {
			fmt.Fprintf(os.Stderr, "filesystem not found: %s\n", missing)
		}
	}
}

var oPattern = regexp.MustCompile(`^-[oO]$`)

func escapeCommand(cmd []string) string {
	for i := range cmd {
		cmd[i] = shellescape.Quote(cmd[i])
		if oPattern.MatchString(cmd[i]) || len(cmd)-1 == i {
			cmd[i] = "\\\n  " + cmd[i]
		}
	}
	return strings.Join(cmd, " ")
}

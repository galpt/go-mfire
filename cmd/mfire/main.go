package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/galpt/go-mfire/pkg/mfire"
)

func main() {
	client := mfire.NewClient()
	reader := bufio.NewReader(os.Stdin)

	for {
		// Fetch and display top 10 on each loop so the user sees fresh results.
		mangas, err := client.FetchHome(10)
		if err != nil {
			fmt.Printf("Error fetching home: %v\n", err)
			return
		}

		fmt.Println("======")
		fmt.Println("------")
		fmt.Println("-- Simple MangaFire Parser Written in Go --")
		fmt.Println("------")
		for i, m := range mangas {
			fmt.Printf("%d) %s\n", i+1, m.Title)
		}
		fmt.Println("search) Search by name")
		fmt.Println("exit) Exit this CLI")
		fmt.Println("------")
		fmt.Print("> ")

		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "exit" {
			fmt.Println("bye")
			return
		}
		if line == "search" {
			fmt.Print("query: ")
			q, _ := reader.ReadString('\n')
			q = strings.TrimSpace(q)
			if q == "" {
				continue
			}
			results, err := client.Search(q, 10)
			if err != nil {
				fmt.Printf("search error: %v\n", err)
				continue
			}
			fmt.Printf("Results for '%s':\n", q)
			for i, r := range results {
				fmt.Printf("%d) %s -- %s\n", i+1, r.Title, r.Url)
			}
			fmt.Println("press Enter to continue...")
			reader.ReadString('\n')
			continue
		}

		if n, err := strconv.Atoi(line); err == nil {
			if n >= 1 && n <= len(mangas) {
				m := mangas[n-1]
				fmt.Printf("%s\nURL: %s\n", m.Title, m.Url)
				fmt.Println("press Enter to continue...")
				reader.ReadString('\n')
				continue
			}
			fmt.Println("invalid number")
			continue
		}

		fmt.Println("unknown command")
	}
}

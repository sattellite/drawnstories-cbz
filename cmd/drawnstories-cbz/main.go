package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

func main() {
	// get address of the comic books page
	address, err := comicsPage(os.Args)
	if err != nil {
		fmt.Println(err.Error())
		usage()
		os.Exit(1)
	}
	fmt.Println(address)

	// parse passed URL page and get all books from it
	// download pages of each book
	// create cbz archive from downloaded pages
}

func comicsPage(args []string) (*url.URL, error) {
	// required cli parameter is URL to the comic book from site drawnstories.ru
	if len(args) < 2 {
		return nil, fmt.Errorf("URL to the comic book is required")
	}
	arg := args[1]
	// check if passed URL is valid
	address, urlErr := url.Parse(arg)
	if urlErr != nil {
		return nil, fmt.Errorf("invalid URL passed: %w", urlErr)
	}

	// check if passed URL is valid
	if address.Host == "" {
		return nil, fmt.Errorf("invalid URL passed: %s", arg)
	}

	// check if passed URL is from drawnstories.ru
	if address.Host != "drawnstories.ru" {
		return nil, fmt.Errorf("unsupported site: %s", address.Host)
	}

	// check that passed URL is comic book page
	if !strings.HasPrefix(address.Path, "/comics/") {
		return nil, fmt.Errorf("not a comic book page")
	}

	return address, nil
}

func usage() {
	fmt.Println("Usage: drawnstories-cbz <URL>")
	fmt.Println("Example: drawnstories-cbz https://drawnstories.ru/comics/Oni-press/rick-and-morty")
}

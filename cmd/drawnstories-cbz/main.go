package main

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func main() {
	// get address of the comic books page
	address, err := comicsPage(os.Args)
	if err != nil {
		fmt.Println(err.Error())
		usage()
		os.Exit(1)
	}

	// parse passed URL page and get all books from it
	books, bErr := getBooks(address.String())
	if bErr != nil {
		fmt.Println(bErr.Error())
		os.Exit(1)
	}

	// if passed specified book number
	if len(os.Args) > 2 {
		// find specified book
		book := make(map[string][]string, 1)
		suffix := fmt.Sprintf("-%s", os.Args[2])
		for name, pages := range books {
			if strings.HasSuffix(name, suffix) {
				book[name] = pages
				break
			}
		}
		books = book
	}

	// make each book
	cbzErr := makeCbz(books)
	if cbzErr != nil {
		fmt.Println(cbzErr.Error())
		os.Exit(1)
	}
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

func getBooks(address string) (map[string][]string, error) {
	resp, rErr := http.Get(address)
	if rErr != nil {
		return nil, fmt.Errorf("failed to get page: %w", rErr)
	}
	defer func(c io.Closer) {
		_ = c.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get page: %s", resp.Status)
	}
	if resp.Body == nil {
		return nil, fmt.Errorf("empty page")
	}

	// parse passed URL page and get all books from it
	doc, dErr := goquery.NewDocumentFromReader(resp.Body)
	if dErr != nil {
		return nil, fmt.Errorf("failed to parse page: %w", dErr)
	}

	books := make(map[string][]string)
	doc.Find("a.fancybox").Each(func(i int, s *goquery.Selection) {
		if ref, ok := s.Attr("href"); ok && ref != "" {
			bookNameParts := strings.Split(ref, "/")
			bookName := bookNameParts[len(bookNameParts)-2]
			if _, ok := books[bookName]; !ok {
				books[bookName] = make([]string, 0)
			}
			books[bookName] = append(books[bookName], ref)
		}
	})
	if len(books) == 0 {
		return nil, fmt.Errorf("no books found")
	}

	return books, nil
}

func makeCbz(books map[string][]string) error {
	for bookName, pages := range books {
		if len(pages) == 0 {
			continue
		}

		fmt.Printf("Processing book %s\n", bookName)
		dir, dErr := os.MkdirTemp("", bookName)
		if dErr != nil {
			return fmt.Errorf("failed to create temp dir: %w", dErr)
		}
		defer func(dirPath string) {
			rErr := os.RemoveAll(dirPath)
			if rErr != nil {
				fmt.Printf("failed to remove temp dir %q: %s", dirPath, rErr.Error())
			}
		}(dir)
		fmt.Println("Created temp dir", dir)

		for _, page := range pages {
			fmt.Printf("Download page %s\n", page)
			resp, err := http.Get(page)
			if err != nil {
				return fmt.Errorf("failed to get page: %w", err)
			}
			defer func(c io.Closer) {
				_ = c.Close()
			}(resp.Body)

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("failed to get page: %s", resp.Status)
			}

			if resp.Body == nil {
				return fmt.Errorf("empty page")
			}

			fileNameParts := strings.Split(page, "/")
			fileName := fileNameParts[len(fileNameParts)-1]

			// save image to temp dir
			f, fErr := os.Create(filepath.Join(dir, fileName))
			if fErr != nil {
				return fmt.Errorf("failed to create file: %w", fErr)
			}
			_, _ = io.Copy(f, resp.Body)
			_ = f.Close()
		}
		// create zip archive
		zipName := fmt.Sprintf("%s.cbz", bookName)
		zipErr := archiver(dir, zipName)
		if zipErr != nil {
			return fmt.Errorf("failed to create zip archive: %w", zipErr)
		}

	}
	return nil
}

func archiver(dir, zipName string) error {
	// make zip archive
	zipFile, zErr := os.Create(zipName)
	if zErr != nil {
		return fmt.Errorf("failed to create zip archive: %w", zErr)
	}
	defer func(c io.Closer) {
		_ = c.Close()
	}(zipFile)

	zipWriter := zip.NewWriter(zipFile)
	defer func(c io.Closer) {
		_ = c.Close()
	}(zipWriter)

	// get all files from temp dir
	files, fErr := os.ReadDir(dir)
	if fErr != nil {
		return fmt.Errorf("failed to read dir: %w", fErr)
	}

	// add files to zip archive
	for _, file := range files {
		fileName := file.Name()
		filePath := filepath.Join(dir, fileName)
		fileInfo, _ := file.Info()
		fileHeader, _ := zip.FileInfoHeader(fileInfo)
		fileHeader.Name = fileName
		fileHeader.Method = zip.Deflate
		writer, _ := zipWriter.CreateHeader(fileHeader)
		f, _ := os.Open(filePath)
		defer func(c io.Closer) {
			_ = c.Close()
		}(f)
		_, _ = io.Copy(writer, f)
	}
	return nil
}

func usage() {
	fmt.Println("Usage: drawnstories-cbz <URL>")
	fmt.Println("Example: drawnstories-cbz https://drawnstories.ru/comics/Oni-press/rick-and-morty")
}

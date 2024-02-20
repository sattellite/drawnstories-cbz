package main

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	p := tea.NewProgram(downloaderModel())
	d := newDownloader(p)
	go d.Run(os.Args)

	if _, err := p.Run(); err != nil {
		fmt.Printf("something went wrong: %v", err)
		os.Exit(1)
	}
}

type book struct {
	Name      string
	Pages     []string
	Publisher string
	Issue     int
}

type pageMsg string
type bookMsg string
type quitMsg struct{}
type errMsg error

type model struct {
	spinner   spinner.Model
	helpStyle lipgloss.Style
	err       error

	page     string
	books    []string
	quitting bool
	done     bool
}

func downloaderModel() *model {
	s := spinner.New()
	s.Spinner = spinner.Ellipsis

	m := &model{
		spinner:   s,
		helpStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Margin(1, 0),
	}

	return m
}

func (m *model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.quitting && m.done {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	// Is it a key press?
	case tea.KeyMsg:
		// Cool, what was the actual key pressed?
		switch msg.Type {
		// These keys should exit the program.
		case tea.KeyCtrlC, tea.KeyEscape:
			return m, tea.Quit
		}
	case pageMsg:
		m.page = string(msg)
	case bookMsg:
		m.books = append(m.books, string(msg))
	case quitMsg:
		m.quitting = true
		m.done = true
		return m, nil
	case errMsg:
		m.err = msg
		m.quitting = true
		return m, nil
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *model) View() string {
	if m.quitting && m.err != nil {
		m.done = true
		s := fmt.Sprintf("ERROR: %v\n\n", m.err)
		s += m.usage()
		return s
	}

	processing := true
	booksCount := len(m.books)

	s := fmt.Sprintf("Searching comics on %q", m.page)
	if booksCount > 0 {
		s += " ✔"
	}

	for idx, book := range m.books {
		if book == "__done__" {
			processing = false
			continue
		}
		s += fmt.Sprintf("\nDownloading book %q", book)
		if idx < booksCount-1 {
			s += " ✔"
		}
	}

	if processing {
		s += " " + m.spinner.View()
		s += m.helpStyle.Render("Press Ctrl+C or Esc to quit.")
	} else {
		s += "\n"
	}

	return s
}

func (d *model) usage() string {
	s := fmt.Sprintln("Usage: drawnstories-cbz <URL> [book numbers]")
	s += fmt.Sprintln("Example: drawnstories-cbz https://drawnstories.ru/comics/Oni-press/rick-and-morty 001")
	return s
}

func newDownloader(p *tea.Program) downloader {
	return downloader{
		prog:  p,
		mutex: new(sync.RWMutex),
	}
}

type downloader struct {
	prog *tea.Program

	mutex *sync.RWMutex
}

func (d *downloader) Run(args []string) {
	// get address of the comic books page
	address, err := d.comicsPage(args)
	if err != nil {
		d.prog.Send(errMsg(err))
		return
	}
	d.prog.Send(pageMsg(address.String()))

	// parse passed URL page and get all books from it
	books, bErr := d.getBooks(address.String())
	if bErr != nil {
		d.prog.Send(errMsg(bErr))
		return
	}

	// if passed specified book numbers
	if len(args) > 2 {
		// find specified books
		bookList := make(map[string]book)
		suffixes := make([]string, 0)
		for _, num := range args[2:] {
			suffixes = append(suffixes, fmt.Sprintf("-%s", num))
		}
		for name, book := range books {
			for _, s := range suffixes {
				if strings.HasSuffix(name, s) {
					bookList[name] = book
				}
			}
		}
		books = bookList
	}

	// make each book
	cbzErr := d.makeCbz(books)
	if cbzErr != nil {
		d.prog.Send(errMsg(cbzErr))
		return
	}
	d.prog.Send(bookMsg("__done__"))
	d.prog.Send(quitMsg{})
}

func (d *downloader) comicsPage(args []string) (*url.URL, error) {
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

func (d *downloader) getBooks(address string) (map[string]book, error) {
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

	bookPublisher := strings.ReplaceAll(strings.Split(address, "/")[4], "-", " ")

	// parse passed URL page and get all books from it
	doc, dErr := goquery.NewDocumentFromReader(resp.Body)
	if dErr != nil {
		return nil, fmt.Errorf("failed to parse page: %w", dErr)
	}

	books := make(map[string]book)
	doc.Find("a.fancybox").Each(func(i int, s *goquery.Selection) {
		if ref, ok := s.Attr("href"); ok && ref != "" {
			bookNameParts := strings.Split(ref, "/")
			bookName := bookNameParts[len(bookNameParts)-2]
			d.mutex.Lock()
			b, ok := books[bookName]
			if !ok {
				b = book{
					Pages:     make([]string, 0),
					Publisher: bookPublisher,
				}
				issueParts := strings.Split(bookName, "-")
				if len(issueParts) > 1 {
					if _, err := fmt.Sscanf(issueParts[len(issueParts)-1], "%d", &b.Issue); err != nil {
						fmt.Printf("failed to parse issue number: %s", err.Error())
					}
				}
				if title, tok := s.Attr("title"); tok {
					b.Name = title
				}
				books[bookName] = b
			}
			b.Pages = append(b.Pages, ref)
			books[bookName] = b
			d.mutex.Unlock()
		}
	})
	if len(books) == 0 {
		return nil, fmt.Errorf("no books found")
	}

	return books, nil
}

func (d *downloader) makeCbz(books map[string]book) error {
	orderedList := make([]string, 0)
	for bookName := range books {
		orderedList = append(orderedList, bookName)
	}
	// sort books by name
	sort.Strings(orderedList)
	for _, bookName := range orderedList {
		b := books[bookName]
		if len(b.Pages) == 0 {
			continue
		}

		//fmt.Printf("Processing book %s\n", bookName)
		d.prog.Send(bookMsg(bookName))
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

		for _, page := range b.Pages {
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
		// generate book description
		descErr := d.generateDescription(dir, b)
		if descErr != nil {
			return fmt.Errorf("failed to create description: %w", descErr)
		}

		// create zip archive
		zipName := fmt.Sprintf("%s.cbz", bookName)
		zipErr := d.archiver(dir, zipName)
		if zipErr != nil {
			return fmt.Errorf("failed to create zip archive: %w", zipErr)
		}
	}
	return nil
}

func (d *downloader) generateDescription(dir string, b book) error {
	// create CoMet.xml file
	cometErr := d.createCometFile(dir, b)
	if cometErr != nil {
		return fmt.Errorf("failed to create comet file: %w", cometErr)
	}
	comicInfoErr := d.createComicInfoFile(dir, b)
	if comicInfoErr != nil {
		return fmt.Errorf("failed to create comic info file: %w", comicInfoErr)
	}
	return nil
}

func (d *downloader) createCometFile(dir string, b book) error {
	// write xml declaration
	cometFile, cErr := os.Create(filepath.Join(dir, "CoMet.xml"))
	if cErr != nil {
		return fmt.Errorf("failed to create comet file: %w", cErr)
	}
	defer func(c io.Closer) {
		_ = c.Close()
	}(cometFile)

	// information about comet found here https://github.com/geometer/FBReaderJ/issues/329
	// comet specification https://www.denvog.com/comet/comet-specification/
	type comet struct {
		XMLName   xml.Name `xml:"comet"`
		XMLNS     string   `xml:"xmlns:comet,attr"`
		XSI       string   `xml:"xmlns:xsi,attr"`
		Schema    string   `xml:"xsi:schemaLocation,attr"`
		Title     string   `xml:"title"`
		Issue     int      `xml:"issue"`
		Publisher string   `xml:"publisher"`
		Pages     int      `xml:"pages"`
		Format    string   `xml:"format"`
	}

	info := comet{
		XMLNS:     "http://www.denvog.com/comet/",
		XSI:       "http://www.w3.org/2001/XMLSchema-instance",
		Schema:    "http://www.denvog.com http://www.denvog.com/comet/comet.xsd",
		Title:     b.Name,
		Format:    "Comic",
		Issue:     b.Issue,
		Publisher: b.Publisher,
		Pages:     len(b.Pages),
	}
	_, hErr := cometFile.WriteString(xml.Header)
	if hErr != nil {
		return fmt.Errorf("failed to write to comet file: %w", hErr)
	}
	enc := xml.NewEncoder(cometFile)
	enc.Indent("", "  ")
	if err := enc.Encode(info); err != nil {
		return fmt.Errorf("failed to encode comet file: %w", err)
	}
	return nil
}

func (d *downloader) createComicInfoFile(dir string, b book) error {
	// write xml declaration
	cometFile, cErr := os.Create(filepath.Join(dir, "ComicInfo.xml"))
	if cErr != nil {
		return fmt.Errorf("failed to create comic info file: %w", cErr)
	}
	defer func(c io.Closer) {
		_ = c.Close()
	}(cometFile)

	// information about comet found here https://github.com/geometer/FBReaderJ/issues/329
	// Some comic info description here https://wiki.mobileread.com/wiki/ComicRack
	type comicInfo struct {
		XMLName   xml.Name `xml:"ComicInfo"`
		XSI       string   `xml:"xmlns:xsi,attr"`
		XSD       string   `xml:"xmlns:xsd,attr"`
		Title     string   `xml:"Title"`
		Publisher string   `xml:"Publisher"`
		Number    int      `xml:"Number"`
		PageCount int      `xml:"PageCount"`
	}

	info := comicInfo{
		XSI:       "http://www.w3.org/2001/XMLSchema-instance",
		XSD:       "http://www.w3.org/2001/XMLSchema",
		Title:     b.Name,
		Publisher: b.Publisher,
		Number:    b.Issue,
		PageCount: len(b.Pages),
	}
	_, hErr := cometFile.WriteString(xml.Header)
	if hErr != nil {
		return fmt.Errorf("failed to write to comic info file: %w", hErr)
	}
	enc := xml.NewEncoder(cometFile)
	enc.Indent("", "  ")
	if err := enc.Encode(info); err != nil {
		return fmt.Errorf("failed to encode comic info file: %w", err)
	}
	return nil
}

func (d *downloader) archiver(dir, zipName string) error {
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

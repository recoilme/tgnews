package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/PuerkitoBio/goquery"
	"github.com/abadojack/whatlanggo"
)

func main() {
	println()
	println("-- tgnews --")
	args := os.Args
	cmd := "languages"
	dir := "data"
	if len(args) >= 3 {
		cmd = args[1]
		dir = args[2]
	}
	println("cmd:", cmd, "dir:", dir)
	switch cmd {
	default:
		lang(dir)
	case "news":
		news(dir)
	}
}

func lang(dir string) {
	info := whatlanggo.Detect("Foje funkcias kaj foje ne funkcias")
	fmt.Println("Language:", info.Lang.String())
	list, err := filePathWalkDir(dir)
	if err != nil {
		panic(err)
	}
	println("files:", len(list))
	eng := make([]string, 0, 0)
	rus := make([]string, 0, 0)
	for _, f := range list {
		_ = f
		title, desc, err := header(f)
		if err != nil || title == "" {
			println("skiped:", f, title, desc, err)
			continue
		}
		//println("file:", f, title, desc, err)
		info := whatlanggo.Detect(title)
		lang := info.Lang.String()
		switch lang {
		case "English":
			eng = append(eng, f)
		case "Russian":
			rus = append(rus, f)
		default:
			//println("not detected", lang)
		}
	}
	println("eng:", len(eng), "rus:", len(rus))
}

func news(dir string) {

}

func header(file string) (title, desc string, err error) {
	f, err := os.Open(file)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	doc, _ := goquery.NewDocumentFromReader(f)
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		op, _ := s.Attr("property")
		con, _ := s.Attr("content")
		if op == "og:title" {
			title = con
		}
		if op == "og:description" {
			desc = con
		}
	})
	return
}

func filePathWalkDir(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

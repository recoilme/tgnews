package main

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/abadojack/whatlanggo"
	"github.com/recoilme/pudge"
)

func main() {
	println()
	println("-- tgnews --")
	args := os.Args
	cmd := "languages"
	dir := "data"
	if len(args) >= 2 {
		cmd = args[1]
	}
	if len(args) >= 3 {
		dir = args[2]
	}
	println("cmd:", cmd, "dir:", dir)
	t1 := time.Now()
	switch cmd {
	default:
		lang(dir)
	case "news":
		news(dir)
	case "test":
		test(dir)
	}
	t2 := time.Now()
	dur := t2.Sub(t1)
	fmt.Printf("The %s took %v to run.  \n", cmd, dur)
}

func lang(dir string) {
	list, err := filePathWalkDir(dir)
	if err != nil {
		panic(err)
	}
	println("files:", len(list))
	eng := make([]string, 0, 0)
	rus := make([]string, 0, 0)
	options := whatlanggo.Options{
		Whitelist: map[whatlanggo.Lang]bool{
			whatlanggo.Eng: true,
			whatlanggo.Rus: true,
		},
	}
	_ = options
	for _, f := range list {
		_ = f
		//title, desc, _, _, text, err := info(f)
		title, desc, text, err := header(f)
		if err != nil || title == "" {
			println("skiped:", f, title, desc, err)
			continue
		}
		//println("file:", f, title, desc, err)

		info := whatlanggo.DetectLang(title + " " + desc + " " + text)
		lang := info.String()
		if lang == "Russian" {
			if strings.ContainsAny(title+desc, "ії") {
				lang = "ua"
				//println(title)
			}
		}
		switch lang {
		case "English":
			eng = append(eng, f)
		case "Russian":
			rus = append(rus, f)
		default:
			//println("not detected", lang)
		}
		//break
	}
	//eng: 36440 rus: 31886
	println("eng:", len(eng), "rus:", len(rus))
	err = pudge.DeleteFile("db/lang")
	if err != nil {
		println(err.Error())
	}
	err = pudge.Set("db/lang", "rus", rus)
	if err != nil {
		println(err.Error())
	}
	err = pudge.Set("db/lang", "eng", eng)
	if err != nil {
		println(err.Error())
	}
}

func news(dir string) {
	rus := make([]string, 0, 0)
	pudge.Get("db/lang", "rus", &rus)
	domains := make(map[string]int, 0)
	for _, f := range rus {
		title, desc, url, _, _, _ := info(f)
		d, _ := domain(url)
		domains[d] = domains[d] + 1
		if d == "btvnovinite.bg" {
			_ = title
			println(f, title, desc)
		}
	}
	for dom, cnt := range domains {
		println(dom, cnt)
		break
	}
	//news-r.ru 944
	//runews24.ru 572
	//dvnovosti.ru 200
	//riasar.ru
	//tengrinews.kz 421
}

func header(file string) (title, desc, text string, err error) {
	f, err := os.Open(file)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer f.Close()
	b := make([]byte, 4096)
	_, err = f.Read(b)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	doc, _ := goquery.NewDocumentFromReader(bytes.NewReader(b))
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
	text = doc.Text()
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

func test(dir string) {
	t := 199063389064
	fmt.Printf("The t3 took %v to run.  \n", time.Duration(t))
	title := ""
	desc := ""
	path := "data/DataClusteringSample0107/20191107/15/1061689992896686063.html"
	f, _ := os.Open(path)
	b1 := make([]byte, 120400)
	_, err := f.Read(b1)
	//println(string(b1), n1, err)
	tit := bytes.Index(b1, []byte("title\" content=\""))
	println(tit)
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(b1))
	if err != nil {
		println("err", err.Error())
	}
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
	println(title, desc)
}

func info(file string) (title, desc, url, name, text string, err error) {
	f, err := os.Open(file)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer f.Close()
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
		if op == "og:url" {
			url = con
		}
		if op == "og:site_name" {
			name = con
		}
	})
	space := regexp.MustCompile(`\s+`)
	text = space.ReplaceAllString(doc.Text(), " ")
	text = strings.Replace(text, ",", "", -1)
	text = strings.Replace(text, ".", "", -1)
	return
}

func domain(uri string) (dom string, err error) {
	uri = strings.ToLower(uri)
	u, err := url.Parse(uri)
	if err != nil {
		return
	}
	parts := strings.Split(u.Hostname(), ".")
	if len(parts) <= 2 {
		return u.Hostname(), err
	}
	return parts[len(parts)-2] + "." + parts[len(parts)-1], nil
}

func words(text string) {
	arr := strings.Fields(text)
	m := map[string]int{}
	for _, w := range arr {

		if len([]rune(w)) < 4 {
			continue
		}
		m[strings.ToLower(w)]++
	}
	n := map[int][]string{}
	var a []int
	for k, v := range m {
		n[v] = append(n[v], k)
	}
	for k := range n {
		a = append(a, k)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(a)))
	for _, k := range a {
		if k == 1 {
			break
		}
		for _, s := range n[k] {
			fmt.Printf("%s, %d\n", s, k)
		}
	}
	/*for _, r := range []rune(text) {
		if unicode.IsSpace(r) {
			println("space")
		}
	}*/
}

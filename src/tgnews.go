package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/abadojack/whatlanggo"
)

const (
	isDebug = false
)

type Tokenizer interface {
	Seg(text string) []string
	Free()
}

type EnTokenizer struct {
}

// TFIDF tfidf model
type TFIDF struct {
	docIndex  map[string]int         // train document index in TermFreqs
	termFreqs []map[string]int       // term frequency for each train document
	termDocs  map[string]int         // documents number for each term in train data
	n         int                    // number of documents in train data
	stopWords map[string]interface{} // words to be filtered
	tokenizer Tokenizer              // tokenizer, space is used as default
}
type ByLang struct {
	LangCode string   `json:"lang_code"`
	Articles []string `json:"articles"`
}
type ByNews struct {
	Articles []string `json:"articles"`
}

type ByCategory struct {
	Category string   `json:"category"`
	Articles []string `json:"articles"`
}

type ByThread struct {
	Title    string   `json:"title"`
	Articles []string `json:"articles"`
}

type ByTop struct {
	Category string     `json:"category"`
	Threads  []ByThread `json:"threads"`
}

type Article struct {
	File       string `json:"file"`
	Name       string `json:",string"`
	LangCode   string `json:"lang_code"`
	Title      string
	Desc       string
	Href       string
	Domain     string
	SName      string
	Text       string
	IsNews     bool
	About      string
	TFIDF      map[string]float64
	CategoryId int
	Words      string
}

type Category struct {
	ID       int
	Name     string
	LangCode string
	Words    []string
	Weights  map[string]float64
}

//category – "society", "economy", "technology", "sports", "entertainment", "science" или "other"
//Society (включает Politics, Elections, Legislation, Incidents, Crime)
//Economy (включает Markets, Finance, Business)
//Technology (включает Gadgets, Auto, Apps, Internet services)
//Sports (включает E-Sports)
//Entertainment (включает Movies, Music, Games, Books, Arts)
//Science (включает Health, Biology, Physics, Genetics)
//Other (новостные статьи, не попавшие в перечисленные выше категории)

//go run tgnews.go languages data/DataClusteringSample0107/20191101/00/
//take at look at https://github.com/timdrijvers/recommendation

// /Users/vadim/Downloads/Telegram\ Desktop/contest2225/20191122
func main() {
	//println()
	//println("-- tgnews --")
	runtime.GOMAXPROCS(runtime.NumCPU())
	args := os.Args
	cmd := "languages"
	dir := "data"
	dirtrain := "train"
	if len(args) >= 2 {
		cmd = args[1]
	}
	if len(args) >= 3 {
		dir = args[2]
	}
	if len(args) >= 4 {
		dirtrain = args[3]
	}
	//println("cmd:", cmd, "dir:", dir)
	t1 := time.Now()
	switch cmd {
	default:
		lang(dir)
	case "news":
		news(dir)
	case "categories":
		categories(dir, true)
	case "threads":
		threads(dir, true)
	case "top":
		toppairs(dir)
	case "train":
		train(dir, dirtrain)
	}
	t2 := time.Now()
	dur := t2.Sub(t1)
	_ = dur
	if isDebug {
		fmt.Printf("The %s took %v to run.  \n", cmd, dur)
	}
}

// AByInfo return parced articles
func AByInfo(in []Article, onlyNews bool) (out []Article) {
	var wg sync.WaitGroup
	parser := func(a Article) {
		defer wg.Done()
		isNews := false
		title, desc, url, sname, text, err := info(a.File)
		if err != nil {
			return
		}
		d, errDom := domain(url)
		if errDom == nil {
			a.Domain = d
		}
		if strings.Contains(url, "news/") {
			isNews = true
		}
		if strings.Contains(sname, "news") {
			isNews = true
		}

		if strings.Contains(d, "news") {
			isNews = true
		}

		if !isNews && onlyNews {
			return
		}
		a.Title = title
		a.Desc = desc
		a.Href = url
		a.SName = sname
		a.Text = text
		a.IsNews = isNews
		a.Words = strings.Join(bigwords(a.Title+" "+a.Desc+" "+a.Text+" "+a.SName), " ")
		out = append(out, a)
	}
	i := 0
	for _, a := range in {
		wg.Add(1)
		i++
		go parser(a)
		if i%500 == 0 {
			wg.Wait()
		}
	}
	wg.Wait()
	return out
}

/*
// AByLang isolate ru and en articles
func AByLang(dir string) []Article {
	list, err := filePathWalkDir(dir)
	if err != nil {
		panic(err)
	}
	//println("files:", len(list))
	articles := make([]Article, 0, 0)
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
			//println("skiped:", f, title, desc, err)
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
			articles = append(articles, Article{Name: filepath.Base(f), File: f, LangCode: "en"})
		case "Russian":
			articles = append(articles, Article{Name: filepath.Base(f), File: f, LangCode: "ru"})
		default:
			//println("not detected", lang)
		}
		//break
	}
	return articles
}*/

// AByLang isolate ru and en articles
func AByLang(dir string) []Article {
	list, err := filePathWalkDir(dir)
	if err != nil {
		panic(err)
	}
	//println("files:", len(list))
	articles := make([]Article, 0, 0)
	/*
		options := whatlanggo.Options{
			Whitelist: map[whatlanggo.Lang]bool{
				whatlanggo.Eng: true,
				whatlanggo.Rus: true,
			},
		}
	*/
	//articles = append(articles, Article{Name: filepath.Base(f), File: f, LangCode: "en"})
	//out := make(chan Article, 10)
	var wg sync.WaitGroup
	detector := func(f string) {
		defer wg.Done()
		title, desc, text, err := header(f)
		if err != nil || title == "" {
			return
		}

		info := whatlanggo.DetectLang(title + " " + desc + " " + text)
		lang := info.String()
		if lang == "Russian" {
			if strings.ContainsAny(title+desc, "ії") {
				lang = "ua"
			}
		}
		switch lang {
		case "English":
			articles = append(articles, Article{Name: filepath.Base(f), File: f, LangCode: "en"})
			return
		case "Russian":
			articles = append(articles, Article{Name: filepath.Base(f), File: f, LangCode: "ru"})
			return
		default:

		}
		return
	}
	cnt := 0
	for _, f := range list {
		cnt++
		wg.Add(1)
		go detector(f)
		if cnt%500 == 0 {
			wg.Wait()
		}
	}
	wg.Wait()
	//println(len(articles))

	return articles
}

func lang(dir string) {
	articles := AByLang(dir)
	//eng: 36440 rus: 31886
	//println("eng:", len(eng), "rus:", len(rus))

	langs := make([]ByLang, 2)
	langs[0].LangCode = "en"
	eng := make([]string, 0)
	rus := make([]string, 0)
	for _, a := range articles {
		if a.LangCode == "en" {
			eng = append(eng, a.Name)
		}
		if a.LangCode == "ru" {
			rus = append(rus, a.Name)
		}
	}
	langs[0].Articles = eng

	langs[1].LangCode = "ru"
	langs[1].Articles = rus
	json, err := json.MarshalIndent(langs, "", "  ")
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println(string(json))
	//bylang[0].LangCode = "en"

	//pudge.DeleteFile("db/bylang")

	//err = pudge.Set("db/bylang", dir, articles)
	//if err != nil {
	//	println(err.Error())
	//}
}

//APrint print articles
func APrint(articles []Article) {

	b, err := json.MarshalIndent(articles, "", "  ")

	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println(string(b))
	println(len(articles))
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func copy(src string, dst string) {
	// Read all content of src to data
	data, err := ioutil.ReadFile(src)
	checkErr(err)
	// Write data to dst
	err = ioutil.WriteFile(dst, data, 0644)
	checkErr(err)
}

//ARu filter by ru
func ARu(in []Article) (out []Article) {
	dom := make(map[string]byte)
	for _, a := range in {
		if a.LangCode == "ru" { //&& a.Domain == "car.ru" {
			dom[a.Domain] = 1
			//copy(a.File, "clusters_ru/technology/"+a.Name)
			out = append(out, a)
		}
	}
	//technology computerworld.ru  car.ru appstudio.org
	//sports vringe.com
	//klerk.ru
	for d, _ := range dom {
		_ = d
		//println(d)
	}
	return
}

func news(dir string) {
	articles := make([]Article, 0, 0)
	articles = categories(dir, false)

	byNews := &ByNews{}
	for _, a := range articles {
		if a.CategoryId == -1 {
			continue
		}
		name := a.Name
		if isDebug {
			name = a.Title
		}
		byNews.Articles = append(byNews.Articles, name)
	}
	b, err := json.MarshalIndent(byNews, "", "  ")
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println(string(b))

}

func traintf(in []Article) []Article {
	tf := NewTFIDF()
	for _, a := range in {
		//words := strings.Join(bigwords(a.Title+" "+a.Desc+" "+a.Text+" "+a.SName), " ")
		tf.AddDocs(a.Words)
	}
	for i, a := range in {
		//words := strings.Join(bigwords(a.Title+" "+a.Desc+" "+a.Text+" "+a.SName), " ")
		w := tf.Cal(a.Words)
		//	println(a.Title)
		in[i].About = top(w, 100)
		in[i].TFIDF = w
		//println(top(w, 30))
		//println()
	}

	return in
}

func printmap(m map[string]float64) {
	n := map[float64][]string{}
	var a []float64
	for k, v := range m {
		n[v] = append(n[v], k)
	}
	for k := range n {
		a = append(a, k)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(a)))
	j := 0
	for _, k := range a {
		if k == 1 {
			break
		}
		for _, s := range n[k] {
			j++
			fmt.Printf("%d %s, %.20f\n", j, s, k)
			if j > 70 {
				break
			}
		}
		if j > 70 {
			break
		}
	}
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
		if err != nil {
			fmt.Println(err.Error())
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
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
			name = strings.ToLower(con)
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

//tf Если документ содержит 100 слов, и слово[3] «заяц» встречается в нём 3 раза, то частота слова (TF) для слова «заяц» в документе будет 0,03 (3/100).
//idf Таким образом, если «заяц» содержится в 1000 документах из 10 000 000 документов, то IDF будет равной: log(10 000 000/1000) = 4.
//TF-IDF вес для слова «заяц» в выбранном документе будет равен: 0,03 × 4 = 0,12

func bigwords(text string) (res []string) {
	arr := strings.Fields(text)
	for _, w := range arr {
		if len([]rune(w)) < 4 {
			continue
		}
		if len([]rune(w)) > 8 {
			w = string([]rune(w)[:8])
		} else {
			if len([]rune(w)) > 6 {
				w = string([]rune(w)[:6])
			}
		}
		if strings.HasPrefix(w, "<") {
			continue
		}
		if strings.ContainsAny(w, ",«»():") {
			w = strings.ReplaceAll(w, ":", "")
			w = strings.ReplaceAll(w, "(", "")
			w = strings.ReplaceAll(w, ")", "")
			w = strings.ReplaceAll(w, "«", "")
			w = strings.ReplaceAll(w, "»", "")
			w = strings.ReplaceAll(w, ",", "")
		}
		res = append(res, strings.ToLower(w))
	}
	return
}

func top(m map[string]float64, limit int) string {
	res := make([]string, 0)
	n := map[float64][]string{}
	var a []float64
	for k, v := range m {
		n[v] = append(n[v], k)
	}
	for k := range n {
		a = append(a, k)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(a)))
	j := 0
	for _, k := range a {
		if k == 1 {
			break
		}
		for _, s := range n[k] {
			j++
			res = append(res, s)
			//fmt.Printf("%d %s, %.20f\n", j, s, k)
			if j > limit {
				break
			}
		}
		if j > limit {
			break
		}
	}
	return strings.Join(res, " ")
}

func makeCorpus(a []string) map[string]int {
	retVal := make(map[string]int)
	var id int
	for _, s := range a {
		for _, f := range strings.Fields(s) {
			if _, ok := retVal[f]; !ok {
				retVal[f] = id
				id++
			}
		}
	}
	return retVal
}

func categWords(dir string) []string {
	articles := make([]Article, 0, 0)
	articles = AByLang(dir)
	articles = AByInfo(articles, false)
	//APrint(ARu(articles))
	res := make([]string, 0)
	for _, a := range articles {
		res = append(res, bigwords(a.Title+" "+a.Desc+" "+a.Text+" "+a.SName)...)
	}
	return res
}

func train(dir, dirtrain string) {
	articles := AByLang(dir)

	articles = AByInfo(articles, false)
	//t2 := time.Now()

	//cosine
	tf := NewTFIDF()
	for _, a := range articles {
		//words := strings.Join(bigwords(a.Title+" "+a.Desc+" "+a.Text+" "+a.SName), " ")
		tf.AddDocs(a.Words)
	}
	categs := initCategs(tf)
	//cosine
	var input string
	all := len(articles)
	println(all)
	cnt := 0
	for i, a := range articles {
		_ = i
		txt := a.Text
		if len(txt) > 500 {
			txt = txt[:500]
		}
		v := &Article{File: a.File, Title: a.Title, LangCode: a.LangCode, Desc: a.Desc, Domain: a.Domain, Text: txt, Href: a.Href}
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			fmt.Println(err.Error())
		}
		_ = b
		println(string(b))
		println()

		//words := strings.Join(bigwords(a.Title+" "+a.Desc+" "+a.Text+" "+a.SName), " ")
		w := tf.Cal(a.Words)
		maxsim := float64(0)
		maxj := -1

		for j := range categs {
			sim := Cosine(w, categs[j].Weights)
			if sim > float64(0.555) && sim > maxsim {
				maxsim = sim
				maxj = j
			}
		}
		if maxj >= 0 {
			//println("Current:", categs[maxj].ID, categs[maxj].LangCode)
			cnt++
			continue
		}

		fmt.Print(i, all, ": Enter text: \n")
		println(`
			// 0. Stop
			// 1. Society (включает Politics, Elections, Legislation, Incidents, Crime)
			// 2. Economy (включает Markets, Finance, Business)
			// 3. Technology (включает Gadgets, Auto, Apps, Internet services)
			// 4. Sports (включает E-Sports)
			// 5. Entertainment (включает Movies, Music, Games, Books, Arts)
			// 6. Science (включает Health, Biology, Physics, Genetics)
			// 7. Other (новостные статьи, не попавшие в перечисленные выше категории)
			// 8. Not news
			// 9. Skip
			`)
		fmt.Scanln(&input)
		fmt.Print(input)
		if input == "0" {
			break
		}
		if input == "9" {
			continue
		}
		if input == "" {
			continue
		}
		s := fmt.Sprintf("train/%s/%s/%s", a.LangCode, input, a.Name)
		copy(a.File, s)
	}
	println(cnt)
}

func initCategs(tf *TFIDF) (categs []Category) {

	langs := []string{"en", "ru"}
	for _, l := range langs {
		for i := 1; i < 8; i++ {
			files := fmt.Sprintf("train/%s/%d", l, i)
			categ := Category{ID: i, LangCode: l}
			categ.LangCode = l

			categ.Words = categWords(files)
			//println(i, len(categ.Words))
			categs = append(categs, categ)
			tf.AddDocs(strings.Join(categ.Words, " "))
		}
	}
	for i := range categs {
		categs[i].Weights = tf.Cal(strings.Join(categs[i].Words, " "))
	}
	return
}

func categories(dir string, print bool) []Article {
	//articles := make([]Article, 0, 0)
	//t1 := time.Now()
	//err := pudge.Get("db/bylang", dir, &articles)
	//if err != nil || len(articles) == 0 {

	articles := AByLang(dir)
	//println(len(articles))
	//}

	articles = AByInfo(articles, false)
	//t2 := time.Now()

	//cosine
	tf := NewTFIDF()
	for _, a := range articles {
		//words := strings.Join(bigwords(a.Title+" "+a.Desc+" "+a.Text+" "+a.SName), " ")
		tf.AddDocs(a.Words)
	}
	categs := initCategs(tf)
	//cosine

	cnt := 0
	for i, a := range articles {
		//words := strings.Join(bigwords(a.Title+" "+a.Desc+" "+a.Text+" "+a.SName), " ")
		w := tf.Cal(a.Words)
		_ = w
		maxsim := float64(0)
		maxj := -1
		articles[i].CategoryId = -1

		N := len(categs)
		sem := make(chan bool, N)
		res := make(map[int]float64)
		var mu sync.Mutex
		for j, _ := range categs {
			if categs[j].LangCode != a.LangCode {
				sem <- true
				continue
			}
			go func(a, b map[string]float64, c int) {
				sim := Cosine(a, b)
				mu.Lock()
				res[c] = sim
				mu.Unlock()
				sem <- true
			}(w, categs[j].Weights, j)
			/*sim := Cosine(w, categs[j].Weights)
			if sim > float64(0.555) && sim > maxsim {
				maxsim = sim
				maxj = j
			}*/
		}
		for i := 0; i < N; i++ {
			<-sem
		}
		for i := 0; i < N; i++ {
			if res[i] > float64(0.555) && res[i] > maxsim {
				maxsim = res[i]
				maxj = i
			}
		}
		if maxj >= 0 {
			if maxj > 6 {
				articles[i].CategoryId = maxj - 7
			} else {
				articles[i].CategoryId = maxj
			}

			//println("Current:", categs[maxj].ID, categs[maxj].LangCode)
			cnt++
			continue
		}
	}
	//t3 := time.Now()
	if isDebug {
		//fmt.Printf("The %v took %v to run.  \n", t2.Sub(t1), t3.Sub(t2))
	}

	sort.Slice(articles, func(i, j int) bool {
		return articles[i].CategoryId < articles[j].CategoryId
	})
	if print {
		//err = pudge.Set("db/bycateg", dir, articles)
		//if err != nil {
		//	fmt.Println(err.Error())
		//}

		byCategs := make([]ByCategory, 7)
		for i := 0; i < 7; i++ {
			byCateg := ByCategory{}
			switch i {
			case 0:
				byCateg.Category = "society"
			case 1:
				byCateg.Category = "economy"
			case 2:
				byCateg.Category = "technology"
			case 3:
				byCateg.Category = "sports"
			case 4:
				byCateg.Category = "entertainment"
			case 5:
				byCateg.Category = "science"
			case 6:
				byCateg.Category = "other"
			}
			byCateg.Articles = make([]string, 0)
			byCategs[i] = byCateg
		}
		for _, a := range articles {
			if a.CategoryId == -1 {
				continue
			}
			name := a.Name
			if isDebug {
				name = a.Title
			}
			byCategs[a.CategoryId].Articles = append(byCategs[a.CategoryId].Articles, name)
		}
		b, err := json.MarshalIndent(byCategs, "", "  ")
		if err != nil {
			fmt.Println(err.Error())
		}

		fmt.Println(string(b))
	}
	return articles
}

//Cosine return cosine similarity
func Cosine(a, b map[string]float64) (sim float64) {
	vec1, vec2 := vector(a, b)

	var product, squareSumA, squareSumB float64
	/*
		N := len(vec1)
		sem := make(chan bool, N)
		for i, v := range vec1 {
			go func(a, b float64) {
				product += a * b
				squareSumA += a * a
				squareSumB += b * b
				sem <- true
			}(v, vec2[i])
		}
		for i := 0; i < N; i++ {
			<-sem
		}*/

	for i, v := range vec1 {

		product += v * vec2[i]
		squareSumA += v * v
		squareSumB += vec2[i] * vec2[i]
	}

	if squareSumA == 0 || squareSumB == 0 {
		return 0
	}

	return normalize(product / (math.Sqrt(squareSumA) * math.Sqrt(squareSumB)))
}

func normalize(cos float64) float64 {
	return 0.5 + 0.5*cos
}

func vector(a, b map[string]float64) ([]float64, []float64) {
	terms := make(map[string]interface{})
	for term := range a {
		terms[term] = nil
	}
	for term := range b {
		terms[term] = nil
	}
	lenterm := len(terms)

	vec1 := make([]float64, 0, lenterm)
	vec2 := make([]float64, 0, lenterm)
	for term := range terms {
		vec1 = append(vec1, a[term])
		vec2 = append(vec2, b[term])
	}

	return vec1, vec2
}

/*
func vector(a, b map[string]float64) ([]float64, []float64) {
	terms := make(map[string]interface{})
	for term := range a {
		terms[term] = nil
	}
	for term := range b {
		terms[term] = nil
	}
	lenterm := len(terms)

	vec1 := make([]float64, lenterm)
	vec2 := make([]float64, lenterm)
	i := 0
	for term := range terms {
		vec1[i] = a[term]
		vec2[i] = a[term]
		//vec2 = append(vec2, b[term])
		i++
	}

	return vec1,vec2
}
*/
func threads(dir string, print bool) [][]Article {
	articles := make([]Article, 0, 0)

	//err := pudge.Get("db/bycateg", dir, &articles)
	//if err != nil || len(articles) == 0 {
	articles = categories(dir, false)
	//}

	chunks := make(map[string][]Article)
	for _, a := range articles {
		if a.CategoryId == -1 {
			continue
		}
		key := fmt.Sprintf("%s%d", a.LangCode, a.CategoryId)
		if _, ok := chunks[key]; !ok {
			chunks[key] = make([]Article, 0)
		}
		chunks[key] = append(chunks[key], a)
	}

	allpairs := make([][]Article, 0)
	for _, val := range chunks {
		arr := pairs(val)
		for _, p := range arr {
			allpairs = append(allpairs, p)
		}
	}
	if print {

		byThreads := make([]ByThread, 0)
		for _, p := range allpairs {
			byThread := ByThread{}
			byThread.Articles = make([]string, 0)
			if len(p) > 0 {
				byThread.Title = p[0].Title
			}
			for _, it := range p {
				name := it.Name
				if isDebug {
					name = it.Title
				}
				byThread.Articles = append(byThread.Articles, name)
			}
			byThreads = append(byThreads, byThread)
		}
		b, err := json.MarshalIndent(byThreads, "", "  ")
		if err != nil {
			fmt.Println(err.Error())
		}

		fmt.Println(string(b))
	}
	return allpairs
	//fmt.Printf("pairs:%+v\n", allpairs)
	//APrint(tra)
}

func pairs(in []Article) (sortedpairs [][]Article) {

	trained := traintf(in)
	tres := float64(0.777)
	var cur Article
	skiplist := make(map[int]bool)
	allpairs := make([][]Article, 0)
	for i := 0; i < len(trained); i++ {
		cur = trained[i]
		if _, ok := skiplist[i]; ok {
			continue
		}
		pairs := make([]Article, 0)
		for j := 0; j < len(trained); j++ {
			if i == j {
				continue
			}
			if _, ok := skiplist[j]; ok {
				continue
			}
			sim := Cosine(cur.TFIDF, trained[j].TFIDF)
			if sim > tres {
				if len(pairs) == 0 {
					pairs = append(pairs, cur)
					skiplist[i] = true
				}
				pairs = append(pairs, trained[j])
				skiplist[j] = true
			}
		}
		if len(pairs) > 0 {
			allpairs = append(allpairs, pairs)
		}
	}
	type forsort struct {
		Article Article
		Sim     float64
	}
	for _, pair := range allpairs {
		tf := NewTFIDF()
		allwords := make([]string, 0)
		for _, a := range pair {
			words := bigwords(a.Title + " " + a.Desc + " " + a.Text + " " + a.SName)
			tf.AddDocs(strings.Join(words, " "))
			allwords = append(allwords, words...)
		}
		tf.AddDocs(strings.Join(allwords, " "))
		allw := tf.Cal(strings.Join(allwords, " "))
		forsorts := make([]forsort, 0)
		for _, a := range pair {

			//words := bigwords(a.Title + " " + a.Desc + " " + a.Text + " " + a.SName)
			curw := tf.Cal(a.Words) //strings.Join(words, " "))
			sim := Cosine(allw, curw)
			forsorts = append(forsorts, forsort{Article: a, Sim: sim})
			//fmt.Printf("%s %s %.5f\n\n", a.Title, a.File, sim)
		}
		sort.Slice(forsorts, func(i, j int) bool {
			return forsorts[i].Sim > forsorts[j].Sim
		})
		sortedpair := make([]Article, 0)
		for _, f := range forsorts {
			//fmt.Printf("%s  %.5f\n\n", f.Article.Title, f.Sim)
			sortedpair = append(sortedpair, f.Article)
		}
		sortedpairs = append(sortedpairs, sortedpair)
	}

	return
}

func toppairs(dir string) {
	allpairs := threads(dir, false)
	type topPair struct {
		Article Article
		Sim     float64
		Pair    []Article
		CategID int
	}
	tf := NewTFIDF()

	tops := make([]topPair, 0)
	for _, p := range allpairs {
		top := topPair{Pair: p}
		if len(p) > 0 {
			top.Article = p[0]
			top.CategID = p[0].CategoryId
		}
		//a := p[0]
		//words := strings.Join(bigwords(a.Title+" "+a.Desc+" "+a.Text+" "+a.SName), " ")
		//_ = words
		//tf.AddDocs(words)
		tops = append(tops, top)
	}
	categs := initCategs(tf)
	//fmt.Printf("%d\n", len(categs))
	for i, t := range tops {
		//println(t.Article.LangCode, t.CategID, t.Article.Title)
		idx := t.CategID
		if t.Article.LangCode == "ru" {
			idx += 7
		}
		a := t.Article
		//words := strings.Join(bigwords(a.Title+" "+a.Desc+" "+a.Text+" "+a.SName), " ")
		w := tf.Cal(a.Words)
		sim := Cosine(categs[idx].Weights, w)
		tops[i].Sim = sim
	}
	sort.Slice(tops, func(i, j int) bool {
		return tops[i].Sim > tops[j].Sim
	})
	bytops := make([]ByTop, 0)
	bytop := ByTop{}
	bytop.Category = "any"
	bytop.Threads = make([]ByThread, 0)
	for i, t := range tops {
		if i > 9 {
			break
		}
		byThread := ByThread{}
		for _, a := range t.Pair {
			byThread.Articles = append(byThread.Articles, a.Name)
		}

		byThread.Title = t.Article.Title
		bytop.Threads = append(bytop.Threads, byThread)
		//fmt.Printf("%s %0.5f\n\n", t.Article.Title, t.Sim)
	}
	bytops = append(bytops, bytop)

	sort.Slice(tops, func(i, j int) bool {
		return tops[i].CategID < tops[j].CategID
	})

	lastcateg := -1
	//var byTop ByTop
	for i, t := range tops {
		if t.CategID != lastcateg {
			if i != 0 {
				bytops = append(bytops, bytop)
			}
			lastcateg = t.CategID
			bytop = ByTop{}
			switch lastcateg {
			case 0:
				bytop.Category = "society"
			case 1:
				bytop.Category = "economy"
			case 2:
				bytop.Category = "technology"
			case 3:
				bytop.Category = "sports"
			case 4:
				bytop.Category = "entertainment"
			case 5:
				bytop.Category = "science"
			case 6:
				bytop.Category = "other"
			}
			bytop.Threads = make([]ByThread, 0)
		}
		byThread := ByThread{}
		for _, a := range t.Pair {
			byThread.Articles = append(byThread.Articles, a.Name)
		}

		byThread.Title = t.Article.Title
		bytop.Threads = append(bytop.Threads, byThread)
		//fmt.Printf("%s %0.5f\n\n", t.Article.Title, t.Sim)
	}
	bytops = append(bytops, bytop)

	b, err := json.MarshalIndent(bytops, "", "  ")
	if err != nil {
		fmt.Println(err.Error())
	}

	fmt.Println(string(b))
}

// New new model with default
func NewTFIDF() *TFIDF {
	return &TFIDF{
		docIndex:  make(map[string]int),
		termFreqs: make([]map[string]int, 0),
		termDocs:  make(map[string]int),
		n:         0,
		tokenizer: &EnTokenizer{},
	}
}

// NewTokenizer new with specified tokenizer
func NewTokenizer(tokenizer Tokenizer) *TFIDF {
	return &TFIDF{
		docIndex:  make(map[string]int),
		termFreqs: make([]map[string]int, 0),
		termDocs:  make(map[string]int),
		n:         0,
		tokenizer: tokenizer,
	}
}

// AddDocs add train documents
func (f *TFIDF) AddDocs(docs ...string) {
	for _, doc := range docs {
		h := hash(doc)
		if f.docHashPos(h) >= 0 {
			return
		}

		termFreq := f.termFreq(doc)
		if len(termFreq) == 0 {
			return
		}

		f.docIndex[h] = f.n
		f.n++

		f.termFreqs = append(f.termFreqs, termFreq)

		for term := range termFreq {
			f.termDocs[term]++
		}
	}
}

// Cal calculate tf-idf weight for specified document
func (f *TFIDF) Cal(doc string) (weight map[string]float64) {
	weight = make(map[string]float64)

	var termFreq map[string]int

	docPos := f.docPos(doc)
	if docPos < 0 {
		termFreq = f.termFreq(doc)
	} else {
		termFreq = f.termFreqs[docPos]
	}

	docTerms := 0
	for _, freq := range termFreq {
		docTerms += freq
	}
	for term, freq := range termFreq {
		weight[term] = tfidf(freq, docTerms, f.termDocs[term], f.n)
	}

	return weight
}

func (f *TFIDF) termFreq(doc string) (m map[string]int) {
	m = make(map[string]int)

	tokens := f.tokenizer.Seg(doc)
	if len(tokens) == 0 {
		return
	}

	for _, term := range tokens {
		//if _, ok := f.stopWords[term]; ok {
		//	continue
		//}

		m[term]++
	}

	return
}

func (f *TFIDF) docHashPos(hash string) int {
	if pos, ok := f.docIndex[hash]; ok {
		return pos
	}

	return -1
}

func (f *TFIDF) docPos(doc string) int {
	return f.docHashPos(hash(doc))
}

func hash(text string) string {
	h := md5.New()
	h.Write([]byte(text))
	return hex.EncodeToString(h.Sum(nil))
}

func tfidf(termFreq, docTerms, termDocs, N int) float64 {
	tf := float64(termFreq) / float64(docTerms)
	idf := math.Log(float64(1+N) / (1 + float64(termDocs)))
	return tf * idf
}

func (s *EnTokenizer) Seg(text string) []string {
	return strings.Fields(text)
}

func (s *EnTokenizer) Free() {

}

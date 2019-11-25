package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/abadojack/whatlanggo"
	. "github.com/go-nlp/dmmclust"
	"github.com/jbrukh/bayesian"
	"github.com/recoilme/pudge"
	"github.com/wilcosheh/tfidf"
	"github.com/wilcosheh/tfidf/similarity"
	"github.com/xtgo/set"
)

const (
	Good    bayesian.Class = "Good"
	Bad     bayesian.Class = "Bad"
	News    bayesian.Class = "News"
	NotNews bayesian.Class = "NotNews"
)

var data = []string{
	// coffee related tweet
	"A Java prefix that I don't hate .",
	"Colleagues must have thought I was crazy on Friday, but @MeccaCoffee ' s latest Xade Burqa is nothing short of orgasm inducing .",

	// JavaScript hate
	"Let me take this time while I wait for your JavaScript to download to tell you to stop using so much JavaScript on your web page .",

	// On Error Resume Next
	"all future programming languages I implement will have On Error Resume Next . Even if it's a functional , expressions-only language . Because I can. ",
	"On Error Resume Next",
	"When I was younger , I used VB. My crutch was On Error Resume Next . I find it weird reimplementing it for a probabilistic parser .",

	// Gophers/Golang
	"Questions for #gopher and #golang people out there : how do you debug a slow compile ? ",
	"In case you missed it , 10000 words on generics in #golang :",
	"Data Science in Go https://speakerdeck.com/chewxy/data-science-in-go … Slides by @chewxy #gopher #golang",
	"Big heap , many pointers . GC killing me . Help ? Tips? #golang . Most pointers unavoidable .",
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

type Article struct {
	File     string `json:"file"`
	Name     string `json:",string"`
	LangCode string `json:"lang_code"`
	Title    string
	Desc     string
	Href     string
	Domain   string
	SName    string
	Text     string
	IsNews   bool
	About    string
	TFIDF    map[string]float64
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
func main() {
	//println()
	//println("-- tgnews --")
	args := os.Args
	cmd := "languages"
	dir := "data"
	if len(args) >= 2 {
		cmd = args[1]
	}
	if len(args) >= 3 {
		dir = args[2]
	}
	//println("cmd:", cmd, "dir:", dir)
	t1 := time.Now()
	switch cmd {
	default:
		lang(dir)
	case "news":
		news(dir)
	case "test":
		test3(dir)
	}
	t2 := time.Now()
	dur := t2.Sub(t1)
	_ = dur
	//fmt.Printf("The %s took %v to run.  \n", cmd, dur)
}

func test3(d string) {
	rus := make([]string, 0, 0)
	pudge.Get("db/lang", "rus", &rus)

	_, _, _, _, text, _ := info(rus[0])
	println(text)
	f := tfidf.New()
	_, _, _, _, text2, _ := info(rus[1])
	f.AddDocs(text)
	f.AddDocs(text2)
	w1 := f.Cal(text)
	printmap(w1)
	fmt.Printf("weight of is %+v.\n", w1)
}

func test2(d string) {
	f := tfidf.New()
	f.AddDocs("how are you", "are you fine", "how old are you", "are you ok", "i am ok", "i am file")

	t1 := "how is so cool"
	w1 := f.Cal(t1)
	fmt.Printf("weight of %s is %+v.\n", t1, w1)

	t2 := "you are so beautiful"
	w2 := f.Cal(t2)
	fmt.Printf("weight of %s is %+v.\n", t2, w2)

	sim := similarity.Cosine(w1, w2)
	fmt.Printf("cosine between %s and %s is %f .\n", t1, t2, sim)
	classifier := bayesian.NewClassifier(Good, Bad)
	goodStuff := strings.Fields("во саду ли во огороде")
	badStuff := strings.Fields("во первых во вторых")
	classifier.Learn(goodStuff, Good)
	classifier.Learn(badStuff, Bad)
	//classifier.ConvertTermsFreqToTfIdf()
	//classifier.WordsByClass3(Good)
	res := classifier.WordsByClass(Good)
	for i, s := range res {
		fmt.Printf("%s %f\n", i, s)
		//break
	}
	scores, likely, st := classifier.LogScores(
		[]string{"ugly", "girl"})

	println(scores, likely, st)
}

func AByInfo(in []Article, onlyNews bool) (out []Article) {
	for _, a := range in {
		isNews := false
		title, desc, url, sname, text, err := info(a.File)
		if err != nil {
			//panic(err)
			continue
		}
		if strings.Contains(url, "news/") {
			isNews = true
		}
		if strings.Contains(sname, "news") {
			isNews = true
		}
		d, errDom := domain(url)
		if errDom == nil {
			a.Domain = d
		}
		if strings.Contains(d, "news") {
			isNews = true
		}
		if !isNews && onlyNews {
			continue
		}
		a.Title = title
		a.Desc = desc
		a.Href = url
		a.SName = sname
		a.Text = text
		a.IsNews = isNews
		out = append(out, a)
	}
	return out
}
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

	pudge.DeleteFile("db/bylang")

	err = pudge.Set("db/bylang", dir, articles)
	if err != nil {
		println(err.Error())
	}
}

func APrint(articles []Article) {

	b, err := json.MarshalIndent(articles, "", "  ")

	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println(string(b))
	println(len(articles))
}

func ARu(in []Article) (out []Article) {
	for _, a := range in {
		if a.LangCode == "ru" {
			out = append(out, a)
		}
	}
	return
}

func news(dir string) {
	articles := make([]Article, 0, 0)
	err := pudge.Get("db/bylang", dir, &articles)
	if err != nil || len(articles) == 0 {
		articles = AByLang(dir)
	}
	articles = AByInfo(articles, true)
	//APrint(ARu(articles))
	err = pudge.Set("db/bynews", dir, articles)
	if err != nil {
		println(err.Error())
	}
	byNews := &ByNews{}
	for _, a := range articles {
		byNews.Articles = append(byNews.Articles, a.Name)
	}
	b, err := json.MarshalIndent(byNews, "", "  ")
	if err != nil {
		fmt.Println(err.Error())
	}
	_ = b
	//fmt.Println(string(b))

	//clusters(ARu(articles))
	tr := traintf(ARu(articles))
	mapped := make(map[int]uint8)
	k := 0
	for i, a := range tr {
		if _, ok := mapped[i]; ok {
			continue
		}
		mapped[i] = 1
		max := float64(0)
		maxj := -1
		maxart := a
		for j, b := range tr {
			if _, ok := mapped[j]; ok {
				continue
			}
			sim := similarity.Cosine(a.TFIDF, b.TFIDF)
			if sim > max {
				max = sim
				maxart = b
				maxj = j
			}
		}
		if max > float64(0.55) {
			k++
			mapped[maxj] = 1
			println(k, a.Title, a.Domain)
			println(k, maxart.Title, maxart.Domain)
			//println("a.About")
			//println(a.About)
			//println("maxart.About")
			//println(maxart.About)
			fmt.Printf("similarity:%f\n", max)
			println()
		}
		if i > 50 {
			//break
		}
	}
	//clusters(tr)
}

func traintf(in []Article) []Article {
	tf := tfidf.New()
	for _, a := range in {
		words := strings.Join(bigwords(a.Title+" "+a.Desc+" "+a.Text+" "+a.SName), " ")
		tf.AddDocs(words)
	}
	for i, a := range in {
		words := strings.Join(bigwords(a.Title+" "+a.Desc+" "+a.Text+" "+a.SName), " ")
		w := tf.Cal(words)
		//	println(a.Title)
		in[i].About = top(w, 100)
		in[i].TFIDF = w
		//println(top(w, 30))
		//println()
	}

	return in
}

func clusters(in []Article) {
	data := make([]string, 0)
	for _, a := range in {
		data = append(data, a.About)
	}
	corp := makeCorpus(data)
	docs := makeDocuments(data, corp, true)
	r := rand.New(rand.NewSource(1337))
	conf := Config{
		K:          10,          // maximum 10 clusters expected
		Vocabulary: len(corp),   // simple example: the vocab is the same as the corpus size
		Iter:       1000,        // iterate 100 times
		Alpha:      0.0001,      // smaller probability of joining an empty group
		Beta:       0.1,         // higher probability of joining groups like me
		Score:      Algorithm4,  // use Algorithm3 to score
		Sampler:    NewGibbs(r), // use Gibbs to sample
	}
	var clustered []Cluster
	var err error
	if clustered, err = FindClusters(docs, conf); err != nil {
		fmt.Println(err)
	}
	fmt.Println("Clusters (Algorithm4):")
	for i, clust := range clustered {
		fmt.Printf("\t%d: %q\n", clust.ID(), data[i])
	}
}

func newsTmp(dir string) {
	// en 18123	rus 16248 "news"
	// en 12509 rus 13711 "news/"
	articles := make([]Article, 0, 0)
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
	pudge.Get("db/bylang", "bylang", &articles)
	domains := make(map[string]int, 0)
	news := make([]string, 0)
	cl := bayesian.NewClassifierTfIdf(News, NotNews)

	lh := tfidf.New()
	sv := tfidf.New()
	all := tfidf.New()

	baglh := make(map[string]float64)
	bagsv := make(map[string]float64)
	bagall := make(map[string]float64)
	t1, t2 := "", ""
	for _, f := range rus {
		title, _, url, _, text, _ := info(f)
		if strings.Contains(url, "news/") {
			news = append(news, url)
			//cl.Learn(bigwords(text), News)
		} else {
			//cl.Learn(bigwords(text), NotNews)
		}
		d, _ := domain(url)
		domains[d] = domains[d] + 1
		if d == "lifehacker.ru" {
			if t1 == "" {
				t1 = text
			}
			lh.AddDocs(text)
			all.AddDocs(text)
			for _, word := range bigwords(text) {
				baglh[word]++
				bagall[word]++
			}
			cl.Learn(bigwords(text), NotNews)
			_ = title
			//println(f, title, desc)
		}
		if d == "svpressa.ru" {
			if t2 == "" {
				t2 = text
			}
			for _, word := range bigwords(text) {
				bagsv[word]++
				bagall[word]++
			}
			sv.AddDocs(text)
			all.AddDocs(text)
			cl.Learn(bigwords(text), News)

		}
	}
	cl.ConvertTermsFreqToTfIdf()
	for dom, cnt := range domains {
		println(dom, cnt)
		break
	}
	for i, u := range news {
		println(i, u)
		break
	}
	tt := top(bagall, 500)
	lhonly := make(map[string]float64)
	for wor, val := range baglh {
		if _, ok := bagsv[wor]; !ok {
			lhonly[wor] = val
		}
	}
	toplhon := top(lhonly, 50)
	println("top:", toplhon)
	println(tt)
	tl := top(baglh, 50)
	println("lh:", tl)
	ts := top(bagsv, 50)
	println("sv:", ts)
	w1 := lh.Cal(tt)
	//printmap(baglh)
	println("lifehacker")
	printmap(w1)
	w2 := sv.Cal(tt)
	println("svpressa")
	//printmap(baglh)
	printmap(w2)
	//0.56560745836484893623 0.53073511349010293880
	//0.56505354053652201429 0.52736314767274861115

	//0.56469524156275596738 0.52792204154531274796
	//0.53984176594954236261 0.58212331460708222064
	wlh := lh.Cal(top(baglh, 10000))
	wsv := sv.Cal(top(bagsv, 10000))
	w3 := all.Cal(t1)
	s1 := similarity.Cosine(w3, wlh)
	s2 := similarity.Cosine(w3, wsv)
	fmt.Printf("%.20f %.20f\n", s1, s2)
	w4 := all.Cal(t2)
	s3 := similarity.Cosine(w4, wlh)
	s4 := similarity.Cosine(w4, wsv)
	fmt.Printf("%.20f %.20f\n", s3, s4)
	//res := cl.WordsByClass(News)
	//m := cl.WordsByClass(News)
	//printmap(m)
	//println("not news")
	//printmap(cl.WordsByClass(NotNews))
	//news-r.ru 944
	//runews24.ru 572
	//dvnovosti.ru 200
	//riasar.ru
	//tengrinews.kz 421
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

func makeDocuments(a []string, c map[string]int, allowRepeat bool) []Document {
	retVal := make([]Document, 0, len(a))
	for _, s := range a {
		var ts []int
		for _, f := range strings.Fields(s) {
			id := c[f]
			ts = append(ts, id)
		}
		if !allowRepeat {
			ts = set.Ints(ts) // this uniquifies the sentence
		}
		retVal = append(retVal, TokenSet(ts))
	}
	return retVal
}

func Example() {
	corp := makeCorpus(data)
	docs := makeDocuments(data, corp, false)
	r := rand.New(rand.NewSource(1337))
	conf := Config{
		K:          10,          // maximum 10 clusters expected
		Vocabulary: len(corp),   // simple example: the vocab is the same as the corpus size
		Iter:       1000,        // iterate 100 times
		Alpha:      0.0001,      // smaller probability of joining an empty group
		Beta:       0.1,         // higher probability of joining groups like me
		Score:      Algorithm3,  // use Algorithm3 to score
		Sampler:    NewGibbs(r), // use Gibbs to sample
	}
	var clustered []Cluster
	var err error
	if clustered, err = FindClusters(docs, conf); err != nil {
		fmt.Println(err)
	}
	fmt.Println("Clusters (Algorithm3):")
	for i, clust := range clustered {
		fmt.Printf("\t%d: %q\n", clust.ID(), data[i])
	}

	// Using Algorithm4, where repeat words are allowed
	docs = makeDocuments(data, corp, true)
	conf.Score = Algorithm4
	if clustered, err = FindClusters(docs, conf); err != nil {
		fmt.Println(err)
	}

	fmt.Println("\nClusters (Algorithm4):")
	for i, clust := range clustered {
		fmt.Printf("\t%d: %q\n", clust.ID(), data[i])
	}
}

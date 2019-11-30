package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/abadojack/whatlanggo"
	"github.com/recoilme/pudge"
	"github.com/wilcosheh/tfidf"
)

const (
	isDebug = true
)

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
	case "categories":
		categories(dir, true)
	case "threads":
		threads(dir, true)
	case "top":
		toppairs(dir)
	case "class":
		class(dir)
	}
	t2 := time.Now()
	dur := t2.Sub(t1)
	_ = dur
	if isDebug {
		fmt.Printf("The %s took %v to run.  \n", cmd, dur)
	}
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

/*
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
}*/

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
	/*
		err := pudge.Get("db/bylang", dir, &articles)
		if err != nil || len(articles) == 0 {
			articles = AByLang(dir)
		}
		articles = AByInfo(articles, true)
		//APrint(ARu(articles))
		err = pudge.Set("db/bynews", dir, articles)
		if err != nil {
			println(err.Error())
		}*/
	byNews := &ByNews{}
	for _, a := range articles {
		if a.CategoryId == -1 {
			continue
		}
		byNews.Articles = append(byNews.Articles, a.Name)
	}
	b, err := json.MarshalIndent(byNews, "", "  ")
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println(string(b))

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

/*
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
*/
/*
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
}*/

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

/*
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
}*/
/*
func categ(dir string) {
	tech := categWords("clusters_ru/technology")
	sports := categWords("clusters_ru/sports")
	_ = tech
	_ = sports
	articles := make([]Article, 0, 0)
	err := pudge.Get("db/bynews", dir, &articles)
	if err != nil || len(articles) == 0 {

		err := pudge.Get("db/bylang", dir, &articles)
		if err != nil || len(articles) == 0 {
			articles = AByLang(dir)
		}
		articles = AByInfo(articles, true)
		//articles = AByLang(dir)
	}
	//APrint(ARu(articles))
	articles = ARu(articles)
	tf := tfidf.New()
	tf.AddDocs(strings.Join(tech, " "))
	tf.AddDocs(strings.Join(sports, " "))
	for _, a := range articles {
		words := strings.Join(bigwords(a.Title+" "+a.Desc+" "+a.Text+" "+a.SName), " ")
		tf.AddDocs(words)
	}
	techw := tf.Cal(strings.Join(tech, " "))
	sportsw := tf.Cal(strings.Join(sports, " "))
	for i, a := range articles {
		words := strings.Join(bigwords(a.Title+" "+a.Desc+" "+a.Text+" "+a.SName), " ")
		w := tf.Cal(words)
		articles[i].About = top(w, 100)
		articles[i].TFIDF = w

		simtech := similarity.Cosine(w, techw)
		simsports := similarity.Cosine(w, sportsw)
		if simtech > simsports {
			//more tech
			if simtech > float64(0.555) {
				fmt.Printf("tech: %s %s %f\n", a.Domain, a.Title, simtech)
			}
		} else {
			//more sports
			if simsports > float64(0.555) {
				fmt.Printf("sport: %s %s %f\n", a.Domain, a.Title, simsports)
			}
		}
	}
	println(len(articles))
}*/

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

func class(dir string) {
	articles := make([]Article, 0, 0)
	err := pudge.Get("db/bynews", dir, &articles)
	if err != nil || len(articles) == 0 {

		err := pudge.Get("db/bylang", dir, &articles)
		if err != nil || len(articles) == 0 {
			articles = AByLang(dir)
		}
		articles = AByInfo(articles, false)
		//articles = AByLang(dir)
	}

	//cosine
	tf := tfidf.New()
	for _, a := range articles {
		words := strings.Join(bigwords(a.Title+" "+a.Desc+" "+a.Text+" "+a.SName), " ")
		tf.AddDocs(words)
	}
	categs := make([]Category, 0)
	langs := []string{"en", "ru"}
	for _, l := range langs {
		for i := 1; i < 8; i++ {
			files := fmt.Sprintf("train/%s/%d", l, i)
			categ := Category{ID: i, LangCode: l}
			categ.Words = categWords(files)
			categs = append(categs, categ)

			tf.AddDocs(strings.Join(categ.Words, " "))
		}
	}
	for i := range categs {
		categs[i].Weights = tf.Cal(strings.Join(categs[i].Words, " "))
	}
	//cosine
	var input string
	_ = input
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
		//println(string(b))
		//println()

		words := strings.Join(bigwords(a.Title+" "+a.Desc+" "+a.Text+" "+a.SName), " ")
		w := tf.Cal(words)
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
		/*
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
					s := fmt.Sprintf("train/%s/%s/%s", a.LangCode, input, a.Name)
					copy(a.File, s)*/
	}
	println(cnt)
}

func initCategs(tf *tfidf.TFIDF) (categs []Category) {

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
	articles := make([]Article, 0, 0)
	t1 := time.Now()
	err := pudge.Get("db/bylang", dir, &articles)
	if err != nil || len(articles) == 0 {
		articles = AByLang(dir)
	}

	articles = AByInfo(articles, false)
	t2 := time.Now()

	//cosine
	tf := tfidf.New()
	for _, a := range articles {
		words := strings.Join(bigwords(a.Title+" "+a.Desc+" "+a.Text+" "+a.SName), " ")
		tf.AddDocs(words)
	}
	categs := initCategs(tf)
	//cosine

	cnt := 0
	for i, a := range articles {
		words := strings.Join(bigwords(a.Title+" "+a.Desc+" "+a.Text+" "+a.SName), " ")
		w := tf.Cal(words)
		_ = w
		maxsim := float64(0)
		maxj := -1
		articles[i].CategoryId = -1

		N := len(categs)
		sem := make(chan bool, N)
		res := make(map[int]float64)
		var mu sync.Mutex
		for j, _ := range categs {
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
	t3 := time.Now()
	if isDebug {
		fmt.Printf("The %v took %v to run.  \n", t2.Sub(t1), t3.Sub(t2))
	}

	sort.Slice(articles, func(i, j int) bool {
		return articles[i].CategoryId < articles[j].CategoryId
	})
	if print {
		err = pudge.Set("db/bycateg", dir, articles)
		if err != nil {
			fmt.Println(err.Error())
		}

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
			byCategs[a.CategoryId].Articles = append(byCategs[a.CategoryId].Articles, a.Name)
		}
		b, err := json.MarshalIndent(byCategs, "", "  ")
		if err != nil {
			fmt.Println(err.Error())
		}

		fmt.Println(string(b))
	}
	return articles
}

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

func vector(a, b map[string]float64) (vec1, vec2 []float64) {
	terms := make(map[string]interface{})
	for term := range a {
		terms[term] = nil
	}
	for term := range b {
		terms[term] = nil
	}

	for term := range terms {
		vec1 = append(vec1, a[term])
		vec2 = append(vec2, b[term])
	}

	return
}

func threads(dir string, print bool) [][]Article {
	articles := make([]Article, 0, 0)

	err := pudge.Get("db/bycateg", dir, &articles)
	if err != nil || len(articles) == 0 {
		articles = categories(dir, false)
	}

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
				byThread.Articles = append(byThread.Articles, it.Name)
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
		tf := tfidf.New()
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

			words := bigwords(a.Title + " " + a.Desc + " " + a.Text + " " + a.SName)
			curw := tf.Cal(strings.Join(words, " "))
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
	tf := tfidf.New()

	tops := make([]topPair, 0)
	for _, p := range allpairs {
		top := topPair{Pair: p}
		if len(p) > 0 {
			top.Article = p[0]
			top.CategID = p[0].CategoryId
		}
		a := p[0]
		words := strings.Join(bigwords(a.Title+" "+a.Desc+" "+a.Text+" "+a.SName), " ")
		_ = words
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
		words := strings.Join(bigwords(a.Title+" "+a.Desc+" "+a.Text+" "+a.SName), " ")
		w := tf.Cal(words)
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

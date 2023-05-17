package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"regexp"
	"sync"
	"time"
)

var count = 0
var lastCount = 0
var mu sync.Mutex

func main() {
	log.Println("Preparing")

	// GET THE TOKEN

	resOfHtml, err := http.Get("https://fast.com/index.html")
	if err != nil {
		log.Fatal("Unable to access fast.com", err)
	}

	html, err := io.ReadAll(resOfHtml.Body)
	if err != nil {
		log.Fatal("Unable to read from fast.com", err)
	}
	
	rForScriptFileId := regexp.MustCompile(`src\=\"\/app-(.*?)\.js\"`)
	scriptFileIdMatch := rForScriptFileId.FindStringSubmatch(string(html))
	if (len(scriptFileIdMatch) == 0) {
		log.Fatal("Unable to find js file. May be renamed from app-*.js pattern.")
	}
	scriptFileId := scriptFileIdMatch[1]
	
	res, err := http.Get("https://fast.com/app-" + scriptFileId + ".js")
	if err != nil {
		log.Fatal("Unable to access fast.com", err)
	}

	h, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatal("Unable to read from fast.com", err)
	}

	r := regexp.MustCompile(`token\:\"(.*?)\"`)
	tokenMatch := r.FindStringSubmatch(string(h))
	if (len(tokenMatch) == 0) {
		log.Fatal("Unable to find token in js file. May be renamed.")
	}
	token := tokenMatch[1]

	// GET THE URLS

	res, err = http.Get("https://api.fast.com/netflix/speedtest?https=true&token=" + token)
	if err != nil {
		log.Fatal("Unable to access api.fast.com", err)
	}

	j, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatal("Unable to read from api.fast.com", err)
	}

	var urls []map[string]string
	json.Unmarshal(j, &urls)

	// MONITOR SPEED

	ticker := time.NewTicker(500 * time.Millisecond)
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				mu.Lock()
				diff := count - lastCount
				lastCount = count
				mu.Unlock()

				log.Println("Speed", prettyByteSize(diff*2*8)+"bps")
			}
		}
	}()

	// FETCH URLS PARALLELY

	start := time.Now()
	var wg sync.WaitGroup
	for _, v := range urls {
		wg.Add(1)

		url := v["url"]

		go func() {
			defer wg.Done()
			download(url)
		}()
	}
	wg.Wait()
	end := time.Since(start)

	// SUMMARY & CLEANUP

	log.Println("Downloaded", prettyByteSize(count)+"B")
	log.Println("Average speed", prettyByteSize(int(float64(count*8)/end.Seconds()))+"bps")
	ticker.Stop()
	done <- true
}

func download(url string) {
	log.Println("Downloading", url)

	res, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}

	buf := make([]byte, 1024*1024*10)
	for {
		n, err := res.Body.Read(buf)
		mu.Lock()
		count = count + n
		mu.Unlock()

		if err != nil {
			break
		}
	}
}

// ref: https://gist.github.com/anikitenko/b41206a49727b83a530142c76b1cb82d?permalink_comment_id=4467913#gistcomment-4467913
func prettyByteSize(b int) string {
	bf := float64(b)
	for _, unit := range []string{"", "K", "M", "G", "T", "P", "E", "Z"} {
		if math.Abs(bf) < 1024.0 {
			return fmt.Sprintf("%3.1f %s", bf, unit)
		}
		bf /= 1024.0
	}
	return fmt.Sprintf("%.1f Y", bf)
}

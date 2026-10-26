package main

import (
	"fmt"
	"context"
	"net/http"
	"net/http/httputil"
	"flag"
	"os"
	"time"
	"errors"
	"net"
)

type result struct {
	responce []byte
	err error
}
func main() {
	var timeout int
	flag.IntVar(&timeout, "t", 15, "set timeout")
	flag.IntVar(&timeout, "timeout", 15, "set timeout")

	var help bool
	flag.BoolVar(&help, "h", false, "showing help")
	flag.BoolVar(&help, "help", false, "showing help")


	flag.Parse()
	
	if timeout <= 0 {
		fmt.Fprintln(os.Stderr, "Error: timout should be >0")
		os.Exit(1)
	}

	if help == true {
		showHelp()
		os.Exit(0)
	}

	urls := flag.Args()
	if len(urls) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no url")
		os.Exit(1)
	}

	client := &http.Client {
		Timeout: time.Duration(timeout) * time.Second,
	}

	results := make(chan result, len(urls))
	
	contx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, u := range urls {
		go func(url string) {
			req, err := http.NewRequestWithContext(contx, http.MethodGet, url, nil)
			if err != nil {
				results <- result{err: err}
				return
			}
			
			resp, err := client.Do(req)
			if err != nil {
				results <- result{err: err}
				return
			}
			defer resp.Body.Close()

			dump, err := httputil.DumpResponse(resp, true)
			if err != nil {
				results <- result{err: err}
				return
			}
			results <- result{responce: dump}
		}(u)
	}


	timeoutOccured := false

	for i := 0; i < len(urls); i++ {
		res := <-results

		if res.err == nil {
			cancel()
			os.Stdout.Write(res.responce)
			os.Exit(0)
		}

		fmt.Fprintln(os.Stderr, res.err)

		var netErr net.Error
		if errors.As(res.err, &netErr) && netErr.Timeout() {
			timeoutOccured = true
		}
	}

	if timeoutOccured {
		fmt.Fprintln(os.Stderr, "all requests timeout")
		os.Exit(228)
	}
	fmt.Fprintln(os.Stderr, "all reqests failed")
	os.Exit(1)
}

func showHelp() {
	fmt.Println("CLI утилита многопоточного curl'a с хеджированием")
	fmt.Println("Usage: hedgedcurl [options] URL1 [URL2 ...]")
	fmt.Println("Flags:")
	fmt.Println("-h, --help    показать справку")
	fmt.Println("-t, --timeout   установить тайм-аут в секундах")
}


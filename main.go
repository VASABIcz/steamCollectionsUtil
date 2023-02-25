package main

import (
	"fmt"
	"github.com/gocolly/colly/v2"
	"github.com/urfave/cli/v2"
	"io"
	"log"
	"math/rand"
	"net/http"
	url2 "net/url"
	"os"
	path2 "path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var urlRegex, _ = regexp.Compile("http://workshop\\d+.abcvg.info/archive/\\d+.\\d+.zip")

type State struct {
	mode     string
	verbose  bool
	savePath string
}

func scrapeCollections(url string, verbose bool) []string {
	c := colly.NewCollector()
	data := make([]string, 0)

	c.OnHTML(".workshopItem a", func(e *colly.HTMLElement) {
		item := e.Attr("href")
		if verbose {
			println("found", item)
		}
		data = append(data, item)
	})

	if verbose {
		c.OnRequest(func(r *colly.Request) {
			fmt.Println("requesting ", r.URL)
		})
	}

	c.Visit(url)

	if verbose {
		println("fetched", len(data), "items")
	}

	return data
}

func saveResolved(f string, data []string) {
	create, err := os.Create(f)

	defer create.Close()

	if err != nil {
		println("failed to create file ", f, err)
		return
	}

	for _, item := range data {
		_, err = create.WriteString(item)
		_, err = create.WriteString("\n")
	}
	if err != nil {
		println("errors trying to write file", f, err)
		return
	}
}

func createDownloaderUrl(steamUrl string, appId int) (string, error) {
	url, err := url2.Parse(steamUrl)
	if err != nil {
		return "", err
	}
	id := url.Query().Get("id")
	if id == "" {
		return "", fmt.Errorf("failed to retrive from %s url", url)
	}
	// http://workshop9.abcvg.info/archive/636480/2721562982.zip

	res, err := http.PostForm("http://steamworkshop.download/online/steamonline.php", url2.Values{"item": {id}, "app": {strconv.Itoa(appId)}})
	if err != nil {
		return "", err
	}

	responseBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	downloadUrl := urlRegex.Find(responseBytes)
	if downloadUrl == nil {
		return "", fmt.Errorf("server didnt return download url %s", steamUrl)
	}

	return string(downloadUrl), nil
}

// https://golangcode.com/download-a-file-from-a-url/
func DownloadFile(filepath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func main() {
	app := &cli.App{
		Name:  "workshopUtil",
		Usage: "download/fetch steam workshop collections",
		Commands: []*cli.Command{
			{
				Name:    "download",
				Usage:   "download steam workshop items",
				Aliases: []string{"d"},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "path",
						Value: "",
						Usage: "output file do dump download urls",
					},
					&cli.BoolFlag{
						Name:    "verbose",
						Value:   false,
						Usage:   "show verbose information",
						Aliases: []string{"v"},
					},
					&cli.IntFlag{
						Name:  "appId",
						Value: -1,
						Usage: "steam id of requested game, reduce wastefully requests",
					},
					&cli.PathFlag{
						Name:  "download",
						Value: "",
						Usage: "directory to download content",
					},
				},
				Action: func(context *cli.Context) error {
					path := context.Path("path")
					verbose := context.Bool("verbose")
					appId := context.Int("appId")
					download := context.Path("download")
					total := 0

					if appId == -1 {
						return fmt.Errorf("program doesnt support dynamic fetching of app ids, provide appId manualy")
					}

					urls := make([]string, 0)

					for _, arg := range context.Args().Slice() {
						data, err := os.ReadFile(arg)
						if err == nil {
							str := string(data)
							splited := strings.Split(str, "\n")

							for _, url := range splited {
								total++
								u, err := createDownloaderUrl(url, appId)
								if err != nil {
									if verbose {
										println("error creating download url", arg, "app", appId, err.Error())
									}
									continue
								} else {
									if verbose {
										println("resolved", u)
									}
									urls = append(urls, u)
								}
							}
							continue
						}
						total++
						u, err := createDownloaderUrl(arg, appId)
						if err != nil {
							if verbose {
								println("error creating download url", arg, "app", appId, err.Error())
							}
							continue
						} else {
							if verbose {
								println("resolved", u)
							}
							urls = append(urls, u)
						}
					}

					if path == "" {
						for _, url := range urls {
							println(url)
						}
					} else {
						saveResolved(path, urls)
					}

					println("generated", len(urls), "errors", total-len(urls))

					if download != "" {
						p, err := filepath.Abs(download)
						if err != nil {
							return err
						}

						for _, url := range urls {
							path := path2.Join(p, strconv.Itoa(rand.Intn(999999))+".zip")
							println("downloading", url, path)
							DownloadFile(path, url)
						}
						println("finished downloading")
					}

					return nil
				},
			},
			{
				Name:    "fetch",
				Usage:   "fetch steam workshop items",
				Aliases: []string{"f"},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "path",
						Value: "",
						Usage: "output file to dump fetched urls",
					},
					&cli.BoolFlag{
						Name:    "verbose",
						Value:   false,
						Usage:   "show verbose information",
						Aliases: []string{"v"},
					},
				},
				Action: func(context *cli.Context) error {
					path := context.Path("path")
					verbose := context.Bool("verbose")
					url := context.Args().Get(0)
					if url == "" {
						return fmt.Errorf("unspecified url to steam workshop")
					}

					uri, err := url2.Parse(url)
					if err != nil {
						return err
					}

					data := scrapeCollections(uri.String(), verbose)

					if path == "" {
						for _, item := range data {
							println(item)
						}
						println("resolved", len(data), "items from", uri.String())
					} else {
						saveResolved(path, data)

						p, _ := filepath.Abs(path)

						println("saved", len(data), "items from", uri.String(), "into", p)
					}
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

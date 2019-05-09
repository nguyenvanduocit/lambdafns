package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/google/go-github/github"
	"github.com/gosimple/slug"
	"github.com/mmcdole/gofeed"
	"golang.org/x/oauth2"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

type Site struct {
	RssURL   string `json:"rss_url"`
	Language string `json:"language"`
}

var (
	siteFileUrl      = "https://raw.githubusercontent.com/12bitvn/news.12bit.vn/master/data/sites.json"
	markdownTemplate = `---
title: "{{.Title}}"
date: {{.PublishedParsed.Format "2006-01-02T15:04:05Z07:00"}}
link: {{.Link}}
site: {{.Custom.site}}{{if .Custom.language}}
language: {{.Custom.language}}{{end}}{{if .Categories}}
category:{{range $category := .Categories}}
  - {{$category}}{{end}}{{end}}
draft: false
---
`
	committer      = "crawler"
	committerEmail = "12bitsvn@gmail.com"
	reponame       = "news.12bit.vn"
	owner          = "12bitvn"
	branch         = "master"
	path           = "content/links"

	feedParser   = gofeed.NewParser()
	httpClient   = &http.Client{Timeout: 10 * time.Second}
	githubClient *github.Client
	err          error

	maxArticlePerSite = 2
)

func handler(request events.CloudWatchEvent) (events.APIGatewayProxyResponse, error) {
	accessToken := os.Getenv("GITHUB_ACCESS_TOKEN")
	if accessToken == "" {
		return events.APIGatewayProxyResponse{}, errors.New("GITHUB_ACCESS_TOKEN is required")
	}
	githubClient = newGithubClient(accessToken)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	sites, err := fetchSiteList(siteFileUrl)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	feedChan := make(chan *gofeed.Feed)

	for _, site := range sites {
		go parseFeed(site, feedChan)
	}
	for index := 0; index < len(sites); index++ {
		select {
		case feed := <-feedChan:
			if feed == nil {
				break
			}
			log.Printf("Site: %s\n", feed.Title)
			articlePerSite := maxArticlePerSite
			if maxArticlePerSite > len(feed.Items) {
				articlePerSite = len(feed.Items) - 1
			}
			for _, feedItem := range feed.Items[:articlePerSite] {
				feedLink, _ := url.Parse(feed.Link)
				feedItemLink, _ := url.Parse(feedItem.Link)
				feedItem.Link = addUTM(feedItemLink)
				feedItem.Title = strings.ReplaceAll(feedItem.Title, `"`, `'`)
				feedItem.Custom = map[string]string{
					"site":     fmt.Sprintf("%s%s", feedLink.Host, strings.Trim(feedLink.Path, "/")),
					"language": feed.Language,
				}
				log.Printf("Link: %s\n", commit(feedItem, githubClient))
			}
		}
	}
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       "success",
	}, nil
}

func addUTM(link *url.URL) string {
	query := link.Query()
	query.Del("utm_source")
	query.Del("utm_medium")
	query.Del("utm_campaign")
	query.Add("utm_source", "news.12bit.vn")
	query.Add("utm_medium", "RSS")
	link.RawQuery = query.Encode()
	return link.String()
}

func parseFeed(site Site, resultChan chan *gofeed.Feed) (result *gofeed.Feed) {
	defer func() {
		resultChan <- result
	}()
	feed, err := feedParser.ParseURL(site.RssURL)
	if err != nil {
		log.Println(err)
		return nil
	}
	feed.Language = site.Language
	return feed
}

func fetchSiteList(fileUrl string) (map[string]Site, error) {
	var sites map[string]Site
	r, err := httpClient.Get(fileUrl)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&sites); err != nil {
		return nil, err
	}
	return sites, nil
}

func newGithubClient(accessToken string) *github.Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}

func commit(feedItem *gofeed.Item, client *github.Client) (result string) {
	defer func() {
		result = fmt.Sprintf("%s: %s", feedItem.Title, result)
	}()

	content, err := render(feedItem, markdownTemplate)
	if err != nil {
		return err.Error()
	}
	ctx := context.Background()
	opts := &github.RepositoryContentFileOptions{
		Message:   github.String(fmt.Sprintf("crawler: %s", feedItem.Title)),
		Content:   content.Bytes(),
		Branch:    github.String(branch),
		Committer: &github.CommitAuthor{Name: github.String(committer), Email: github.String(committerEmail)},
	}
	if !isFileExist(getFileName(feedItem)) {
		if _, _, err := client.Repositories.CreateFile(ctx, owner, reponame, filepath.Join(path, getFileName(feedItem)), opts); err != nil {
			return err.Error()
		}
	}
	return "success"
}

func getFileName(feedItem *gofeed.Item) string {
	return fmt.Sprintf("%s-%s.md", slug.Make(feedItem.Title), feedItem.PublishedParsed.Format(time.RFC3339))
}

func render(feedItem *gofeed.Item, templateString string) (*bytes.Buffer, error) {
	tmpl, err := template.New("feedItem-template").Parse(templateString)
	if err != nil {
		return nil, err
	}
	var tpl bytes.Buffer
	if err := tmpl.Execute(&tpl, *feedItem); err != nil {
		return nil, err
	}
	return &tpl, nil
}

func isFileExist(filename string) bool {
	fileUrl := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/master/%s", owner, reponame, filepath.Join(path, filename))
	response, err := http.Head(fileUrl)
	if err != nil {
		return false
	}
	return response.StatusCode == http.StatusOK
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	lambda.Start(handler)
}

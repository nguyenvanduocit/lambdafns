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
	"net/http"
	"os"
	"path/filepath"
	"text/template"
	"time"
)

type Site struct {
	RssURL string `json:"rss_url"`
}

var (
	siteFileUrl      = "https://raw.githubusercontent.com/12bitvn/news.12bit.vn/master/data/links.json"
	markdownTemplate = `---
title: "{{ .Title }}"
date: {{ .PublishedParsed.Format "2006-01-02T15:04:05Z07:00" }}
feedItem: "{{ .feedItem }}"
site: {{.Custom.site}}
draft: false
---
`
	committer      = "crawler"
	committerEmail = "12bitsvn@gmail.com"
	reponame       = "news.12bit.vn"
	owner          = "12bitvn"
	branch         = "master"
	path           = "content/feedItems"

	feedParser   = gofeed.NewParser()
	httpClient   = &http.Client{Timeout: 10 * time.Second}
	githubClient *github.Client
	err          error
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
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

	for _, site := range sites {
		feed, err := feedParser.ParseURL(site.RssURL)
		if err != nil {
			return events.APIGatewayProxyResponse{}, err
		}
		for _, feedItem := range feed.Items[:5] {
			feedItem.Custom["site"] = feed.Link
			if isFileExist(getFileName(feedItem)) {
				continue
			}
			if err := Commit(feedItem, githubClient); err != nil {
				return events.APIGatewayProxyResponse{}, err
			}
		}
	}
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       "success",
	}, nil
}

func fetchSiteList(fileUrl string) ([]Site, error) {
	var sites []Site
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

func Commit(feedItem *gofeed.Item, client *github.Client) error {
	content, err := Render(feedItem)
	if err != nil {
		return err
	}
	ctx := context.Background()
	opts := &github.RepositoryContentFileOptions{
		Message:   github.String(fmt.Sprintf("add new feedItem %s", getFileName(feedItem))),
		Content:   content,
		Branch:    github.String(branch),
		Committer: &github.CommitAuthor{Name: github.String(committer), Email: github.String(committerEmail)},
	}

	if _, _, err := client.Repositories.CreateFile(ctx, owner, reponame, filepath.Join(path, getFileName(feedItem)), opts); err != nil {
		return err
	}
	return nil
}

func getFileName(feedItem *gofeed.Item) string {
	return fmt.Sprintf("%s-%s.md", slug.Make(feedItem.Title), feedItem.PublishedParsed.Format(time.RFC3339))
}

func Render(feedItem *gofeed.Item) ([]byte, error) {
	tmpl, err := template.New("feedItem-template").Parse(markdownTemplate)
	if err != nil {
		return nil, err
	}
	var tpl bytes.Buffer
	if err := tmpl.Execute(&tpl, feedItem); err != nil {
		return nil, err
	}
	return tpl.Bytes(), nil
}

func isFileExist(filename string) bool {
	req, _ := http.NewRequest("GET", fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/master/%s", owner, reponame, filepath.Join(path, filename)), nil)
	req.Header.Add("Range", "bytes=0-1023")
	resp, _ := httpClient.Do(req)
	return resp.StatusCode == http.StatusOK
}

func main() {
	lambda.Start(handler)
}

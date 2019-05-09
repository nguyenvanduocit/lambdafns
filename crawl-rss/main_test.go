package main

import (
	"reflect"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
)

func Test_render(t *testing.T) {
	type args struct {
		feedItem       *gofeed.Item
		templateString string
	}
	now, _ := time.Parse(time.RFC3339, "2019-04-23T12:16:15+07:00")
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "test template",
			wantErr: false,
			want: `---
title: "title"
date: 2019-04-23T12:16:15+07:00
link: http://examp.com/article
site: http://muatocroi.com
language: vietnamese
category:
	- category 1
	- category 2
draft: false
---`,
			args: args{
				feedItem: &gofeed.Item{
					Title:           "title",
					PublishedParsed: &now,
					Link:            "http://examp.com/article",
					Custom: map[string]string{
						"site": "http://muatocroi.com",
					},
					Categories: []string{"category 1", "category 2"},
				},
				templateString: markdownTemplate,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := render(tt.args.feedItem, tt.args.templateString)
			if (err != nil) != tt.wantErr {
				t.Errorf("render() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.String(), tt.want) {
				t.Errorf("render() = %v, want %v", got, tt.want)
			}
		})
	}
}

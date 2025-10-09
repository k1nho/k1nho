package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"dagger/k-1-nho/internal/dagger"
)

type RSSFeed struct {
	Channel struct {
		Title         string    `xml:"title"`
		Link          string    `xml:"link"`
		Description   string    `xml:"description"`
		Language      string    `xml:"language"`
		Copyright     string    `xml:"copyright"`
		LastBuildDate string    `xml:"lastBuildDate"`
		Item          []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Guid        string `xml:"guid"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Category    string `xml:"category"`
}

func formatBlogList(items []RSSItem, max int) string {
	var sb strings.Builder
	count := 0
	for _, item := range items {
		if count >= max {
			break
		}
		sb.WriteString(fmt.Sprintf("- [%s](%s)\n", item.Title, item.Link))
		count++
	}
	return sb.String()
}

type K1nho struct{}

func fetchFeed(url string) (RSSFeed, error) {
	httpclient := http.Client{Timeout: 10 * time.Second}

	rssFeed := RSSFeed{}
	res, err := httpclient.Get(url)
	if err != nil {
		return rssFeed, err
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return rssFeed, err
	}

	err = xml.Unmarshal(data, &rssFeed)
	if err != nil {
		return rssFeed, err
	}

	return rssFeed, err
}

// Updates README with any new articles from my rss feed from by blog
func (m *K1nho) UpdateReadme(
	// +defaultPath="."
	source *dagger.Directory,
	token *dagger.Secret,

) {
	rssFeed, err := fetchFeed("https://k1nho.github.io/blog/index.xml")
	if err != nil {
		log.Fatal("could not fetch RSS Feed")
	}
	blogListString := formatBlogList(rssFeed.Channel.Item, 4)

	file := source.Directory(".").File("README.md")
	readmeContent, err := file.Contents(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	start, end := "<!-- BLOG-POST-LIST:START -->", "<!-- BLOG-POST-LIST:END -->"
	startIdx := strings.Index(readmeContent, start)
	endIdx := strings.Index(readmeContent, end)
	if startIdx == -1 || endIdx == -1 || startIdx > endIdx {
		log.Fatal("could not retrieve the start/end of the blog list")
	}

	newContent := readmeContent[:startIdx+len(start)] + "\n" + blogListString + readmeContent[endIdx:]

	ctr := dag.Container().From("alpine/git").
		WithDirectory("/k1nho", source.WithNewFile("README.md", newContent)).
		WithWorkdir("/k1nho").
		WithSecretVariable("GITHUB_TOKEN", token).
		WithExec([]string{"git", "config", "--global", "user.name", "k1nho"}).
		WithExec([]string{"git", "config", "--global", "user.email", "kinhong99@gmail.com"}).
		WithExec([]string{"git", "add", "README.md"}).
		WithExec([]string{"sh", "-c", "git diff --cached --quiet || git commit -m 'cron: update blog posts in README'"}).
		WithExec([]string{"git", "push", "origin", "HEAD"})

	_, err = ctr.ExitCode(context.Background())
	if err != nil {
		log.Fatalf("failed to push to git %v\n", err)
	}
}

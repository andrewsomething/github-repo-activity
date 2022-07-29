package ghra

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/hako/durafmt"
	"golang.org/x/oauth2"
)

type ActivityReport struct {
	RepoActivityReports map[string]*RepoActivityReport
	TotalIssues         int
	TotalPullRequests   int
}

type RepoActivityReport struct {
	Issues       []IssueInfo
	PullRequests []IssueInfo
}

type IssueInfo struct {
	ID     *int64      `json:"id,omitempty"`
	Number *int        `json:"number,omitempty"`
	Title  *string     `json:"title"`
	Author IssueAuthor `json:"author"`
	Repo   string      `json:"repo"`
	URL    *string     `json:"url"`
	Status *string     `json:"status"`
	Age    string      `json:"age"`
}

type IssueAuthor struct {
	DisplayName *string `json:"title"`
	ProfileURL  *string `json:"url"`
}

type RepoActivityService interface {
	FetchIssues(string) (*[]IssueInfo, error)
	BuildQuery(string) string
	BuildReport() (*ActivityReport, error)
}

type GitHubRepoActivityOptions struct {
	Repos   []string
	DaysOld int

	APIEndpoint string
	Token       string
}

type GitHubRepoActivityService struct {
	client  *github.Client
	options *GitHubRepoActivityOptions
}

var _ RepoActivityService = &GitHubRepoActivityService{}

func NewGitHubRepoActivityService(options *GitHubRepoActivityOptions) *GitHubRepoActivityService {
	client := github.NewClient(nil)

	if options.Token != "" {
		tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: options.Token})
		client = github.NewClient(oauth2.NewClient(context.Background(), tokenSource))
	}

	if options.APIEndpoint != "" {
		baseURL, err := url.Parse(options.APIEndpoint)
		if err != nil {
			panic("invalid base URL: " + err.Error())
		}
		client.BaseURL = baseURL
	}

	return &GitHubRepoActivityService{
		client:  client,
		options: options,
	}
}

func (ghra *GitHubRepoActivityService) BuildQuery(issueType string) string {
	var repos []string
	for _, s := range ghra.options.Repos {
		repos = append(repos, fmt.Sprintf("repo:%s", s))
	}

	created := time.Now().AddDate(0, 0, ghra.options.DaysOld*-1).Format("2006-01-02")
	query := fmt.Sprintf("is:%s %s created:>=%s", issueType, strings.Join(repos, " "), created)

	return query
}

func (ghra *GitHubRepoActivityService) FetchIssues(issueType string) (*[]IssueInfo, error) {
	ctx := context.TODO()
	opt := &github.SearchOptions{
		ListOptions: github.ListOptions{
			PerPage: 200,
		},
	}

	query := ghra.BuildQuery(issueType)

	issueList := []IssueInfo{}
	for {
		result, resp, err := ghra.client.Search.Issues(ctx, query, opt)
		if err != nil {
			return nil, err
		}

		for _, issue := range result.Issues {
			age := durafmt.Parse(time.Since(*issue.CreatedAt).Round(time.Hour * 24))

			info := IssueInfo{
				ID:     issue.ID,
				Number: issue.Number,
				Title:  issue.Title,
				Author: IssueAuthor{
					DisplayName: issue.User.Login,
					ProfileURL:  issue.User.HTMLURL,
				},
				Repo:   strings.TrimPrefix(*issue.RepositoryURL, "https://api.github.com/repos/"),
				URL:    issue.HTMLURL,
				Status: issue.State,
				Age:    age.String(),
			}

			issueList = append(issueList, info)
		}

		if resp.NextPage == 0 {
			break
		}
		opt.ListOptions.Page = resp.NextPage
	}

	return &issueList, nil
}

func (ghra *GitHubRepoActivityService) BuildReport() (*ActivityReport, error) {
	issues, err := ghra.FetchIssues("issue")
	if err != nil {
		return nil, err
	}

	prs, err := ghra.FetchIssues("pr")
	if err != nil {
		return nil, err
	}

	repoReports := make(map[string]*RepoActivityReport)
	for _, i := range *issues {
		if repoReports[i.Repo] == nil {
			repoReports[i.Repo] = &RepoActivityReport{}
		}
		repoReports[i.Repo].Issues = append(repoReports[i.Repo].Issues, i)
	}

	for _, p := range *prs {
		if repoReports[p.Repo] == nil {
			repoReports[p.Repo] = &RepoActivityReport{}
		}
		repoReports[p.Repo].PullRequests = append(repoReports[p.Repo].PullRequests, p)
	}

	return &ActivityReport{
		RepoActivityReports: repoReports,
		TotalIssues:         len(*issues),
		TotalPullRequests:   len(*prs),
	}, nil
}

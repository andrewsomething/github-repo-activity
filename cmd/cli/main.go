package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	ghra "github.com/andrewsomething/github-repo-activity/repo-activity"
)

var (
	version string
	commit  string

	repos       = flag.String("repos", "", "A comma seperated list GitHub repositories (required)")
	days        = flag.Int("days", 14, "The number of days to cover in the report")
	endpoint    = flag.String("api-endpoint", "", "API endpoint for use with GitHub Enterprise")
	token       = flag.String("token", os.Getenv("GITHUB_TOKEN"), "GitHub API token")
	versionFlag = flag.Bool("version", false, "Print version")
)

func main() {
	flag.Parse()

	if *versionFlag {
		if version == "" {
			version = "dev"
		}
		fmt.Printf("Version: %s\nCommit: %s\n", version, commit)
		os.Exit(0)
	}

	if *repos == "" {
		fmt.Println("Must set at least one repo...")
		flag.Usage()
		os.Exit(1)
	}

	options := &ghra.GitHubRepoActivityOptions{
		Repos:       strings.Split(*repos, ","),
		DaysOld:     *days,
		APIEndpoint: *endpoint,
		Token:       *token,
	}

	service := ghra.NewGitHubRepoActivityService(options)
	report, err := service.BuildReport()
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 8, 8, 0, '\t', 0)

	for repo, activity := range report {
		fmt.Fprintf(w, "\n## Repo: %s\n\n", repo)
		fmt.Fprintf(w, "### New issues opened in the past %d days\n\n", options.DaysOld)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t\n", "Number", "Status", "Age", "Author", "Title", "URL")
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t\n", "----", "----", "----", "----", "----", "----")
		for _, i := range activity.Issues {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n", *i.Number, *i.Status, i.Age, *i.Author.DisplayName, *i.Title, *i.URL)
		}
		fmt.Fprintf(w, "\n")

		fmt.Fprintf(w, "### New PRs opened in the past %d days\n\n", options.DaysOld)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t\n", "Number", "Status", "Age", "Author", "Title", "URL")
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t\n", "----", "----", "----", "----", "----", "----")
		for _, p := range activity.PullRequests {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n", *p.Number, *p.Status, p.Age, *p.Author.DisplayName, *p.Title, *p.URL)
		}
		fmt.Fprintf(w, "\n")
	}

	w.Flush()
}

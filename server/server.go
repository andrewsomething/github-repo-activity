package server

import (
	"context"
	"html/template"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	ghra "github.com/andrewsomething/github-repo-activity/repo-activity"
)

const (
	defaultDays = 14
	defaultPort = "3000"
)

// Server is the interface for the server.
type Server interface {
	Start() error
	Shutdown(ctx context.Context) error

	Report(w http.ResponseWriter, r *http.Request)
}

// Options hold options for the server.
type Options struct {
	Log         *log.Logger
	Repos       []string
	DaysOld     int
	APIEndpoint string
	Token       string
	Port        string
}

type server struct {
	options    *ghra.GitHubRepoActivityOptions
	logger     *log.Logger
	httpServer *http.Server
}

type pageData struct {
	Days   int
	Repos  []string
	Report map[string]*ghra.RepoActivityReport
}

// NewServer initializes a new server.
func NewServer(opts Options) (Server, error) {
	if opts.DaysOld == 0 {
		opts.DaysOld = defaultDays
	}

	if opts.Port == "" {
		opts.Port = defaultPort
	}

	if opts.Log == nil {
		opts.Log = log.New()
	}

	router := mux.NewRouter()
	srv := &server{
		options: &ghra.GitHubRepoActivityOptions{
			Repos:       opts.Repos,
			DaysOld:     opts.DaysOld,
			APIEndpoint: opts.APIEndpoint,
			Token:       opts.Token,
		},
		logger: opts.Log,
		httpServer: &http.Server{
			Addr:    ":" + opts.Port,
			Handler: router,
		},
	}
	reportHandler := http.HandlerFunc(srv.Report)
	router.HandleFunc("/", reportHandler)

	return srv, nil
}

// Start starts the server.
func (srv *server) Start() error {
	srv.logger.Infof("listening on %s", srv.httpServer.Addr)
	return srv.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (srv *server) Shutdown(ctx context.Context) error {
	return srv.httpServer.Shutdown(ctx)
}

func (srv *server) Report(w http.ResponseWriter, r *http.Request) {
	srv.logger.WithFields(log.Fields{
		"host":   r.Host,
		"method": r.Method,
		"path":   r.RequestURI,
	}).Info("request received")

	query := r.URL.Query()
	daysQuery := query.Get("days")
	if daysQuery != "" {
		days, err := strconv.Atoi(daysQuery)
		if err == nil {
			srv.options.DaysOld = days
		}
	}

	funcMap := template.FuncMap{
		"deref": deref,
	}
	tmpl := template.Must(template.New("page").Funcs(funcMap).Parse(page))

	service := ghra.NewGitHubRepoActivityService(srv.options)
	report, err := service.BuildReport()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	data := pageData{
		Days:   srv.options.DaysOld,
		Repos:  srv.options.Repos,
		Report: report,
	}

	tmpl.Execute(w, data)
}

func deref(s *string) string {
	if s != nil {
		return *s
	}

	return ""
}

const page = `{{ $days := .Days }}
{{ $report := .Report }}
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>GitHub Activity Report</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bulma@0.9.1/css/bulma.min.css">

  <script type='text/javascript'>
  function daysSubmit(){
    document.getElementById('days-select').submit();
  }
  </script>
</head>

<body>
  <section class="hero is-link">
    <div class="hero-body">
      <div class="columns is-vcentered">
        <div class="column is-8">
          <h1 class="title">GitHub Activity Report</h1>
        </div>
        <div class="column">

          <div class="control is-pulled-right">
            <form id="days-select" action="/" method='GET' onchange="daysSubmit()">
              <div class="select">
                <select name="days">
                  <option value="{{ $days }}">{{ $days }} Days</option>
                  <option value="7">7 Days</option>
                  <option value="14">14 Days</option>
                  <option value="30">30 Days</option>
                  <option value="60">60 Days</option>
                  <option value="90">90 Days</option>
                </select>
              </div>
            </form>
          </div>
        </div>
      </div>
    </div>
  </section>

  {{ range $repo := .Repos }}
  <section class="section">
    <div class="box" id={{ $repo }}>
      <h1 class="title"> Repo: <a href="https://github.com/{{ $repo }}">{{ $repo }}</a></h1>
      <h3 class="subtitle">New issues opened in the past {{ $days }} days</h3>
      <div class="block">
      {{ range $r, $activity := $report }}
        {{ if eq $repo $r }}
        {{if not $activity.Issues}}
          <h4>No new issues in time range...</h4>
        {{ else }}
        <div id="{{ $r }}-issues" class="block">
          <table class="table is-hoverable">
            <thead>
              <tr>
                <th>#</th>
                <th>Status</th>
                <th>Age</th>
                <th>Author</th>
                <th>Title</th>
              </tr>
            </thead>
            {{ range  $i := $activity.Issues }}
              <tbody>
                <tr>
                  <td><a href={{ $i.URL }}>{{ $i.Number }}</a></td>
                  <td>
                    {{ if eq ($i.Status | deref) "open" }}
                      <span class="tag is-success">
                    {{ else if eq ($i.Status | deref) "closed" }}
                      <span class="tag is-danger">
                    {{ else }}
                      <span class="tag">
                    {{ end }}
                    {{ $i.Status }}
                    </span>
                  </td>
                  <td>{{ $i.Age }}</td>
                  <td><a href={{ $i.Author.ProfileURL }}>{{ $i.Author.DisplayName }}</a></td>
                  <td><a href={{ $i.URL }}>{{ $i.Title }}</a></td>
                </tr>
              </tbody>
            {{ end }}
          </table>
        </div>
        {{ end }}
        </div>
        {{ end }}
      {{ end }}

      <h3 class="subtitle">New PRs opened in the past {{ $days }} days</h3>
      <div class="block">
      {{ range $r, $activity := $report }}
        {{ if eq $repo $r }}
        {{if not $activity.PullRequests}}
        <h4>No new PRs in time range...</h4>
        {{ else }}
        <div id="{{ $r }}-prs" class="block">
          <table class="table is-hoverable">
            <thead>
              <tr>
                <th>#</th>
                <th>Status</th>
                <th>Age</th>
                <th>Author</th>
                <th>Title</th>
              </tr>
            </thead>
            {{ range  $pr := $activity.PullRequests }}
              <tbody>
                <tr>
                  <td><a href={{ $pr.URL }}>{{ $pr.Number }}</a></td>
                  <td>
                    {{ if eq ($pr.Status | deref) "open" }}
                      <span class="tag is-success">
                    {{ else if eq ($pr.Status | deref) "closed" }}
                      <span class="tag is-danger">
                    {{ else }}
                      <span class="tag">
                    {{ end }}
                    {{ $pr.Status }}
                    </span>
                  </td>
                  <td>{{ $pr.Age }}</td>
                  <td><a href={{ $pr.Author.ProfileURL }}>{{ $pr.Author.DisplayName }}</a></td>
                  <td><a href={{ $pr.URL }}>{{ $pr.Title }}</a></td>
                </tr>
              </tbody>
            {{ end }}
          </table>
        </div>
        {{ end }}
        </div>
        {{ end }}
      {{ end }}
    </div>
  </section>
  {{ end }}
</body>
`

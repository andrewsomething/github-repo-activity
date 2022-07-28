package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/andrewsomething/github-repo-activity/server"
)

func main() {
	endpoint := os.Getenv("GITHUB_ENDPOINT")
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		log.Fatal("GitHub API token not configured")
	}

	repos := os.Getenv("REPORT_REPOS")
	if repos == "" {
		log.Fatal("Must set at least one repo...")
	}

	var (
		err     error
		daysOld int
	)
	days := os.Getenv("REPORT_DAYS")
	if days != "" {
		daysOld, err = strconv.Atoi(days)
		if err != nil {
			log.WithError(err).Fatal("can not parse REPORT_DAYS")
		}
	}

	port := os.Getenv("PORT")

	ll := log.New()

	options := server.Options{
		Repos:       strings.Split(repos, ","),
		DaysOld:     daysOld,
		APIEndpoint: endpoint,
		Token:       token,
		Port:        port,
		Log:         ll,
	}

	srv, err := server.NewServer(options)
	if err != nil {
		log.WithError(err).Fatal("failed build server")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	startShutdown := make(chan struct{})
	go func() {
		<-c
		log.Info("shutting down")
		close(startShutdown)
		time.Sleep(20 * time.Second)
		cancel()
	}()

	var group errgroup.Group
	group.Go(func() error {
		<-startShutdown
		log.Info("stopping server")
		err := srv.Shutdown(ctx)
		if err != nil {
			return fmt.Errorf("failed to stop server: %s", err)
		}
		log.Info("server stopped")
		return nil
	})
	group.Go(func() error {
		err := srv.Start()
		if err != http.ErrServerClosed {
			return fmt.Errorf("failed to start server: %s", err)
		}
		return nil
	})

	log.Info("starting server")
	if err := group.Wait(); err != nil {
		log.WithError(err).Fatal()
	}
	log.Info("shutdown completed")
}

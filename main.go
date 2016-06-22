package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"runtime"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v2"
)

type Data struct {
	commentDuration int `json:"commentDuration"`
	submitDuration  int `json:"submitDuration"`
	prID            int `json:"prID"`
}

type Config struct {
	AccessToken string `yaml:"AccessToken"` 
	Search      struct {
		User string `yaml:"User"`
		Repo string `yaml:"Repo"`
	} `yaml:"Search"`
	TimeLine struct {
		Start string `yaml:"Start"`
		End   string `yaml:"End"`
	} `yaml:"TimeLine"`
	OutputFile string `yaml:"OutputFile"`
}

type agent struct {
	config      Config
	startTime   time.Time
	endTime     time.Time
	client      *github.Client
	currentTime time.Time
	file        *os.File
}

func New() *agent {

	a := &agent{}
	source, err := ioutil.ReadFile("config.yaml")
	check(err)

	err = yaml.Unmarshal(source, &(a.config))
	check(err)

	fmt.Printf("Your token value: %#v\n", a.config.AccessToken)

	a.startTime, err = time.Parse("2006-01-02", a.config.TimeLine.Start)
	check(err)

	a.endTime, err = time.Parse("2006-01-02", a.config.TimeLine.End)
	check(err)

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: a.config.AccessToken},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	a.client = github.NewClient(tc)

	currentTime := time.Now()
	currentTime.Format("2006-01-02")
	a.currentTime = currentTime

	a.file, err = os.Create(a.config.OutputFile)
	check(err)

	return a
}

func main() {

	var (
		breakLoop bool
		wg        sync.WaitGroup
	)

	a := New()
	opt := &github.PullRequestListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
		State:       "all",
	}
	breakLoop = false

	for {
		prs, resp, err := a.client.PullRequests.List(a.config.Search.User, a.config.Search.Repo, opt)
		
		if _, ok := err.(*github.RateLimitError); ok {
			log.Println("Hit rate limit. Try after an hour.")
		} else {
			check(err)
		}

		for i := 0; i < len(prs); i++ {
			current := *prs[i].CreatedAt

			if current.Before(a.startTime) {
				if current.Before(a.endTime) {
					breakLoop = true
					break
					fmt.Println("break")
				}
				wg.Add(1)

				go a.process(current, prs[i].Number, prs[i].ClosedAt, prs[i].MergedAt, getName(prs[i].User), getName(prs[i].Assignee), prs[i].State, &wg)
				time.Sleep(1000 * time.Millisecond)
			}
		}

		if resp.NextPage == 0 {
			break
		}

		opt.ListOptions.Page = resp.NextPage

		if breakLoop {
			break
		}
	}

	wg.Wait()
	defer a.file.Close()
}

func (a *agent) process(createdAt time.Time, id *int, closedAt *time.Time, mergedAt *time.Time, user string, assignee string, status *string, wg *sync.WaitGroup) {

	var (
		commentCreatedAt time.Time
		commentDuration  time.Duration
		submitDuration   time.Duration
	)

	optComments := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 2},
	}
	comments, _, err := a.client.Issues.ListComments(a.config.Search.User, a.config.Search.Repo, *id, optComments)
	
	if _, ok := err.(*github.RateLimitError); ok {
		log.Println("hit rate limit. Try after an hour.")
		os.Exit(0)
	} else {
		check(err)
	}

	if len(comments) > 0 {
		commentCreatedAt = *comments[0].CreatedAt
		commentDuration = commentCreatedAt.Sub(createdAt)

		if closedAt != nil {
			submitDuration = closedAt.Sub(commentCreatedAt)
		} else if mergedAt != nil {
			submitDuration = mergedAt.Sub(commentCreatedAt)
		} else {
			if a.endTime.Sub(commentCreatedAt) > 0 {
				submitDuration = a.currentTime.Sub(commentCreatedAt)
			} else {
				submitDuration = commentCreatedAt.Sub(a.endTime)
			} //negative
		}
	} else {
		if closedAt != nil {
			commentDuration = 0
			submitDuration = closedAt.Sub(createdAt)
		} else if mergedAt != nil {
			commentDuration = 0
			submitDuration = mergedAt.Sub(createdAt)
		} else {
			commentDuration = a.currentTime.Sub(createdAt)
			submitDuration = 0
		}
	}

	_commentDuration := strconv.Itoa(int(commentDuration.Hours() / 24))
	_submitDuration := strconv.Itoa(int(submitDuration.Hours() / 24))
	_prID := strconv.Itoa(*id)
	created_on := strings.Split(createdAt.String(), " ")

	message := `{` +
		`"commentDuration": ` + _commentDuration +
		`, "submitDuration": ` + _submitDuration +
		`, "prID": "` + _prID +
		`", "status": "` + (*status) +
		`", "assignee": "` + assignee +
		`", "createdBy": "` + user +
		`", "created_on": "` + created_on[0] +
		`"}`

	_, err = a.file.Write([]byte(message + "\n"))
	check(err)

	fmt.Println(message)

	wg.Done()
}

func getName(user *github.User) string {

	if user != nil {
		return *user.Login
	} else {
		return "null"
	}

}

func check(err error) {
	if err != nil {
		pc, _, line, _ := runtime.Caller(1)
    		message := fmt.Sprintf("[error] %s[%d] %s", runtime.FuncForPC(pc).Name(), line, err.Error())
    		log.Println(message)
    		os.Exit(0)
	}
}

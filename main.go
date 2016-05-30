package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"sync"
	"time"
	"strconv"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"gopkg.in/olivere/elastic.v3"
	"gopkg.in/yaml.v2"
)

type Data struct {
	mDays int   `json:"mDays"`
	nDays int   `json:"nDays"`
	prID  int   `json:"prID"`
}

type Config struct {
	AccessToken string `yaml:"AccessToken"` //mapping of the name.
	Search      struct {
		User string `yaml:"User"`
		Repo string `yaml:"Repo"`
	} `yaml:"Search"`
	TimeLine struct {
		Start string `yaml:"Start"`
		End   string `yaml:"End"`
	} `yaml:"TimeLine"`
	Elasticsearch string `yaml:"Elasticsearch"`
}

type agent struct {
	config    Config
	startTime time.Time
	endTime   time.Time
	client    *github.Client
	esClient  *elastic.Client
}

func New() *agent {

	a := &agent{}
	source, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		panic(err)
	}

	err = yaml.Unmarshal(source, &(a.config))
	if err != nil {
		panic(err)
	}

	fmt.Printf("Your token value: %#v\n", a.config.TimeLine.Start)

	a.startTime, err = time.Parse("2006-01-02", a.config.TimeLine.Start)
	if err != nil {
		panic(err)
	}
	a.endTime, err = time.Parse("2006-01-02", a.config.TimeLine.End)
	if err != nil {
		panic(err)
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: a.config.AccessToken},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	a.client = github.NewClient(tc)

	a.esClient, err = elastic.NewClient(
		elastic.SetURL(a.config.Elasticsearch),
		elastic.SetSniff(false),
	)
	if err != nil {
		panic(err)
	}
	fmt.Println(a.config.Elasticsearch)
	info, code, err := a.esClient.Ping(a.config.Elasticsearch).Do()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Elasticsearch returned with code %d and version %s", code, info.Version.Number)

	return a
}

func main() {

	var (
		breakLoop bool
		wg        sync.WaitGroup
	)
	a := New()

	opt := &github.PullRequestListOptions{
		ListOptions: github.ListOptions{PerPage: 1000},
		State:       "all",
	}
	breakLoop = false

	for {
		prs, resp, err := a.client.PullRequests.List(a.config.Search.User, a.config.Search.Repo, opt)

		if _, ok := err.(*github.RateLimitError); ok {
			log.Println("hit rate limit. Try after an hour.")
		} else if err != nil {
			panic(err)
		}
		if resp.NextPage == 0 {
			break
		}

		opt.ListOptions.Page = resp.NextPage

		for i := 0; i < len(prs); i++ {
			current := *prs[i].CreatedAt
			if current.Before(a.endTime) {
				if current.Before(a.startTime) {
					breakLoop = true
					break
				}
				wg.Add(1)
				go a.process(current, prs[i].Number, prs[i].ClosedAt, prs[i].MergedAt, &wg)
			}
		}

		if breakLoop {
			break
		}
	}
	wg.Wait()
}

func (a *agent) process(createdAt time.Time, id *int, closedAt *time.Time, mergedAt *time.Time, wg *sync.WaitGroup) {
	var (
		commentCreatedAt time.Time
		mDays            time.Duration
		nDays            time.Duration
	)

	opt := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 2},
	}
	comments, _, err := a.client.Issues.ListComments(a.config.Search.User, a.config.Search.Repo, *id, opt)

	if _, ok := err.(*github.RateLimitError); ok {
		log.Println("hit rate limit. Try after an hour.")
	} else if err != nil {
		panic(err)
	}

	if len(comments) > 0 {
		commentCreatedAt = *comments[0].CreatedAt
		mDays = commentCreatedAt.Sub(createdAt)
		if closedAt != nil {
			nDays = closedAt.Sub(commentCreatedAt)
		} else if mergedAt != nil {
			nDays = mergedAt.Sub(commentCreatedAt)
		} else {
			if a.endTime.Sub(commentCreatedAt) > 0 {
				nDays = a.endTime.Sub(commentCreatedAt)
			} else {
				nDays = commentCreatedAt.Sub(a.endTime)
			} //negative
		}
	} else {
		if closedAt != nil {
			mDays = 0
			nDays = closedAt.Sub(createdAt)
		} else if mergedAt != nil {
			mDays = 0
			nDays = mergedAt.Sub(createdAt)
		} else {
			mDays = a.endTime.Sub(createdAt)
			nDays = 0
		}
	}
	//fmt.Printf("mdays : %v  ndays : %v", int(mDays.Hours()/24), int(nDays.Hours()/24))
	//fmt.Println()

	exists, err := a.esClient.IndexExists("test").Do()
	if err != nil {
		panic(err)
	}
	if !exists {
		// Create a new index.
		_, err := a.esClient.CreateIndex("test").Do()
		if err != nil {
			panic(err)
		}
	}
	_mDays := strconv.Itoa(int(mDays.Hours()/24))
	_nDays := strconv.Itoa(int(nDays.Hours()/24))
	_id := strconv.Itoa(*id)

	// Index a tweet (using JSON serialization)
	message := `{"mDays": `+ _mDays +`, "nDays": `+ _nDays +`, "prID": `+ _id +`}`
	fmt.Println(message)
	put1, err := a.esClient.Index().
		Index("test").
		Type("PullRequests").
		Id(_id).
		BodyJson(message).
		Do()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Indexed message %s to index %s, type %s\n", put1.Id, put1.Index, put1.Type)
	wg.Done()
}

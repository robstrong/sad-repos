package analyze

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/go-github/github"

	"golang.org/x/oauth2"
)

const (
	sentimentEndpoint = "http://sentiment.vivekn.com/api/batch/"
)

type Sentiment struct {
	endpoint    string
	repo        string
	githubToken string
	client      *http.Client
}

func New(githubToken string) *Sentiment {
	return &Sentiment{
		endpoint:    sentimentEndpoint,
		client:      &http.Client{},
		githubToken: githubToken,
	}
}

//analyzes that last X commits and returns the sentiment analysis
func (s *Sentiment) Analyze(repoOwner, repoName string) ([]Analysis, error) {
	log.Printf("Analyzing: %s/%s\n", repoOwner, repoName)
	msgs, err := getMessages(repoOwner, repoName, s.githubToken)
	if err != nil {
		return nil, err
	}
	res, err := s.getAnalysis(msgs)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func getMessages(repoOwner, repoName string, token string) ([]string, error) {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	client := github.NewClient(tc)

	msgs := []string{}
	opt := &github.CommitsListOptions{
		ListOptions: github.ListOptions{
			Page:    1,
			PerPage: 100,
		},
	}
	disregarded := 0
	for {
		commits, resp, err := client.Repositories.ListCommits(
			repoOwner,
			repoName,
			opt,
		)
		if err != nil {
			return nil, err
		}
		for _, commit := range commits {
			//skip merge commits
			if strings.HasPrefix(*commit.Commit.Message, "Merge pull request") ||
				strings.HasPrefix(*commit.Commit.Message, "Merge branch") {
				disregarded++
				continue
			}
			msgs = append(msgs, *commit.Commit.Message)
		}
		if resp.NextPage == 0 {
			break
		}
		opt.ListOptions.Page = resp.NextPage
	}
	return msgs, nil
}

func (s *Sentiment) getAnalysis(msgs []string) ([]Analysis, error) {
	res := []Analysis{}
	payload := []string{}
	for _, msg := range msgs {
		t := append(payload, msg)
		b, err := json.Marshal(t)
		if err != nil {
			return nil, err
		}
		//if payload is over 1MB, send an api request with the payload minus the last msg
		if binary.Size(b) > 1000000 {
			rs, err := s.bulkAnalyze(payload)
			if err != nil {
				return nil, err
			}
			for _, r := range rs {
				res = append(res, r)
			}
			//add last message to next payload
			payload = []string{msg}
		} else {
			payload = append(payload, msg)
		}
	}

	if len(payload) > 0 {
		var err error
		rs, err := s.bulkAnalyze(payload)
		if err != nil {
			return nil, err
		}
		for _, r := range rs {
			res = append(res, r)
		}
	}
	return res, nil
}

func (s *Sentiment) bulkAnalyze(msgs []string) (res []Analysis, err error) {
	body, err := json.Marshal(msgs)
	if err != nil {
		return res, err
	}
	resp, err := s.client.Post(s.endpoint, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return res, errors.New("http: " + err.Error())
	}
	js, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return res, errors.New("http body: " + err.Error())
	}
	as := []Analysis{}
	err = json.Unmarshal(js, &as)
	if err != nil {
		return res, errors.New("json err: " + err.Error() + "\njson: " + string(js))
	}
	for _, a := range as {
		conf, err := strconv.ParseFloat(a.ConfidenceStr, 32)
		if err != nil {
			return res, err
		}
		a.Confidence = float32(conf)
		res = append(res, a)
	}
	return res, nil
}

type Analysis struct {
	Sentiment     string `json:"result"`
	ConfidenceStr string `json:"confidence"`
	Confidence    float32
}

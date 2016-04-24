package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/olekukonko/tablewriter"
	"github.com/robstrong/sad-repos/analyze"
)

var (
	githubToken   = kingpin.Arg("token", "Github Token for accessing commit history").Required().String()
	githubRepos   = kingpin.Arg("repos", "Github Repositories to analyze in the format 'owner/repository'").Required().Strings()
	minConfidence = kingpin.Flag("confidence", "Minimum required confidence level").Default("75").Float32()
)

func main() {
	kingpin.Parse()
	s := analyze.New(*githubToken)
	if *minConfidence < 0 || *minConfidence > 100 {
		log.Fatal("confidence must be between 0-100 inclusively")
	}
	log.Printf("Minimum Confidence: %f%%\n", *minConfidence)
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Repo", "Positive Commits", "Negative Commits", "Commits Below Confidence", "Pos-to-Neg Ratio"})
	repos := NewRepos(*githubRepos)
	for _, repo := range repos {
		res, err := s.Analyze(repo.Owner, repo.Name)
		if err != nil {
			log.Fatal(err)
		}
		var numPos, numNeg, numBelow int
		for _, a := range res {
			if a.Confidence < *minConfidence {
				numBelow++
				continue
			}
			switch a.Sentiment {
			case "Positive":
				numPos++
			case "Negative":
				numNeg++
			}
		}
		table.Append([]string{
			repo.FullName(),
			strconv.Itoa(numPos),
			strconv.Itoa(numNeg),
			strconv.Itoa(numBelow),
			fmt.Sprintf("%f", float32(numPos)/float32(numNeg)),
		})
	}

	table.Render()
}

func NewRepos(r []string) []Repo {
	var repos []Repo
	for _, repo := range r {
		parts := strings.Split(repo, "/")
		if len(parts) != 2 {
			continue
		}
		repos = append(repos, Repo{parts[0], parts[1]})
	}
	return repos
}

type Repo struct {
	Owner string
	Name  string
}

func (r Repo) FullName() string {
	return fmt.Sprintf("%s/%s", r.Owner, r.Name)
}

func avg(nums []float32) float32 {
	sum := float32(0)
	for _, x := range nums {
		sum += x
	}
	return sum / float32(len(nums))
}

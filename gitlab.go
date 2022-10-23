package main

import (
	"flag"
	"fmt"
	"github.com/tidwall/gjson"
	"golang.org/x/exp/maps"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var gitlabURL = os.Getenv("G_GITLAB_URL") //base gitlab url
var token = os.Getenv("G_GITLAB_TOKEN")   //personal token
var users = os.Getenv("G_WATCH_USERS")    //comma separated
var groups = os.Getenv("G_GITLAB_GROUPS") //comma separated

// invert time zone here
var timeZone = 0

func main() {
	//get input value from command line
	inUsers := flag.String("u", users, "list of username separated by a comma")
	//isVerbose := flag.Bool("v", false, "is verbose mode")
	inDate := flag.String("d", time.Now().Format("02-01"), "date format DD-MM-YYYY or DD-MM - default is current date")
	flag.Parse()

	//prepare parameters
	currentYear := time.Now().Year()
	if len(*inDate) < 10 {
		*inDate = fmt.Sprintf("%v-%v", *inDate, currentYear)
	}
	timeFrom, _ := time.Parse("02-01-2006", *inDate)
	timeFrom = timeFrom.Add(time.Hour * time.Duration(timeZone))
	timeTo := timeFrom.Add(time.Hour * 24)

	projectIds := getListProjectId(groups)
	if len(projectIds) == 0 {
		log.Fatalln("Can't find any project id!")
	}
	checkCommit(projectIds, *inUsers, timeFrom, timeTo, *inDate)
}

func checkCommit(projectIds []string, users string, from time.Time, to time.Time, date string) {
	var userCommits = make(map[string][]string)
	for _, projectId := range projectIds {
		maps.Copy(userCommits, getCommitOfProject(projectId, 100, "1", from, to))
	}
	log.Printf("%v", userCommits)
	userInfos := getUserInfos(users)
	log.Printf("%v", userInfos)
	for key, value := range userCommits {
		userName := userInfos[key]
		if userName != "" {
			log.Printf("[%v] make [%v] commit on %v", key, len(value), date)
		}
	}
}

func formatDate(time time.Time) string {
	return time.Format("2006-01-02T15:04:05Z")
}

func getCommitOfProject(id string, itemPerPage int, page string, from time.Time, to time.Time) map[string][]string {
	var userCommits = make(map[string][]string)

	url := fmt.Sprintf("%v/api/v4/projects/%v/repository/commits?since=%v&until=%v&per_page=%v&page=%v", gitlabURL, id, formatDate(from), formatDate(to), itemPerPage, page)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatalln(err)
	}
	req.Header.Set("PRIVATE-TOKEN", token)

	log.Printf("[DEBUG] url request %v", url)
	resp, err := http.DefaultClient.Do(req)
	//We Read the response body on the line below.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	result := gjson.GetManyBytes(body, "#.id", "#.author_email")
	if !result[0].Exists() {
		return userCommits
	}
	for index, commitId := range result[0].Array() {
		author := result[1].Array()[index].String()
		userCommits[author] = append(userCommits[author], commitId.String())
	}
	header := resp.Header
	nextPage := header.Get("X-Next-Page")
	if nextPage != "" {
		maps.Copy(userCommits, getCommitOfProject(id, itemPerPage, nextPage, from, to))
	}
	return userCommits
}

func getListProjectId(groupProjects string) []string {
	var projectIds []string
	for _, group := range strings.Split(groupProjects, ",") {
		groupProjectIds := getAllProjectIdOfGroup(group, 100, "1", false)
		projectIds = append(projectIds, groupProjectIds...)
	}
	return projectIds
}

func getAllProjectIdOfGroup(groupName string, itemPerPage int, page string, isArchived bool) []string {
	var projectIds []string
	url := fmt.Sprintf("%v/api/v4/groups/%v/projects?archived=%v&per_page=%v&page=%v", gitlabURL, groupName, isArchived, itemPerPage, page)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatalln(err)
	}
	req.Header.Set("PRIVATE-TOKEN", token)

	log.Printf("[DEBUG] url request %v", url)
	resp, err := http.DefaultClient.Do(req)
	//We Read the response body on the line below.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	result := gjson.GetBytes(body, "#.id")
	if !result.Exists() {
		return projectIds
	}
	for _, projectId := range result.Array() {
		projectIds = append(projectIds, projectId.String())
	}
	header := resp.Header
	nextPage := header.Get("X-Next-Page")
	if nextPage != "" {
		projectIds = append(projectIds, getAllProjectIdOfGroup(groupName, itemPerPage, nextPage, isArchived)...)
	}

	return projectIds
}

func getUserInfos(users string) map[string]string {
	userInfos := make(map[string]string)
	for _, username := range strings.Split(users, ",") {
		url := fmt.Sprintf("%v/api/v4/users/%v", gitlabURL, username)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			log.Fatalln(err)
		}
		req.Header.Set("PRIVATE-TOKEN", token)

		log.Printf("[DEBUG] url request %v", url)
		resp, err := http.DefaultClient.Do(req)
		//We Read the response body on the line below.
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalln(err)
		}
		result := gjson.GetManyBytes(body, "public_email", "name")
		if result[0].Exists() {
			userInfos[result[0].String()] = result[1].String()
		}
	}
	return userInfos
}

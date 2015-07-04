package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
)

// TimeEntry is single time entry item
type TimeEntry struct {
	IssueID int `json:"issue_id"`
	//SpentOn    time.Time `json:"spent_on,omitempty"`
	Hours      float64 `json:"hours"`
	ActivityID int     `json:"activity_id"`
	Comment    string  `json:"comment"`
}

// TimeRequest used for time entry wrapper
type TimeRequest struct {
	TimeEntry `json:"time_entry"`
	User      string `json:"-"`
}

// Issue is used for update issue
type Issue struct {
	StatusID int    `json:"status_id,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

// IssueRequest is used to do a request
type IssueRequest struct {
	Issue `json:"issue"`
	ID    int    `json:"-"`
	User  string `json:"-"`
}

func addTimeEntry(tr *TimeRequest) error {
	payload, err := json.Marshal(tr)
	if err != nil {
		return err
	}

	api := viper.GetString("redmine_apikey")
	url := viper.GetString("redmine_url") + "/time_entries.json"

	return callCurl("POST", tr.User, api, url, string(payload))
}

func changeStatus(i *IssueRequest) error {

	payload, err := json.Marshal(i)
	if err != nil {
		return err
	}

	api := viper.GetString("redmine_apikey")
	url := viper.GetString("redmine_url") + "/issues/%d.json"
	url = fmt.Sprintf(url, i.ID)

	return callCurl("PUT", i.User, api, url, string(payload))
}

// callCurl, yes I can use http.Client but I am lazy.
func callCurl(method, user, apikey, url, payload string) error {
	args := []string{
		"-X",
		strings.ToUpper(method),
		"-H",
		"Content-Type: application/json",
		"-H",
		"X-Redmine-API-Key: " + apikey,
		"-H",
		"X-Redmine-Switch-User: " + user,
		"-d",
		payload,
		url,
	}
	log.Warn(strings.Join(args, " "))
	curl := exec.Command("curl", args...)
	out, err := curl.CombinedOutput()
	log.WithField("command", strings.Join(args, " ")).WithField("output", string(out)).Debug(err)

	if err != nil {
		log.Warn(string(out))
	}

	return err
}

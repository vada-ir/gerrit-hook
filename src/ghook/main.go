package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
)

var tt = regexp.MustCompile("(@[0-9][0-9hm]+)")
var issue = regexp.MustCompile("(#[0-9]+)")
var umail = regexp.MustCompile("([^<(]+)[<(]([^>)]+)")

func main() {
	initConfig()

	var cmd = make(map[string]string)

	s := strings.Split(os.Args[0], "/")
	cmd["action"] = s[len(s)-1]

	args := os.Args[1:]
	for i := range args {
		if i%2 == 0 {
			key := strings.Trim(args[i], "-")
			key = strings.Replace(key, "-", "_", -1)
			value := ""
			if len(args) > i+1 {
				value = args[i+1]
			}
			cmd[key] = value
		}
	}

	// If the commit is not available just ignore the hook
	if _, ok := cmd["commit"]; !ok {
		return
	}

	cmd = addExtraField(cmd)

	// Ok the commit has no isue attaced, exit. its not correct
	_, ok := cmd["original_issue"]
	if !ok {
		os.Exit(1)
	}
	var (
		commiter string
		msg      []byte
		err      error
	)
	commiter, msg, err = getCommitData(cmd)
	if err != nil {
		logrus.Warn(err)
		return
	}

	out, err := execGitCommand(viper.GetString("root_path"), "commit", "--allow-empty", "--message", string(msg), "--author", commiter)
	if err != nil {
		logrus.Warn(err)
		logrus.Warn(string(out))
	}

	err = nil
	if cmd["action"] == "patchset-created" {
		err = pathsetCreated(cmd)
	} else if cmd["action"] == "comment-added" {
		err = commentAdded(cmd)
	} else if cmd["action"] == "change-merged" {
		err = changeMerged(cmd)
	}

	if err != nil {
		logrus.Warn(err)
		os.Exit(1)
	}

}

func initConfig() {
	usr, err := user.Current()
	if err != nil {
		logrus.Warn(err)
	}
	viper.SetConfigName("main")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(usr.HomeDir + "/ghooks/")

	// Root path of the ghook repo to commit and push
	_ = viper.BindEnv("root_path", "ROOT_PATH")
	viper.SetDefault("root_path", usr.HomeDir+"/ghooks")

	// Git directory from gerrit
	_ = viper.BindEnv("git_dir", "GIT_DIR")

	viper.SetDefault("commiter_name", "gerrit hook")
	viper.SetDefault("commiter_mail", "dev@vada.ir")

	viper.SetDefault("redmine_url", "")
	viper.SetDefault("redmine_apikey", "")
	viper.SetDefault("redmine_status_inprogress", 2)
	viper.SetDefault("redmine_status_inreview", 7)
	viper.SetDefault("redmine_status_resolved", 3)

	viper.SetDefault("redmine_activity_review", 18)

	if err := viper.ReadInConfig(); err != nil {
		logrus.Warn(err)
	}
}

func addExtraField(cmd map[string]string) map[string]string {
	if hash, ok := cmd["commit"]; ok {
		out, err := execGitCommand("", "cat-file", "-p", hash)
		if err == nil {
			reader := bytes.NewReader(out)
			buf := bufio.NewReader(reader)
			gpgsign := regexp.MustCompile("^gpgsig")
			empty := 0
			for {
				l, err := buf.ReadString('\n')
				if err != nil {
					break
				}

				if strings.Trim(l, "\n ") == "" {
					if empty == 0 {
						break
					}
					empty--
				}

				if gpgsign.MatchString(l) {
					empty++
				}
			}

			msg := ""
			for {
				l, err := buf.ReadString('\n')
				if err != nil {
					break
				}
				msg = msg + l + "\n"
			}

			cmd["original_message"] = msg

			res := issue.FindAllString(msg, -1)
			if len(res) > 0 {
				cmd["original_issue"] = res[0]
			}

			res = tt.FindAllString(msg, -1)
			if len(res) > 0 {
				cmd["original_time"] = res[0]
			}

		} else {
			logrus.Warn(err)
		}
	}

	return cmd
}

func getCommitData(cmd map[string]string) (string, []byte, error) {
	commiter := viper.GetString("commiter_name") + " <" + viper.GetString("commiter_mail") + ">"
	b, err := json.MarshalIndent(cmd, "", "    ")
	return commiter, b, err
}

/*
func addFile(cmd map[string]string) error {
	j, err := json.MarshalIndent(cmd, "", "    ")
	if err != nil {
		logrus.Warn(err)
		return err
	}

	t := time.Now()
	format := fmt.Sprintf("/2006/01/02/15-04-05Z07-00-%s.json", cmd["action"])
	path := viper.GetString("root_path") + t.Format(format)

	dir := filepath.Dir(path)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path, []byte(j), 0644)
	if err != nil {
		return err
	}

	out, err := execGitCommand(viper.GetString("root_path"), "add", path)
	if err != nil {
		logrus.Warn(string(out))
	}
	return err
}
*/
func execGitCommand(root string, args ...string) (stdout []byte, err error) {
	cmd := exec.Command("git", args...)
	if root != "" {
		cmd.Dir = root
		// Gerrit pass the GIT_DIR as env variable to the current process, make sure its removed from child process
		cmd.Env = []string{
			"GIT_COMMITTER_NAME=" + viper.GetString("commiter_name"),
			"GIT_COMMITTER_EMAIL=" + viper.GetString("commiter_mail"),
		}
	} else {
		cmd.Env = os.Environ()
	}
	stdout, err = cmd.CombinedOutput()
	return
}

// getUserEmail convert "test <test@test.com>" to test, test@test.com
func getUserEmail(s string) (string, string, error) {
	m := umail.FindAllSubmatch([]byte(s), -1)
	if len(m) != 1 || len(m[0]) != 3 {
		return "", "", errors.New("not matched")
	}

	return string(m[0][1]), string(m[0][2]), nil
}

func pathsetCreated(cmd map[string]string) error {
	owner, _, err := getUserEmail(cmd["uploader"])
	if err != nil {
		return err
	}
	url := cmd["change_url"]
	t := cmd["original_time"]
	msg := cmd["original_message"]
	issue := cmd["original_issue"]
	if issue[0] == '#' {
		issue = issue[1:]
	}

	if len(t) > 1 {
		t = t[1:]
	}

	msg = fmt.Sprintf(`
pathset created

url is %s

the comment is
 %s
`, url, msg)

	i := &IssueRequest{}

	i.User = owner
	i.Notes = msg
	i.ID, err = strconv.Atoi(issue)
	if err != nil {
		return err
	}
	i.StatusID = viper.GetInt("redmine_status_inreview")

	return changeStatus(i)
}

func commentAdded(cmd map[string]string) error {
	owner, _, err := getUserEmail(cmd["author"])
	if err != nil {
		return err
	}
	comment := cmd["comment"]

	var t time.Duration
	res := tt.FindAllString(comment, -1)
	if len(res) > 0 {
		first := res[0]
		first = first[1:]
		t, err = time.ParseDuration(first)
		if err != nil {
			logrus.Warn(err)
		}
	}
	issue := cmd["original_issue"]
	if issue[0] == '#' {
		issue = issue[1:]
	}

	if t > 0 {
		tr := &TimeRequest{}
		//tr.SpentOn = time.Now()
		tr.IssueID, err = strconv.Atoi(issue)
		if err != nil {
			return err
		}
		tr.User = owner
		tr.ActivityID = viper.GetInt("redmine_activity_review")
		tr.Hours = t.Hours()
		tr.Comment = comment
		err = addTimeEntry(tr)
		if err != nil {
			return err
		}
	}

	i := &IssueRequest{}
	i.User = owner
	i.Notes = comment
	i.ID, err = strconv.Atoi(issue)
	if err != nil {
		return err
	}
	return changeStatus(i)
}

func changeMerged(cmd map[string]string) error {
	owner, _, err := getUserEmail(cmd["submitter"])
	if err != nil {
		return err
	}
	issue := cmd["original_issue"]
	if issue[0] == '#' {
		issue = issue[1:]
	}

	i := &IssueRequest{}
	i.User = owner
	i.Notes = fmt.Sprintf(`change submitted, the review thread is %s `, cmd["change_url"])
	i.ID, err = strconv.Atoi(issue)
	if err != nil {
		return err
	}
	return changeStatus(i)
}

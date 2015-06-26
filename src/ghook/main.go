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
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/fzerorubid/go-redmine"
	"github.com/spf13/viper"
)

var r *redmine.Client

var tt = regexp.MustCompile("(@[0-9][0-9hm])")
var issue = regexp.MustCompile("(#[0-9]+)")

func main() {
	initConfig()
	r = redmine.NewClient(viper.GetString("redmine_url"), viper.GetString("redmine_apikey"))

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
	var (
		commiter string
		msg      string
	)
	if cmd["action"] == "patchset_created" {
		var err error
		commiter, msg, err = pathsetCreated(cmd)
		if err != nil {
			logrus.Warn(err)
			return
		}
	} else {
		return
	}

	// Ok the commit has no isue attaced, exit. its not correct
	_, ok := cmd["original_issue"]
	if !ok {
		os.Exit(1)
	}

	out, err := execGitCommand(viper.GetString("root_path"), "commit", "--allow-empty", "--message", string(msg), "--author", commiter)
	if err != nil {
		logrus.Warn(err)
		logrus.Warn(string(out))
	}
}

func initConfig() {
	usr, err := user.Current()
	if err != nil {
		logrus.Warn(err)
	}
	viper.AddConfigPath(usr.HomeDir + "/ghooks")

	// Root path of the ghook repo to commit and push
	_ = viper.BindEnv("root_path", "ROOT_PATH")
	viper.SetDefault("root_path", usr.HomeDir+"/ghooks")

	// Git directory from gerrit
	_ = viper.BindEnv("git_dir", "GIT_DIR")

	viper.SetDefault("commiter_name", "gerrit hook")
	viper.SetDefault("commiter_mail", "dev@vada.ir")
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

func pathsetCreated(cmd map[string]string) (string, string, error) {
	kind, ok := cmd["kind"]
	if !ok {
		return "", "", errors.New("invalid")
	}

	if kind == "NO_CHANGE" {
		return "", "", errors.New("no change")
	}

	owner := cmd["change_owner"]
	url := cmd["change_url"]
	time := cmd["original_time"]
	issue := cmd["original_issue"]

	msg := fmt.Sprintf(`
pathset created

url is %s

refs %s %s
`, url, issue, time)
	return owner, msg, nil
}

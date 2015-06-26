package main

import (
	"os"
	"os/user"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
)

func main() {
	initConfig()

	var cmd = make(map[string]string)

	s := strings.Split(os.Args[0], "/")
	cmd["action"] = s[len(s)-1]

	args := os.Args[1:]
	for i := range args {
		if i%2 == 0 {
			key := strings.Trim(args[i], "-")
			value := ""
			if len(args) > i+1 {
				value = args[i+1]
			}
			cmd[key] = value
		}
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
}

func processTemplate() {

}

package commands

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	flags "github.com/jessevdk/go-flags"
	"github.com/lighttiger2505/lab/cmd"
	"github.com/lighttiger2505/lab/git"
	lab "github.com/lighttiger2505/lab/gitlab"
	"github.com/lighttiger2505/lab/ui"
)

type BrowseType int

const (
	Issue BrowseType = iota
	MergeRequest
	PipeLine
)

var browseTypePrefix = map[string]BrowseType{
	"#": Issue,
	"i": Issue,
	"I": Issue,
	"!": MergeRequest,
	"m": MergeRequest,
	"M": MergeRequest,
	"p": PipeLine,
	"P": PipeLine,
}

type BrowseCommandOption struct {
	BrowseOption *BrowseOption `group:"Global Options"`
}

func newBrowseOptionParser(opt *BrowseCommandOption) *flags.Parser {
	opt.BrowseOption = newBrowseOption()
	parser := flags.NewParser(opt, flags.Default)
	parser.Usage = "browse [options]"
	return parser
}

type BrowseCommand struct {
	Ui        ui.Ui
	Provider  lab.Provider
	GitClient git.Client
	Cmd       cmd.Cmd
}

func (c *BrowseCommand) Synopsis() string {
	return "Browse repository page"
}

func (c *BrowseCommand) Help() string {
	buf := &bytes.Buffer{}
	var browseCommnadOption BrowseCommandOption
	browseOptionParser := newBrowseOptionParser(&browseCommnadOption)
	browseOptionParser.WriteHelp(buf)
	return buf.String()
}

func (c *BrowseCommand) Run(args []string) int {
	var browseCommnadOption BrowseCommandOption
	browseOptionParser := newBrowseOptionParser(&browseCommnadOption)
	// Parse option
	parseArgs, err := browseOptionParser.ParseArgs(args)
	if err != nil {
		c.Ui.Error(err.Error())
		return ExitCodeError
	}

	// Validate option
	browseOption := browseCommnadOption.BrowseOption
	if err := browseOption.IsValid(); err != nil {
		c.Ui.Error(err.Error())
		return ExitCodeError
	}

	// Initialize provider
	if err := c.Provider.Init(); err != nil {
		c.Ui.Error(err.Error())
		return ExitCodeError
	}

	// Getting git remote info
	gitlabRemote, err := c.Provider.GetCurrentRemote()
	if err != nil {
		c.Ui.Error(err.Error())
		return ExitCodeError
	}
	if browseOption.Project != "" {
		namespace, project := browseOption.NameSpaceAndProject()
		gitlabRemote.NameSpace = namespace
		gitlabRemote.Repository = project
	}

	if browseOption.Path != "" {
		path := browseOption.Path
		fullpath, err := filepath.Abs(path)
		if err != nil {
			c.Ui.Error(err.Error())
		}

		if !isFileExist(fullpath) {
			c.Ui.Error(fmt.Sprintf("Not found file or path. Path:%s", fullpath))
			return ExitCodeError
		}

		gitroot, err := git.Root()
		if err != nil {
			c.Ui.Error(err.Error())
			return ExitCodeError
		}

		branch, err := c.GitClient.CurrentRemoteBranch(gitlabRemote)
		gitAbsPath := strings.Replace(strings.Replace(fullpath, gitroot, "", -1), "/", "", 1)
		gitlabPath := gitlabRemote.BranchPath(branch, gitAbsPath)

		browser := searchBrowserLauncher(runtime.GOOS)
		c.Cmd.SetCmd(browser)
		c.Cmd.WithArg(gitlabPath)
		if err := c.Cmd.Spawn(); err != nil {
			c.Ui.Error(err.Error())
			return ExitCodeError
		}

		return ExitCodeOK
	}

	if browseOption.CurrentPath {
		currentDir, err := os.Getwd()
		if err != nil {
			c.Ui.Error(err.Error())
			return ExitCodeError
		}

		gitroot, err := git.Root()
		if err != nil {
			c.Ui.Error(err.Error())
			return ExitCodeError
		}

		branch, err := c.GitClient.CurrentRemoteBranch(gitlabRemote)
		gitAbsPath := strings.Replace(strings.Replace(currentDir, gitroot, "", -1), "/", "", 1)
		gitlabPath := gitlabRemote.BranchPath(branch, gitAbsPath)

		browser := searchBrowserLauncher(runtime.GOOS)
		c.Cmd.SetCmd(browser)
		c.Cmd.WithArg(gitlabPath)
		if err := c.Cmd.Spawn(); err != nil {
			c.Ui.Error(err.Error())
			return ExitCodeError
		}
		return ExitCodeOK
	}

	// Getting browse repository
	var url = ""
	if browseOption.Project != "" {
		url, err = getUrlByUserSpecific(gitlabRemote, parseArgs, gitlabRemote.Domain)
		if err != nil {
			c.Ui.Error(err.Error())
			return ExitCodeError
		}
	} else {
		branch, err := c.GitClient.CurrentRemoteBranch(gitlabRemote)
		if err != nil {
			c.Ui.Error(err.Error())
			return ExitCodeError
		}
		url, err = getUrlByRemote(gitlabRemote, parseArgs, branch)
		if err != nil {
			c.Ui.Error(err.Error())
			return ExitCodeError
		}
	}

	browser := searchBrowserLauncher(runtime.GOOS)

	c.Cmd.SetCmd(browser)
	c.Cmd.WithArg(url)
	if err := c.Cmd.Spawn(); err != nil {
		c.Ui.Error(err.Error())
		return ExitCodeError
	}

	return ExitCodeOK
}

func getUrlByRemote(gitlabRemote *git.RemoteInfo, args []string, branch string) (string, error) {
	if len(args) > 0 {
		// Gitlab resource page
		browseType, number, err := splitPrefixAndNumber(args[0])
		if err != nil {
			return "", err
		}
		return makeGitlabResourceUrl(gitlabRemote, browseType, number), nil
	} else {
		if branch == "master" {
			// Repository top page
			return gitlabRemote.RepositoryUrl(), nil
		} else {
			// Current branch top page
			return gitlabRemote.BranchUrl(branch), nil
		}
	}
}

func getUrlByUserSpecific(gitlabRemote *git.RemoteInfo, args []string, domain string) (string, error) {
	// Browse current repository page
	if gitlabRemote != nil {
		if len(args) > 0 {
			// Gitlab resource page
			browseType, number, err := splitPrefixAndNumber(args[0])
			if err != nil {
				return "", err
			}
			return makeGitlabResourceUrl(gitlabRemote, browseType, number), nil
		} else {
			// Repository top page
			return gitlabRemote.RepositoryUrl(), nil
		}
	} else {
		if domain != "" {
			// Browse current domain page
			return "https://" + domain, nil
		}
	}
	return "", fmt.Errorf("Not found browse url.")
}

func makeGitlabResourceUrl(gitlabRemote *git.RemoteInfo, browseType BrowseType, number int) string {
	var url string
	if number > 0 {
		switch browseType {
		case Issue:
			url = gitlabRemote.IssueDetailUrl(number)
		case MergeRequest:
			url = gitlabRemote.MergeRequestDetailUrl(number)
		case PipeLine:
			url = gitlabRemote.PipeLineDetailUrl(number)
		default:
			url = ""
		}
	} else {
		switch browseType {
		case Issue:
			url = gitlabRemote.IssueUrl()
		case MergeRequest:
			url = gitlabRemote.MergeRequestUrl()
		case PipeLine:
			url = gitlabRemote.PipeLineUrl()
		default:
			url = ""
		}
	}
	return url
}

func searchBrowserLauncher(goos string) (browser string) {
	switch goos {
	case "darwin":
		browser = "open"
	case "windows":
		browser = "cmd /c start"
	default:
		candidates := []string{
			"xdg-open",
			"cygstart",
			"x-www-browser",
			"firefox",
			"opera",
			"mozilla",
			"netscape",
		}
		for _, b := range candidates {
			path, err := exec.LookPath(b)
			if err == nil {
				browser = path
				break
			}
		}
	}
	return browser
}

func splitPrefixAndNumber(arg string) (BrowseType, int, error) {
	for k, v := range browseTypePrefix {
		if strings.HasPrefix(arg, k) {
			numberStr := strings.TrimPrefix(arg, k)
			if numberStr == "" {
				return v, 0, nil
			} else {
				number, err := strconv.Atoi(numberStr)
				if err != nil {
					return 0, 0, errors.New(fmt.Sprintf("Invalid browse number. \"%s\"", numberStr))
				}
				return v, number, nil
			}
		}
	}
	return 0, 0, errors.New(fmt.Sprintf("Invalid arg. %s", arg))
}

func isFileExist(fPath string) bool {
	_, err := os.Stat(fPath)
	return err == nil || !os.IsNotExist(err)
}

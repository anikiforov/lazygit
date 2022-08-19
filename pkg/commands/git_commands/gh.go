package git_commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
)

type GhCommands struct {
	*GitCommon
}

func NewGhCommand(gitCommon *GitCommon) *GhCommands {
	return &GhCommands{
		GitCommon: gitCommon,
	}
}

// https://github.com/cli/cli/issues/2300
func (self *GhCommands) BaseRepo() error {
	return self.cmd.New("git config --local --get-regexp .gh-resolved").Run()
}

// Ex: git config --local --add "remote.origin.gh-resolved" "jesseduffield/lazygit"
func (self *GhCommands) SetBaseRepo(repository string) (string, error) {
	return self.cmd.New(fmt.Sprintf("git config --local --add \"remote.origin.gh-resolved\" \"%s\"", repository)).RunWithOutput()
}

func (self *GhCommands) prList() (string, error) {
	return self.cmd.New("gh pr list --limit 500 --state all --json state,url,number,headRefName,headRepositoryOwner").RunWithOutput()
}

func (self *GhCommands) GithubMostRecentPRs() ([]*models.GithubPullRequest, error) {
	commandOutput, err := self.prList()
	if err != nil {
		return nil, err
	}

	prs := []*models.GithubPullRequest{}
	err = json.Unmarshal([]byte(commandOutput), &prs)
	if err != nil {
		return nil, err
	}

	return prs, nil
}

func GenerateGithubPullRequestMap(prs []*models.GithubPullRequest, branches []*models.Branch, remotes []*models.Remote) map[*models.Branch]*models.GithubPullRequest {
	res := map[*models.Branch]*models.GithubPullRequest{}

	if len(prs) == 0 {
		return res
	}

	remotesToOwnersMap := getRemotesToOwnersMap(remotes)

	if len(remotesToOwnersMap) == 0 {
		return res
	}

	prWithStringKey := map[string]models.GithubPullRequest{}

	for _, pr := range prs {
		prWithStringKey[pr.UserName()+":"+pr.BranchName()] = *pr
	}

	for _, branch := range branches {
		if !branch.IsTrackingRemote() || branch.UpstreamBranch == "" {
			continue
		}

		owner, foundRemoteOwner := remotesToOwnersMap[branch.UpstreamRemote]
		if !foundRemoteOwner {
			continue
		}

		pr, hasPr := prWithStringKey[owner+":"+branch.UpstreamBranch]

		if !hasPr {
			continue
		}

		res[branch] = &pr
	}

	return res
}

func GetRepoInfoFromURL(url string) RepoInformation {
	isHTTP := strings.HasPrefix(url, "http")

	if isHTTP {
		splits := strings.Split(url, "/")
		owner := strings.Join(splits[3:len(splits)-1], "/")
		repo := strings.TrimSuffix(splits[len(splits)-1], ".git")

		return RepoInformation{
			Owner:      owner,
			Repository: repo,
		}
	}

	tmpSplit := strings.Split(url, ":")
	splits := strings.Split(tmpSplit[1], "/")
	owner := strings.Join(splits[0:len(splits)-1], "/")
	repo := strings.TrimSuffix(splits[len(splits)-1], ".git")

	return RepoInformation{
		Owner:      owner,
		Repository: repo,
	}
}

func getRemotesToOwnersMap(remotes []*models.Remote) map[string]string {
	res := map[string]string{}
	for _, remote := range remotes {
		if len(remote.Urls) == 0 {
			continue
		}

		res[remote.Name] = GetRepoInfoFromURL(remote.Urls[0]).Owner
	}
	return res
}

type RepoInformation struct {
	Owner      string
	Repository string
}
package git_commands

import (
	"strings"
	"sync"

	"github.com/jesseduffield/lazygit/pkg/commands/oscommands"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/samber/lo"
	"github.com/sasha-s/go-deadlock"
)

type ExistingMainBranches struct {
	configuredMainBranches []string
	existingBranches       []string

	cmd   oscommands.ICmdObjBuilder
	mutex *deadlock.Mutex
}

func NewExistingMainBranches(
	configuredMainBranches []string,
	cmd oscommands.ICmdObjBuilder,
) *ExistingMainBranches {
	return &ExistingMainBranches{
		configuredMainBranches: configuredMainBranches,
		existingBranches:       nil,
		cmd:                    cmd,
		mutex:                  &deadlock.Mutex{},
	}
}

func (self *ExistingMainBranches) Get() []string {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	if self.existingBranches == nil {
		self.existingBranches = self.determineMainBranches()
	}

	return self.existingBranches
}

// Return the merge base of the given refName with the closest main branch.
func (self *ExistingMainBranches) GetMergeBase(refName string) string {
	mainBranches := self.Get()
	if len(mainBranches) == 0 {
		return ""
	}

	// We pass all existing main branches to the merge-base call; git will
	// return the base commit for the closest one.

	// We ignore errors from this call, since we can't distinguish whether the
	// error is because one of the main branches has been deleted since the last
	// call to determineMainBranches, or because the refName has no common
	// history with any of the main branches. Since the former should happen
	// very rarely, users must quit and restart lazygit to fix it; the latter is
	// also not very common, but can totally happen and is not an error.

	output, _ := self.cmd.New(
		NewGitCmd("merge-base").Arg(refName).Arg(mainBranches...).
			ToArgv(),
	).DontLog().RunWithOutput()
	return ignoringWarnings(output)
}

func (self *ExistingMainBranches) determineMainBranches() []string {
	var existingBranches []string
	var wg sync.WaitGroup

	existingBranches = make([]string, len(self.configuredMainBranches))

	for i, branchName := range self.configuredMainBranches {
		wg.Add(1)
		go utils.Safe(func() {
			defer wg.Done()

			// Try to determine upstream of local main branch
			if ref, err := self.cmd.New(
				NewGitCmd("rev-parse").Arg("--symbolic-full-name", branchName+"@{u}").ToArgv(),
			).DontLog().RunWithOutput(); err == nil {
				existingBranches[i] = strings.TrimSpace(ref)
				return
			}

			// If this failed, a local branch for this main branch doesn't exist or it
			// has no upstream configured. Try looking for one in the "origin" remote.
			ref := "refs/remotes/origin/" + branchName
			if err := self.cmd.New(
				NewGitCmd("rev-parse").Arg("--verify", "--quiet", ref).ToArgv(),
			).DontLog().Run(); err == nil {
				existingBranches[i] = ref
				return
			}

			// If this failed as well, try if we have the main branch as a local
			// branch. This covers the case where somebody is using git locally
			// for something, but never pushing anywhere.
			ref = "refs/heads/" + branchName
			if err := self.cmd.New(
				NewGitCmd("rev-parse").Arg("--verify", "--quiet", ref).ToArgv(),
			).DontLog().Run(); err == nil {
				existingBranches[i] = ref
			}
		})
	}

	wg.Wait()

	existingBranches = lo.Filter(existingBranches, func(branch string, _ int) bool {
		return branch != ""
	})

	return existingBranches
}

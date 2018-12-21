package dstask

// an interface to the filesystem/git based database -- loading, saving, committing

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"
)

const (
	GIT_REPO = "~/.dstask/"
	// space delimited keyword file for compgen
	COMPLETION_FILE = "~/.cache/dstask/completions"
	CONTEXT_FILE    = "~/.cache/dstask/context"
)

func MustGetRepoDirectory(directory ...string) string {
	root := MustExpandHome(GIT_REPO)
	return path.Join(append([]string{root}, directory...)...)
}

func LoadTaskSetFromDisk(statuses []string) *TaskSet {
	ts := &TaskSet{
		tasksByID:   make(map[int]*Task),
		tasksByUuid: make(map[string]*Task),
	}

	gitDotGitLocation := MustGetRepoDirectory(".git")

	if _, err := os.Stat(gitDotGitLocation); os.IsNotExist(err) {
		ExitFail("Could not find git repository at " + GIT_REPO + ", please clone or create. Try `dstask help` for more information.")
	}

	for _, status := range statuses {
		dir := MustGetRepoDirectory(status)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			err = os.Mkdir(dir, 0700)
			if err != nil {
				ExitFail("Failed to create directory in git repository")
			}
		}

		files, err := ioutil.ReadDir(dir)
		if err != nil {
			ExitFail("Failed to read " + dir)
		}

		for _, file := range files {
			filepath := path.Join(dir, file.Name())

			if len(file.Name()) != 40 {
				// not <uuid4>.yml
				continue
			}

			uuid := file.Name()[0:36]

			if !IsValidUuid4String(uuid) {
				continue
			}

			t := Task{
				Uuid:   uuid,
				Status: status,
			}

			data, err := ioutil.ReadFile(filepath)
			if err != nil {
				ExitFail("Failed to read %s", filepath)
			}
			err = yaml.Unmarshal(data, &t)
			if err != nil {
				// TODO present error to user, specific error message is important
				ExitFail("Failed to unmarshal %s", filepath)
			}

			ts.AddTask(t)
		}
	}

	return ts
}

func (t *Task) SaveToDisk() {
	if !t.WritePending {
		return
	}

	t.Modified = time.Now()

	filepath := MustGetRepoDirectory(t.Status, t.Uuid+".yml")
	d, err := yaml.Marshal(&t)
	if err != nil {
		// TODO present error to user, specific error message is important
		ExitFail("Failed to marshal task %s", t)
	}

	err = ioutil.WriteFile(filepath, d, 0600)
	if err != nil {
		ExitFail("Failed to write task %s", t)
	}

	// delete from all other locations to make sure there is only one copy
	// that exists
	for _, st := range ALL_STATUSES {
		if st == t.Status {
			continue
		}

		filepath := MustGetRepoDirectory(st, t.Uuid+".yml")

		if _, err := os.Stat(filepath); !os.IsNotExist(err) {
			err := os.Remove(filepath)
			if err != nil {
				ExitFail("Failed to delete " + filepath)
			}
		}
	}
}

// may be removed
func (ts *TaskSet) SaveToDisk(format string, a ...interface{}) {
	for _, task := range ts.tasks {
		task.SaveToDisk()
	}

	commitMsg := fmt.Sprintf(format, a...)

	// git add all changed/created files
	// could optimise this to be given an explicit list of
	// added/modified/deleted files -- only if slow.
	MustRunGitCmd("add", ".")
	MustRunGitCmd("commit", "--no-gpg-sign", "-m", commitMsg)
}

func SaveContext(args ...string) {
	fp := MustExpandHome(CONTEXT_FILE)
	os.MkdirAll(filepath.Dir(fp), os.ModePerm)
	context := ParseTaskLine(args...)
	MustWriteGob(fp, &context)
}

func LoadContext() TaskLine {
	fp := MustExpandHome(CONTEXT_FILE)
	if _, err := os.Stat(fp); os.IsNotExist(err) {
		return TaskLine{}
	}

	context := TaskLine{}
	MustReadGob(fp, &context)
	return context
}

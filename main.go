package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	MaxCommitLoad = 15
	MaxDiffLines  = 50
)

var (
	commitCache = map[string]string{}
	cacheMutex  sync.RWMutex
)

func executeCommand(name string, args ...string) string {

	cmd := exec.Command(name, args...)

	output, err := cmd.Output()
	if err != nil {
		return err.Error()
	}

	return string(output)
}

// DEFAULT RECENT COMMITS
func getRecentCommits() []string {

	output := executeCommand(
		"git",
		"log",
		fmt.Sprintf("-%d", MaxCommitLoad),
		"--pretty=format:%H|%s",
	)

	return splitLines(output)
}

// SEARCH ENTIRE HISTORY
func searchCommits(keyword string) []string {

	keyword = strings.TrimSpace(keyword)

	// EMPTY -> RECENT COMMITS
	if keyword == "" {
		return getRecentCommits()
	}

	output := executeCommand(
		"git",
		"log",
		"--all",
		"--pretty=format:%H|%s",
		"--grep="+keyword,
		fmt.Sprintf("-%d", MaxCommitLoad),
	)

	return splitLines(output)
}

func splitLines(content string) []string {

	if strings.TrimSpace(content) == "" {
		return []string{}
	}

	return strings.Split(
		strings.TrimSpace(content),
		"\n",
	)
}

func getCommitDetail(hash string) string {

	// CACHE HIT
	cacheMutex.RLock()
	cached, ok := commitCache[hash]
	cacheMutex.RUnlock()

	if ok {
		return cached
	}

	output := executeCommand(
		"git",
		"show",
		"--minimal",
		"--stat",
		"--patch",
		"--unified=10",
		hash,
	)

	lines := strings.Split(output, "\n")

	if len(lines) > MaxDiffLines {

		lines = lines[:MaxDiffLines]

		lines = append(
			lines,
			"",
			"... output truncated ...",
		)
	}

	result := strings.Join(lines, "\n")

	// SAVE CACHE
	cacheMutex.Lock()
	commitCache[hash] = result
	cacheMutex.Unlock()

	return result
}

func colorizeGitOutput(content string) string {

	lines := strings.Split(content, "\n")

	var result []string

	for _, line := range lines {

		switch {

		case strings.HasPrefix(line, "commit "):
			result = append(result, "[yellow]"+line)

		case strings.HasPrefix(line, "Author:"):
			result = append(result, "[aqua]"+line)

		case strings.HasPrefix(line, "Date:"):
			result = append(result, "[purple]"+line)

		case strings.HasPrefix(line, "diff --git"):
			result = append(result, "[blue]"+line)

		case strings.HasPrefix(line, "+++"),
			strings.HasPrefix(line, "---"):
			result = append(result, "[teal]"+line)

		case strings.HasPrefix(line, "+"):
			result = append(result, "[green]"+line)

		case strings.HasPrefix(line, "-"):
			result = append(result, "[red]"+line)

		case strings.HasPrefix(line, "@@"):
			result = append(result, "[yellow]"+line)

		case strings.Contains(line, "|"):
			result = append(result, "[white]"+line)

		default:
			result = append(result, "[gray]"+line)
		}
	}

	return strings.Join(result, "\n")
}

func main() {

	app := tview.NewApplication()

	// SEARCH INPUT
	searchInput := tview.NewInputField().
		SetLabel(" Search Commit: ")

	searchInput.SetBorder(true)

	// COMMIT LIST
	commitList := tview.NewList()

	commitList.SetBorder(true)
	commitList.SetTitle(" GitForest ")

	// DIFF VIEW
	diffView := tview.NewTextView()

	diffView.SetBorder(true)
	diffView.SetTitle(" Commit Preview ")
	diffView.SetScrollable(true)
	diffView.SetDynamicColors(true)

	// FOOTER
	footer := tview.NewTextView()

	footer.SetDynamicColors(true)
	footer.SetTextAlign(tview.AlignRight)

	footer.SetText(
		"[green]⚡ [white]gitforest [gray]by [yellow]aji mustofa",
	)

	// BUILD LIST
	buildCommitList := func(commits []string) {

		commitList.Clear()

		if len(commits) == 0 {

			commitList.AddItem(
				"[red]No commits found",
				"",
				0,
				nil,
			)

			return
		}

		for i := 0; i < len(commits); i++ {

			commit := commits[i]

			if strings.TrimSpace(commit) == "" {
				continue
			}

			parts := strings.SplitN(commit, "|", 2)

			if len(parts) < 2 {
				continue
			}

			hash := parts[0]
			message := parts[1]

			idx := strconv.Itoa(i + 1)

			title := fmt.Sprintf(
				"(%s) %s [gray][%s]",
				idx,
				message,
				hash[:7],
			)

			currentHash := hash

			commitList.AddItem(
				title,
				"",
				0,
				func() {

					diffView.SetText(
						"[yellow]Loading commit detail...",
					)

					go func(hash string) {

						content := getCommitDetail(hash)

						colored := colorizeGitOutput(content)

						app.QueueUpdateDraw(func() {

							diffView.SetText(colored)
						})

					}(currentHash)
				},
			)
		}
	}

	// INITIAL LOAD
	initialCommits := getRecentCommits()

	buildCommitList(initialCommits)

	// SEARCH DEBOUNCE
	var searchTimer *time.Timer

	searchInput.SetChangedFunc(func(text string) {

		if searchTimer != nil {
			searchTimer.Stop()
		}

		searchTimer = time.AfterFunc(
			300*time.Millisecond,
			func() {

				app.QueueUpdateDraw(func() {

					commitList.Clear()

					commitList.AddItem(
						"[yellow]Searching commits...",
						"",
						0,
						nil,
					)
				})

				go func(keyword string) {

					results := searchCommits(keyword)

					app.QueueUpdateDraw(func() {

						buildCommitList(results)
					})

				}(text)
			},
		)
	})

	// LEFT PANEL
	leftPanel := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(searchInput, 3, 0, true).
		AddItem(commitList, 0, 1, false)

	// RIGHT PANEL
	rightPanel := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(diffView, 0, 1, false).
		AddItem(footer, 1, 0, false)

	// MAIN LAYOUT
	layout := tview.NewFlex().
		AddItem(leftPanel, 0, 1, true).
		AddItem(rightPanel, 0, 2, false)

	// TAB SWITCHING
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {

		switch event.Key() {

		case tcell.KeyTAB:

			if app.GetFocus() == searchInput {

				app.SetFocus(commitList)

			} else if app.GetFocus() == commitList {

				app.SetFocus(diffView)

			} else {

				app.SetFocus(searchInput)
			}

			return nil
		}

		return event
	})

	// INITIAL TEXT
	diffView.SetText(
		"[green]Welcome to GitForest\n\n" +
			"[white]• Search entire git history\n" +
			"[white]• Press Enter to preview commit\n" +
			"[white]• Use TAB to switch focus\n\n" +
			"[gray]Optimized for large repositories.",
	)

	if err := app.SetRoot(layout, true).Run(); err != nil {
		panic(err)
	}
}

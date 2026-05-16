package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func executeCommand(name string, args ...string) string {
	cmd := exec.Command(name, args...)

	output, err := cmd.Output()
	if err != nil {
		return err.Error()
	}

	return string(output)
}

func getCommitList() []string {
	output := executeCommand(
		"git",
		"log",
		"--oneline",
	)

	return strings.Split(
		strings.TrimSpace(output),
		"\n",
	)
}

func getCommitFiles(hash string) []string {
	output := executeCommand(
		"git",
		"show",
		"--name-only",
		"--format=",
		hash,
	)

	return strings.Split(
		strings.TrimSpace(output),
		"\n",
	)
}

func getFileDiff(hash string, file string) string {
	return executeCommand(
		"git",
		"show",
		hash,
		"--",
		file,
	)
}

func colorizeDiff(diff string) string {

	lines := strings.Split(diff, "\n")

	var result []string

	for _, line := range lines {

		switch {

		case strings.HasPrefix(line, "+"):
			result = append(result, "[green]"+line)

		case strings.HasPrefix(line, "-"):
			result = append(result, "[red]"+line)

		case strings.HasPrefix(line, "@@"):
			result = append(result, "[yellow]"+line)

		case strings.HasPrefix(line, "diff --git"):
			result = append(result, "[blue]"+line)

		default:
			result = append(result, "[white]"+line)
		}
	}

	return strings.Join(result, "\n")
}

func main() {

	app := tview.NewApplication()

	allCommits := getCommitList()

	// SEARCH INPUT
	searchInput := tview.NewInputField().
		SetLabel("Search: ")

	searchInput.SetBorder(true)

	// LEFT TREE
	root := tview.NewTreeNode("[yellow]Commits")
	root.SetExpanded(true)

	tree := tview.NewTreeView().
		SetRoot(root).
		SetCurrentNode(root)

	tree.SetBorder(true)
	tree.SetTitle("Git History")

	// RIGHT DIFF PANEL
	diffView := tview.NewTextView()

	diffView.SetBorder(true)
	diffView.SetTitle("Diff Preview")
	diffView.SetScrollable(true)
	diffView.SetDynamicColors(true)

	// BUILD TREE FUNCTION
	buildTree := func(keyword string) {

		root.ClearChildren()

		for i := 0; i < len(allCommits); i++ {

			commit := allCommits[i]

			if strings.TrimSpace(commit) == "" {
				continue
			}

			// FILTER
			if keyword != "" &&
				!strings.Contains(
					strings.ToLower(commit),
					strings.ToLower(keyword),
				) {
				continue
			}

			parts := strings.Split(commit, " ")

			hash := parts[0]

			idx := strconv.Itoa(i + 1)

			coloredCommit := fmt.Sprintf(
				"[yellow](%s)[white] %s",
				idx,
				commit,
			)

			commitNode := tview.NewTreeNode(coloredCommit)

			commitNode.SetExpanded(false)

			files := getCommitFiles(hash)

			for _, file := range files {

				if strings.TrimSpace(file) == "" {
					continue
				}

				pathParts := strings.Split(file, "/")

				currentNode := commitNode

				fullPath := ""

				for _, part := range pathParts {

					if fullPath == "" {
						fullPath = part
					} else {
						fullPath += "/" + part
					}

					var foundNode *tview.TreeNode

					for _, child := range currentNode.GetChildren() {

						cleanText := strings.ReplaceAll(
							child.GetText(),
							"[blue]",
							"",
						)

						cleanText = strings.ReplaceAll(
							cleanText,
							"[cyan]",
							"",
						)

						if cleanText == part {
							foundNode = child
							break
						}
					}

					if foundNode == nil {

						color := "[cyan]"

						if !strings.Contains(part, ".") {
							color = "[blue]"
						}

						foundNode = tview.NewTreeNode(
							color + part,
						)

						foundNode.SetExpanded(true)

						foundNode.SetReference(map[string]string{
							"hash": hash,
							"file": fullPath,
						})

						currentNode.AddChild(foundNode)
					}

					currentNode = foundNode
				}
			}

			root.AddChild(commitNode)
		}
	}

	buildTree("")

	// SEARCH LIVE
	searchInput.SetChangedFunc(func(text string) {
		buildTree(text)
	})

	// SELECT FILE
	tree.SetSelectedFunc(func(node *tview.TreeNode) {

		ref := node.GetReference()

		if ref == nil {
			node.SetExpanded(!node.IsExpanded())
			return
		}

		data, ok := ref.(map[string]string)

		if !ok {
			return
		}

		hash := data["hash"]
		file := data["file"]

		diff := getFileDiff(hash, file)

		diffView.SetText(
			colorizeDiff(diff),
		)

		node.SetExpanded(!node.IsExpanded())
	})

	// TOP + CONTENT
	leftPanel := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(searchInput, 3, 0, true).
		AddItem(tree, 0, 1, false)

	// MAIN LAYOUT
	layout := tview.NewFlex().
		AddItem(leftPanel, 0, 1, true).
		AddItem(diffView, 0, 2, false)

	// TAB SWITCHING
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {

		switch event.Key() {

		case tcell.KeyTAB:

			if app.GetFocus() == searchInput {
				app.SetFocus(tree)
			} else if app.GetFocus() == tree {
				app.SetFocus(diffView)
			} else {
				app.SetFocus(searchInput)
			}

			return nil
		}

		return event
	})

	if err := app.SetRoot(layout, true).Run(); err != nil {
		panic(err)
	}
}

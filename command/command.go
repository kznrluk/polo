package command

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/kznrluk/aski/config"
	"github.com/kznrluk/aski/conv"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type cmd struct {
	name        string
	description string
}

var availableCommands = []cmd{
	{
		name:        ":history",
		description: "Show conversation history.",
	},
	{
		name:        ":summary",
		description: "Show conversation summary.",
	},
	{
		name:        ":move",
		description: "Change HEAD to another message.",
	},
	{
		name:        ":config",
		description: "Open configuration directory.",
	},
	{
		name:        ":editor",
		description: "Open an embedded text editor.",
	},
	{
		name:        ":exit",
		description: "Exit the program.",
	},
}

func matchCommand(input string) (string, bool) {
	matched := false
	var matchedCmd string

	for _, cmd := range availableCommands {
		if strings.HasPrefix(cmd.name, input) {
			if matched {
				return "", false // ambiguous input, more than one command matches
			}
			matched = true
			matchedCmd = cmd.name
		}
	}

	return matchedCmd, matched
}

func unknownCommand() string {
	output := "unknown command.\n\n"
	for _, cmd := range availableCommands {
		output += fmt.Sprintf("  %-8s - %s\n", cmd.name, cmd.description)
	}

	return output
}

func Parse(input string, conv conv.Conversation) (string, bool, error) {
	trimmedInput := strings.TrimSpace(input)
	commands := strings.Split(trimmedInput, " ")

	matchedCmd, found := matchCommand(commands[0])
	if !found {
		return "", false, fmt.Errorf(unknownCommand())
	}
	commands[0] = matchedCmd

	if commands[0] == ":history" {
		showContext(conv)
		return "", false, nil
	} else if commands[0] == ":summary" {
		showSummary(conv)
		return "", false, nil
	} else if commands[0] == ":move" {
		err := changeHead(commands[1], conv)
		return "", false, err
	} else if commands[0] == ":config" {
		_ = config.OpenConfigDir()
		return "", false, nil
	} else if commands[0] == ":editor" {
		return openEditor(conv)
	}

	return "", false, fmt.Errorf("unknown command")
}

func changeHead(sha1Partial string, context conv.Conversation) error {
	if sha1Partial == "" {
		return fmt.Errorf("No SHA1 partial provided")
	}
	msg, err := context.ChangeHead(sha1Partial)
	if err != nil {
		return err
	}

	yellow := color.New(color.FgHiYellow).SprintFunc()
	blue := color.New(color.FgHiBlue).SprintFunc()
	fmt.Printf("%s %s\n", yellow(fmt.Sprintf("%.*s [%s]", 6, msg.Sha1, msg.Role)), blue("Head"))
	for _, context := range strings.Split(msg.Content, "\n") {
		fmt.Printf("  %s\n", context)
	}

	return nil
}

func showContext(conv conv.Conversation) {
	yellow := color.New(color.FgHiYellow).SprintFunc()
	blue := color.New(color.FgHiBlue).SprintFunc()
	for _, msg := range conv.GetMessages() {
		head := ""
		if msg.Head {
			head = "Head"
		}
		fmt.Printf("%s %s\n", yellow(fmt.Sprintf("%.*s -> %.*s [%s]", 6, msg.Sha1, 6, msg.ParentSha1, msg.Role)), blue(head))

		for _, context := range strings.Split(msg.Content, "\n") {
			fmt.Printf("  %s\n", context)
		}

		fmt.Printf("\n")
	}
}

func showSummary(conv conv.Conversation) {
	blue := color.New(color.FgHiBlue).SprintFunc()
	fmt.Printf(blue(conv.GetSummary()))
}

func openEditor(conv conv.Conversation) (string, bool, error) {
	tmpFile, err := os.CreateTemp("", "aski-editor-")
	if err != nil {
		return "", false, fmt.Errorf("failed to create a temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	comments := "\n\n"
	s := conv.MessagesFromHead()
	for i := len(s) - 1; i >= 0; i-- {
		msg := s[i]
		head := ""
		if msg.Head {
			head = "Head"
		}

		d := fmt.Sprintf("#\n# %.*s -> %.*s [%s] %s\n", 6, msg.Sha1, 6, msg.ParentSha1, msg.Role, head)
		for _, context := range strings.Split(msg.Content, "\n") {
			d += fmt.Sprintf("#   %s\n", context)
		}
		comments += d
	}

	_, err = tmpFile.WriteString(comments)
	if err != nil {
		return "", false, fmt.Errorf("failed to write to the temp file: %v", err)
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		if runtime.GOOS == "windows" {
			editor = "notepad.exe"
		} else {
			editor = "vim"
		}
	}

	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// for vscode :)
	if strings.Contains(editor, "code") {
		cmd = exec.Command(editor, "--wait", tmpFile.Name())
	}

	err = cmd.Run()
	if err != nil {
		return "", false, fmt.Errorf("failed to open editor: %v", err)
	}

	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", false, fmt.Errorf("failed to read the edited content: %v", err)
	}

	result := ""
	for _, d := range strings.Split(string(content), "\n") {
		if !strings.HasPrefix(d, "#") {
			result += d + "\n"
		}
	}

	if len(strings.TrimSpace(result)) == 0 {
		return "", false, nil
	}

	return result, true, nil
}

// Package message provides utilities for parsing, executing, and replying to Discord messages.
package message

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"syscall"
	"unicode/utf8"

	"github.com/disgoorg/disgo/discord"
	"github.com/norio-nomura/cli_discord_bot2/pkg/options"
	"github.com/norio-nomura/cli_discord_bot2/pkg/shellwords"
)

// ExecutionResult represents the result of executing a command, including content and any files to send.
type ExecutionResult struct {
	Content string
	Files   []*discord.File
}

// executeTarget executes a command with the given options and input, then returns the execution result.
// It runs the command in a temporary directory, captures output, and returns both content and files.
func executeTarget(
	ctx context.Context,
	o *options.Options,
	commandline string,
	input io.Reader,
	outputCommandline bool,
) (*ExecutionResult, error) {
	// Create a temporary directory for execution
	cwd, err := os.MkdirTemp("", "execute_target")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(cwd); err != nil {
			slog.Error("executeTarget", slog.String("error", fmt.Sprintf("failed to remove temp directory %s: %v", cwd, err)))
		}
	}()

	contentMax := 2000
	content := ""

	cli := []string{o.TargetCLI}

	// Parse the commandline into executable and arguments using shellwords
	args, err := shellwords.Split(commandline)
	if err != nil {
		return nil, fmt.Errorf("failed to parse commandline \"%s\" with error: %w", commandline, err)
	}
	if len(args) == 0 {
		args = o.TargetDefaultArgs
	}
	cli = append(cli, args...)
	if input != nil {
		cli = append(cli, o.TargetArgsToUseStdin...)
	}
	args = slices.Concat(o.EnvCommand, cli)

	if outputCommandline {
		content += fmt.Sprintf("`%s`\n", shellwords.Join(cli))
	}

	// Create a new context with a timeout for the command execution.
	ctx, cancel := o.ContextWithTimeout(ctx)
	defer cancel()

	// Prepare the command
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = cwd
	cmd.Stdin = input
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// Ensure the command runs in a new process group to allow for proper cancellation.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		// If the command is running, send a SIGINT to the process group.
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
	}
	// Waits for the command to finish before force killing it.
	// cmd.WaitDelay = 5 * time.Second

	// Run the command
	if err = cmd.Run(); err != nil {
		var errString string
		switch ctx.Err() {
		case context.Canceled:
			errString = err.Error()
		case context.DeadlineExceeded:
			errString = context.Cause(ctx).Error()
		default:
			errString = err.Error()
		}
		slog.Error("executeTarget", slog.String("args", shellwords.Join(args)), slog.String("error", errString))
		content += fmt.Sprintf("%s with ", errString)
	} else {
		slog.Info("executeTarget", slog.String("args", shellwords.Join(args)))
	}

	// Process outputs
	files := []*discord.File{}
	type output struct {
		Name   string
		Output *bytes.Buffer
	}
	outputs := []output{}
	if stdout.Len() > 0 {
		outputs = append(outputs, output{"stdout", &stdout})
	}
	if stderr.Len() > 0 {
		outputs = append(outputs, output{"stderr", &stderr})
	}
	if len(outputs) == 0 {
		content += "no output"
	} else {
		maxLinesToEmbed := o.NumberOfLinesToEmbedOutput
		previewLinesForUploaded := o.NumberOfLinesToEmbedUploadedOutput
		for i, out := range outputs {
			header := "```\n"
			if i == 0 && err != nil {
				header = out.Name + ":" + header
			}
			footer := "```"
			limit := contentMax - utf8.RuneCountInString(content) - utf8.RuneCountInString(header) - utf8.RuneCountInString(footer)
			embed, reader := bytesToEmbedAndReader(out.Output.Bytes(), maxLinesToEmbed, limit, previewLinesForUploaded)
			content += header + embed + footer
			if reader != nil {
				files = append(files, &discord.File{
					Name:   out.Name + ".log",
					Reader: reader,
				})
			}
		}
	}

	// Collect additional files from the temp directory
	entries, err := os.ReadDir(cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			path := filepath.Join(cwd, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("failed to read file %s: %w", path, err)
			}
			files = append(files, &discord.File{
				Name:   entry.Name(),
				Reader: bytes.NewReader(data),
			})
		}
	}

	return &ExecutionResult{
		Content: content,
		Files:   files,
	}, nil
}

// bytesToEmbedAndReader splits the byte slice into a string for embedding and a reader for uploading as a file.
// It limits the number of lines and runes in the embed, and provides a preview if the output is too large.
func bytesToEmbedAndReader(b []byte, maxLines, maxRunes, previewLines int) (string, *bytes.Reader) {
	lineNumber := 0
	runeCount := 0
	embedEnd := 0

	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		i += size
		if r == '\n' {
			lineNumber++
			if lineNumber == previewLines {
				embedEnd = i
			}
			if lineNumber > maxLines {
				return string(b[:embedEnd]), bytes.NewReader(b)
			}
		}
		runeCount++
		if runeCount == maxRunes {
			return string(b[:i]), bytes.NewReader(b)
		}
	}
	return string(b), nil
}

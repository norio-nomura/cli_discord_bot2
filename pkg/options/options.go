// Package options provides configuration structures and utilities for the Discord bot.
package options

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/norio-nomura/cli_discord_bot2/pkg/shellwords"
)

// Options holds configuration values for the Discord bot, loaded from environment variables or JSON.
type Options struct {
	AttachmentExtensionToTreatAsInput  string   `env:"ATTACHMENT_EXTENSION_TO_TREAT_AS_INPUT" json:","`
	DiscordNickname                    string   `env:"DISCORD_NICKNAME" json:",omitempty"`
	DiscordPlaying                     string   `env:"DISCORD_PLAYING" json:",omitempty"`
	DiscordToken                       string   `env:"DISCORD_TOKEN" json:","`
	EnvCommand                         []string `env:"ENV_COMMAND" json:","`
	NumberOfLinesToEmbedOutput         int      `env:"NUMBER_OF_LINES_TO_EMBED_OUTPUT" json:","`
	NumberOfLinesToEmbedUploadedOutput int      `env:"NUMBER_OF_LINES_TO_EMBED_UPLOADED_OUTPUT" json:","`
	RestTimeoutSeconds                 int      `env:"REST_TIMEOUT_SECONDS" json:","`
	TargetArgsToUseStdin               []string `env:"TARGET_ARGS_TO_USE_STDIN" json:","`
	TargetCLI                          string   `env:"TARGET_CLI" json:","`
	TargetDefaultArgs                  []string `env:"TARGET_DEFAULT_ARGS" json:","`
	TimeoutSeconds                     int      `env:"TIMEOUT_SECONDS" json:","`
}

// defaultOptions creates a new Options instance with default values.
func defaultOptions() *Options {
	return &Options{
		EnvCommand:                         []string{"/usr/bin/env", "-i"},
		NumberOfLinesToEmbedOutput:         20,
		NumberOfLinesToEmbedUploadedOutput: 3,
		RestTimeoutSeconds:                 10,
		TargetCLI:                          "cat",
		TimeoutSeconds:                     30,
	}
}

// FromEnv populates Options from environment variables dynamically.
// Returns an Options pointer or an error if required fields are missing or invalid.
func FromEnv() (*Options, error) {
	options := defaultOptions()
	v := reflect.ValueOf(options).Elem()
	t := v.Type()

	for i := range v.NumField() {
		field := v.Field(i)
		fieldType := t.Field(i)

		envKey := fieldType.Tag.Get("env")
		if envKey == "" {
			continue
		}

		envValue, exists := os.LookupEnv(envKey)
		if !exists {
			continue
		}

		// Set the field value based on its type
		switch field.Kind() {
		case reflect.Slice:
			if field.Type().Elem().Kind() == reflect.String {
				// Split the string by spaces to create a slice of strings
				sliceValue, err := shellwords.Split(envValue)
				if err != nil {
					return nil, fmt.Errorf("failed to parse %s: %w", envKey, err)
				}
				field.Set(reflect.ValueOf(sliceValue))
			} else {
				return nil, fmt.Errorf("unsupported slice type for %s", envKey)
			}
		case reflect.String:
			field.SetString(envValue)
		case reflect.Int:
			intValue, err := strconv.Atoi(envValue)
			if err != nil {
				return nil, fmt.Errorf("invalid value for %s: %w", envKey, err)
			}
			field.SetInt(int64(intValue))
		}

		// Remove the environment variable after reading it
		if err := os.Unsetenv(envKey); err != nil {
			return nil, fmt.Errorf("failed to unset environment variable %s: %w", envKey, err)
		}
	}

	// Ensure required fields are set
	if options.DiscordToken == "" {
		return nil, errors.New("`DISCORD_TOKEN` is missing in environment variables")
	}

	// pass PATH="..." to EnvCommand if not set
	if !slices.ContainsFunc(options.EnvCommand, func(s string) bool { return strings.HasPrefix(s, "PATH=") }) {
		options.EnvCommand = append(options.EnvCommand, "PATH="+os.Getenv("PATH"))
	}

	return options, nil
}

// FromStdin reads JSON from standard input and populates Options.
// Returns an Options pointer or an error if required fields are missing or invalid.
func FromStdin() (*Options, error) {
	options := defaultOptions()
	decoder := json.NewDecoder(os.Stdin)
	if err := decoder.Decode(options); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}

	// Ensure required fields are set
	if options.DiscordToken == "" {
		return nil, errors.New("`DISCORD_TOKEN` is missing in JSON")
	}
	return options, nil
}

// Discord returns the Discord nickname and playing status from the options.
// If not set, it falls back to the TargetCLI value.
func (o *Options) Discord() (nickname, playing string) {
	if o.DiscordNickname != "" {
		nickname = o.DiscordNickname
	} else {
		nickname = o.TargetCLI
	}
	if o.DiscordPlaying != "" {
		playing = o.DiscordPlaying
	} else {
		playing = o.TargetCLI
	}
	return
}

// ExecWithPassingOptionsToStdin serializes the Options to JSON, sets up a pipe, and replaces the current process.
// This method can be used to re-execute the current process with options passed via stdin.
func (o *Options) ExecWithPassingOptionsToStdin() error {
	// Serialize options to JSON
	jsonData, err := json.Marshal(o)
	if err != nil {
		return fmt.Errorf("failed to serialize options to JSON: %w", err)
	}

	// Prepare arguments for execve
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	cmd := os.Args[0]
	args := slices.Insert(os.Args[1:], 0, "--stdin")
	env := os.Environ()

	// Create a pipe for stdin
	r, w, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}
	// Redirect the pipe's read end to standard input
	if err = dup2(int(r.Fd()), int(os.Stdin.Fd())); err != nil {
		return fmt.Errorf("failed to redirect stdin: %w", err)
	}
	// Write JSON to the pipe
	if _, err = w.Write(jsonData); err != nil {
		return fmt.Errorf("failed to write to pipe: %w", err)
	}
	// Close the write end of the pipe
	if err = w.Close(); err != nil {
		return fmt.Errorf("failed to close pipe: %w", err)
	}

	// Replace the current process
	err = syscall.Exec(executable, append([]string{cmd}, args...), env)
	// If Exec returns, it means there was an error
	return fmt.Errorf("failed to exec process: %w", err)
}

// ContextWithRestTimeout creates a context with the REST timeout duration.
// This context can be used to enforce a timeout for REST API calls.
func (o *Options) ContextWithRestTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := o.RestTimeoutSeconds
	if timeout <= 0 {
		timeout = defaultOptions().RestTimeoutSeconds
	}
	return context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
}

// ContextWithTimeout creates a context with the timeout duration.
// This context can be used to enforce a timeout for operations that may take too long.
func (o *Options) ContextWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := o.TimeoutSeconds
	if timeout <= 0 {
		timeout = defaultOptions().TimeoutSeconds
	}
	return context.WithTimeoutCause(ctx, time.Duration(timeout)*time.Second, fmt.Errorf("process killed due to timeout of %d seconds", timeout))
}

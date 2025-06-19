// Package message provides utilities for parsing, executing, and replying to Discord messages.
package message

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
	"github.com/norio-nomura/cli_discord_bot2/pkg/future"
	"github.com/norio-nomura/cli_discord_bot2/pkg/options"
	"github.com/norio-nomura/cli_discord_bot2/pkg/xiter"
)

// --- Public API ---

// ChannelType returns the channel type for the given message.
func ChannelType(ctx context.Context, e *events.GenericMessage) (discord.ChannelType, error) {
	ch, err := e.Client().Rest().GetChannel(e.ChannelID, rest.WithCtx(ctx))
	if err != nil {
		var zero discord.ChannelType
		return zero, fmt.Errorf("failed to get channel type: %w", err)
	}
	return ch.Type(), nil
}

// ExecuteCmds executes commands found in a message that mentions the bot.
// It returns a sequence of Futures, each representing the asynchronous execution result of a command.
func ExecuteCmds(ctx context.Context, o *options.Options, e *events.GenericMessage) iter.Seq[future.Future[*ExecutionResult]] {
	// Ensure the context has a timeout for rest operations.
	restCtx, cancel := o.ContextWithRestTimeout(ctx)
	defer cancel()

	// If the message should be ignored, return an empty sequence.
	emptySeq := xiter.SeqOf[future.Future[*ExecutionResult]]()
	if ShouldIgnore(e) {
		return emptySeq
	}
	// Determine the channel type and set default commands based on it.
	channelType, err := ChannelType(restCtx, e)
	if err != nil {
		return xiter.SeqOf(future.NewError[*ExecutionResult](err))
	}
	defaultCmds := make([]string, 0)
	switch channelType {
	case discord.ChannelTypeGuildText, discord.ChannelTypeGuildPublicThread, discord.ChannelTypeGuildPrivateThread:
		if !mentioning(e, e.Client().ID()) {
			return emptySeq
		}
	case discord.ChannelTypeDM:
		defaultCmds = append(defaultCmds, "")
	default:
		return emptySeq
	}
	// detect input from attachments or code blocks
	input, err := inputFromAttachment(restCtx, e, o.AttachmentExtensionToTreatAsInput)
	if err != nil {
		return xiter.SeqOf(future.NewError[*ExecutionResult](err))
	} else if input == nil {
		input = inputFromCodeblock(e)
	}
	// detect command lines from mentions in the message content
	cmds := commandlinesFromMentions(e)
	if len(cmds) == 0 {
		cmds = defaultCmds
	}
	// If multiple commands are provided, we will output the command being executed.
	outputCmd := len(cmds) > 1

	// If commands are provided, we will send a typing indicator to the channel.
	if len(cmds) > 0 {
		_ = e.Client().Rest().SendTyping(e.ChannelID, rest.WithCtx(restCtx))
	}
	// Prepare the commands for execution, deduplicating them.
	seqCmds := xiter.Dedupe(slices.Values(cmds))
	executeCmdFunc := func(cmd string) future.Future[*ExecutionResult] {
		var reader io.Reader
		if input != nil {
			reader = bytes.NewReader(input)
		} else if strings.TrimSpace(cmd) == "" {
			// If the command is empty and no input is provided, return a help message.
			return future.NewDeferred(func(_ context.Context) (*ExecutionResult, error) {
				return helpResult(e)
			})
		}
		return future.New(ctx, func(ctx context.Context) (*ExecutionResult, error) {
			return executeTarget(ctx, o, cmd, reader, outputCmd)
		})
	}
	return xiter.Map(seqCmds, executeCmdFunc)
}

// GetReplies returns a future for all bot replies to a given message.
func GetReplies(o *options.Options, e *events.GenericMessage) future.Future[iter.Seq[discord.Message]] {
	botID := e.Client().ID()
	return getMessagesWithFilter(o, e, e.ChannelID, func(m discord.Message) bool {
		return m.Author.ID == botID && m.Type == discord.MessageTypeReply && m.MessageReference != nil && *m.MessageReference.MessageID == e.MessageID
	})
}

// GetRepliesInThread returns a future for all bot replies in a thread to a given message.
func GetRepliesInThread(o *options.Options, e *events.GenericMessage) future.Future[iter.Seq[discord.Message]] {
	botID := e.Client().ID()
	return getMessagesWithFilter(o, e, e.MessageID, func(m discord.Message) bool {
		return m.Author.ID == botID && m.Type == discord.MessageTypeDefault
	})
}

func getMessagesWithFilter(o *options.Options, e *events.GenericMessage, channelID snowflake.ID, filterFunc func(discord.Message) bool) future.Future[iter.Seq[discord.Message]] {
	return future.NewDeferred(func(ctx context.Context) (iter.Seq[discord.Message], error) {
		// Ensure the context has a timeout for rest operations.
		ctx, cancel := o.ContextWithRestTimeout(ctx)
		defer cancel()
		messages, err := e.Client().Rest().GetMessages(channelID, 0, 0, e.MessageID, 0, rest.WithCtx(ctx))
		if err != nil {
			return nil, fmt.Errorf("failed to get messages in channel %s: %w", channelID, err)
		}
		replies := xiter.Filter(slices.Values(messages), filterFunc)
		cmpMessage := func(m1, m2 discord.Message) int {
			return cmp.Compare(m1.ID, m2.ID)
		}
		return slices.Values(slices.SortedFunc(replies, cmpMessage)), nil
	})
}

// SendReply sends a reply to the given message with the provided execution result.
// Returns a future for the sent Discord message.
func SendReply(o *options.Options, e *events.GenericMessage, r *ExecutionResult) future.Future[*discord.Message] {
	if r == nil {
		return future.NewValue[*discord.Message](nil)
	}
	var reply discord.MessageCreate
	var channelID snowflake.ID
	if e.Message.Flags.Has(discord.MessageFlagHasThread) {
		channelID = e.MessageID
		reply = discord.NewMessageCreateBuilder().SetContent(r.Content).SetFiles(r.Files...).Build()
	} else {
		channelID = e.ChannelID
		reply = discord.NewMessageCreateBuilder().SetContent(r.Content).SetFiles(r.Files...).SetMessageReferenceByID(e.MessageID).Build()
	}
	return future.NewDeferred(func(ctx context.Context) (*discord.Message, error) {
		// Ensure the context has a timeout for rest operations.
		ctx, cancel := o.ContextWithRestTimeout(ctx)
		defer cancel()
		return e.Client().Rest().CreateMessage(channelID, reply, rest.WithCtx(ctx))
	})
}

// ShouldIgnore returns true if the message should be ignored (e.g., from a bot or unsupported type).
func ShouldIgnore(e *events.GenericMessage) bool {
	switch e.Message.Type {
	case discord.MessageTypeDefault, discord.MessageTypeReply:
		return e.Message.Author.Bot
	default:
		return true
	}
}

// UpdateMessage updates the given Discord message with the new execution result.
// Returns a future for the updated message.
func UpdateMessage(o *options.Options, e *events.GenericMessage, m discord.Message, r *ExecutionResult) future.Future[*discord.Message] {
	if r == nil {
		return future.NewValue[*discord.Message](nil)
	}
	msg := discord.NewMessageUpdateBuilder().SetContent(r.Content).SetFiles(r.Files...).RetainAttachments().Build()
	return future.NewDeferred(func(ctx context.Context) (*discord.Message, error) {
		// Ensure the context has a timeout for rest operations.
		ctx, cancel := o.ContextWithRestTimeout(ctx)
		defer cancel()
		return e.Client().Rest().UpdateMessage(m.ChannelID, m.ID, msg, rest.WithCtx(ctx))
	})
}

// DeleteMessage deletes the specified message in the given channel.
// Returns a future that resolves when the deletion is complete.
func DeleteMessage(o *options.Options, e *events.GenericMessage, id snowflake.ID) future.Future[any] {
	return future.NewDeferred(func(ctx context.Context) (any, error) {
		// Ensure the context has a timeout for rest operations.
		ctx, cancel := o.ContextWithRestTimeout(ctx)
		defer cancel()
		if err := e.Client().Rest().DeleteMessage(e.ChannelID, id, rest.WithCtx(ctx)); err != nil {
			return nil, fmt.Errorf("failed to delete message %s: %w", id, err)
		}
		return nil, nil
	})
}

// --- Private helpers ---

// commandlinesFromMentions extracts command lines from mention lines in the message content.
//
//	e: Discord message event
//
// Returns a slice of command line strings, or nil if none found.
func commandlinesFromMentions(e *events.GenericMessage) []string {
	if e.Message.Content == "" {
		return nil
	}
	mentionLinePattern := regexp.MustCompile("(?ms)<@!?" + e.Client().ID().String() + ">(.*?)(?:```|$)")
	matches := mentionLinePattern.FindAllStringSubmatch(e.Message.Content, -1)
	if len(matches) == 0 {
		return nil
	}
	lines := make([]string, 0, len(matches))
	mentionPattern := regexp.MustCompile(`<@!?\d+>`)
	for _, match := range matches {
		lines = append(lines, mentionPattern.ReplaceAllString(match[1], ""))
	}
	return lines
}

// helpResult returns a default help message for the bot, formatted with code blocks.
// It includes usage instructions and an example of how to provide input.
func helpResult(e *events.GenericMessage) (*ExecutionResult, error) {
	const tripleBackticks = "```"
	const zeroWithSpace = "\u200b"
	// To embed triple backticks in a code block, we need to use zero-width spaces
	const tripleBackticksForCodeblock = "`" + zeroWithSpace + "`" + zeroWithSpace + "`"
	user, _ := e.Client().Caches().SelfUser()
	return &ExecutionResult{
		Content: tripleBackticks + `
Usage:
@` + user.Username + `
` + tripleBackticksForCodeblock + `
[contents for standard input]
` + tripleBackticksForCodeblock + `
` + tripleBackticks,
	}, nil
}

// inputFromAttachment returns an io.Reader for the first attachment in the message
// whose filename matches the given extension. Returns (nil, nil) if not found.
//
// ctx: context for the request
//
//	e: Discord message event
//	extension: file extension to match (e.g. ".txt")
func inputFromAttachment(ctx context.Context, e *events.GenericMessage, extension string) ([]byte, error) {
	hasTargetExtension := func(a discord.Attachment) bool {
		return strings.HasSuffix(a.Filename, extension)
	}
	next, stop := iter.Pull(xiter.Filter(slices.Values(e.Message.Attachments), hasTargetExtension))
	defer stop()
	attachment, ok := next()
	if !ok {
		return nil, nil // No matching attachment found
	}
	req, err := http.NewRequestWithContext(ctx, "GET", attachment.URL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for attachment %s: %w", attachment.Filename, err)
	}
	resp, err := e.Client().Rest().HTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download attachment %s: %w", attachment.Filename, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("Failed to close response body", slog.String("attachment", attachment.Filename), slog.Any("error", err))
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download attachment %s: %s", attachment.Filename, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read attachment %s: %w", attachment.Filename, err)
	}
	return body, nil
}

// inputFromCodeblock extracts the first code block from the message content.
//
//	e: Discord message event
//
// Returns an io.Reader for the code block, or nil if no code block is found.
func inputFromCodeblock(e *events.GenericMessage) []byte {
	if e.Message.Content == "" {
		return nil
	}
	re := regexp.MustCompile("(?ms)```(?:.*?\\n)?(.*?)```")
	matches := re.FindStringSubmatch(e.Message.Content)
	if len(matches) == 0 {
		return nil
	}
	return []byte(matches[1])
}

// mentioning returns true if the specified user ID is mentioned in the message.
//
//	e: Discord message event
//	userID: user to check for mention
func mentioning(e *events.GenericMessage, userID snowflake.ID) bool {
	return slices.ContainsFunc(e.Message.Mentions, func(m discord.User) bool { return m.ID == userID })
}

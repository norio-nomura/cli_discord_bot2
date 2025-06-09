// Package client provides Discord client setup and event handling utilities.
package client

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
	"github.com/norio-nomura/cli_discord_bot2/pkg/future"
	"github.com/norio-nomura/cli_discord_bot2/pkg/message"
	"github.com/norio-nomura/cli_discord_bot2/pkg/options"
	"github.com/norio-nomura/cli_discord_bot2/pkg/xiter"
)

// messageEventsHandler handles Discord message events and manages event processing for each message ID.
// It stores the latest event for each message and processes them in a thread-safe manner.
type messageEventsHandler struct {
	options *options.Options
	syncMap sync.Map
}

// onMessageCreate handles the MessageCreate event and stores it for processing.
func (q *messageEventsHandler) onMessageCreate(e *events.MessageCreate) {
	if message.ShouldIgnore(e.GenericMessage) {
		return
	}
	q.storeLatestEventForMessageID(e.MessageID, e)
}

// onMessageUpdate handles the MessageUpdate event and stores it for processing.
func (q *messageEventsHandler) onMessageUpdate(e *events.MessageUpdate) {
	if message.ShouldIgnore(e.GenericMessage) {
		return
	}
	q.storeLatestEventForMessageID(e.MessageID, e)
}

// onMessageDelete handles the MessageDelete event and stores it for processing.
func (q *messageEventsHandler) onMessageDelete(e *events.MessageDelete) {
	if e.Message.ID != 0 && message.ShouldIgnore(e.GenericMessage) {
		return
	}
	q.storeLatestEventForMessageID(e.MessageID, e)
}

// storeLatestEventForMessageID stores the latest event for a given message ID in the sync map.
// If the event is newly stored, it starts a goroutine to process events for that message ID.
func (q *messageEventsHandler) storeLatestEventForMessageID(id snowflake.ID, e any) {
	ch := make(chan any, 1)
	ch <- e // Store the event in the channel.
	if old, stored := storeToSyncMap(&q.syncMap, id, ch); stored {
		// If the value was newly stored, start a goroutine to process the event for the message ID.
		go q.processEventsForMessageID(id)
	} else {
		// If the value was updated, close the old channel to signal that it is no longer needed.
		close(old)
	}
}

// storeToSyncMap stores a value in a sync.Map for the given key.
// Returns true if the value was newly stored, or false if it updated an existing value.
func storeToSyncMap[K, V any](m *sync.Map, k K, v V) (old V, stored bool) {
	if oldAny, loaded := m.LoadOrStore(k, v); loaded {
		swapped := m.CompareAndSwap(k, oldAny, v)
		if swapped {
			// value was updated.
			old, _ := oldAny.(V)
			return old, false
		}
		// Try again from the beginning.
		return storeToSyncMap(m, k, v)
	}
	// value was not loaded, so it was newly stored.
	var zero V
	return zero, true
}

// loadFromSyncMap retrieves a value from a sync.Map for the given key.
// Returns the value and an error if the key was not found or if the value is of an unexpected type.
func loadFromSyncMap[K, V any](m *sync.Map, k K) (V, error) {
	latest, ok := m.Load(k)
	if !ok {
		var zero V
		return zero, fmt.Errorf("key %v not found in sync map", k)
	}
	v, ok := latest.(V)
	if !ok {
		var zero V
		return zero, fmt.Errorf("loaded value is not of expected type: %T", latest)
	}
	return v, nil
}

// contextFromChannel creates a context that is cancelled when the provided channel is closed.
func contextFromChannel[T any](ch chan T) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		// Ensure the context is cancelled when the channel is closed.
		<-ch
		cancel()
	}()
	return ctx
}

// processEventsForMessageID processes all events for a given message ID in order.
// It handles command execution and reply management for the message, updating or deleting as needed.
func (q *messageEventsHandler) processEventsForMessageID(id snowflake.ID) {
	for {
		ch, err := loadFromSyncMap[snowflake.ID, chan any](&q.syncMap, id)
		if err != nil {
			slog.Error("Failed to load channel from sync map", slog.Any("id", id), slog.Any("err", err))
			return
		}
		e, ok := <-ch
		if !ok {
			slog.Error("Failed to receive event for message ID", slog.Any("id", id))
			return
		}
		ctx := contextFromChannel(ch)
		var gm *events.GenericMessage
		executeCmdFutures := xiter.SeqOf[future.Future[*message.ExecutionResult]]()
		repliesFuture := future.NewValue(xiter.SeqOf[discord.Message]())
		repliesToBeDeletedFuture := future.NewValue(xiter.SeqOf[discord.Message]())
		switch event := e.(type) {
		case *events.MessageCreate:
			gm = event.GenericMessage
			executeCmdFutures = message.ExecuteCmds(ctx, q.options, gm)
		case *events.MessageUpdate:
			gm = event.GenericMessage
			executeCmdFutures = message.ExecuteCmds(ctx, q.options, gm)
			if gm.Message.Flags.Has(discord.MessageFlagHasThread) {
				repliesFuture = message.GetRepliesInThread(q.options, gm)
				repliesToBeDeletedFuture = message.GetReplies(q.options, gm)
			} else {
				repliesFuture = message.GetReplies(q.options, gm)
			}
		case *events.MessageDelete:
			gm = event.GenericMessage
			repliesFuture = message.GetReplies(q.options, gm)
		default:
			slog.Error("Unknown event type", slog.Any("event", event))
			return
		}
		cmdResults := future.Await(ctx, executeCmdFutures)
		replies, err := repliesFuture.Await(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("Failed to get replies from message", slog.Any("err", err))
			return
		}
		repliesToBeDeleted, err := repliesToBeDeletedFuture.Await(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("Failed to get replies to be deleted from message", slog.Any("err", err))
			return
		}

		deleted := q.syncMap.CompareAndDelete(id, ch)
		if deleted {
			// If the event was deleted, stop processing.
			for z := range xiter.ZipLongest(cmdResults, replies) {
				if z.OK1 && z.OK2 {
					// If both the command result and replies are available, send the reply.
					executionResult := z.V1.Value
					reply := z.V2
					if _, err := message.UpdateMessage(q.options, gm, reply, executionResult).Await(ctx); err != nil {
						slog.Error("Failed to update message", slog.Any("replyID", reply.ID), slog.Any("err", err))
						return
					}
				} else if z.OK1 {
					executionResult := z.V1.Value
					if _, err := message.SendReply(q.options, gm, executionResult).Await(ctx); err != nil {
						slog.Error("Failed to send reply", slog.Any("err", err))
						return
					}
				} else { // z.OK2
					reply := z.V2
					if _, err := message.DeleteMessage(q.options, gm, reply.ID).Await(ctx); err != nil {
						slog.Error("Failed to delete reply", slog.Any("replyID", reply.ID), slog.Any("err", err))
						return
					}
				}
			}
			for reply := range repliesToBeDeleted {
				if _, err := message.DeleteMessage(q.options, gm, reply.ID).Await(ctx); err != nil {
					slog.Error("Failed to delete reply", slog.Any("replyID", reply.ID), slog.Any("err", err))
					return
				}
			}
			return
		}
	}
}

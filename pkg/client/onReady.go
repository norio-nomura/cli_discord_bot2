// Package client provides the Discord bot client initialization and event handler registration.
package client

import (
	"context"
	"log/slog"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/disgo/rest"
	"github.com/norio-nomura/cli_discord_bot2/pkg/options"
)

// onReady is an internal event handler for the Discord Ready event.
// It sets the bot's presence and updates the nickname in all joined guilds if needed.
func onReady(o *options.Options, e *events.Ready) {
	nickname, playing := o.Discord()
	err := e.Client().SetPresence(
		context.TODO(),
		gateway.WithPlayingActivity(playing),
	)
	if err != nil {
		slog.Error("Failed to set presence", slog.Any("err", err))
	} else {
		slog.Info("`ready`: changed status to", slog.String("playing", playing))
	}
	for _, g := range e.Guilds {
		member, err := e.Client().Rest().GetMember(g.ID, e.User.ID)
		if err != nil {
			slog.Error("Failed to get member", slog.Any("guild.id", g.ID), slog.Any("err", err))
			continue
		}
		if member.Nick == nil || *member.Nick != nickname {
			// UpdateCurrentMember() produces marshalling response error.
			// err := e.Client().Rest().UpdateCurrentMember(g.ID, nickname)
			err := e.Client().Rest().Do(rest.UpdateCurrentMember.Compile(nil, g.ID), discord.CurrentMemberUpdate{Nick: nickname}, nil)
			if err != nil {
				slog.Error("Failed to update member nickname", slog.Any("guild.id", g.ID), slog.Any("err", err))
			} else {
				slog.Info("Updated member nickname", slog.Any("guild.id", g.ID), slog.String("nickname", nickname))
			}
		}
	}
}

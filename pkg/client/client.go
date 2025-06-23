// Package client provides the Discord bot client initialization and event handler registration.
package client

import (
	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"

	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"

	"github.com/norio-nomura/cli_discord_bot2/pkg/options"
)

// New creates and returns a new Discord bot client configured with the given options.
// It registers all necessary event listeners for message and ready events.
func New(o *options.Options) (bot.Client, error) {
	handler := messageEventsHandler{options: o}
	return disgo.New(o.DiscordTokens[0],
		bot.WithEventListeners(
			bot.NewListenerFunc(func(e *events.Ready) { onReady(o, e) }),
			bot.NewListenerFunc(handler.onMessageCreate),
			bot.NewListenerFunc(handler.onMessageUpdate),
			bot.NewListenerFunc(handler.onMessageDelete),
		),
		bot.WithEventManagerConfigOpts(
			bot.WithAsyncEventsEnabled(),
		),
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(
				gateway.IntentGuilds,
				gateway.IntentGuildMessages,
				gateway.IntentDirectMessages,
			),
		),
	)
}

/*
Copyright © 2025 Norio Nomura

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/norio-nomura/cli_discord_bot2/pkg/client"
	"github.com/norio-nomura/cli_discord_bot2/pkg/options"
)

func main() {
	var (
		debug                bool
		readOptionsFromStdin bool
		opt                  *options.Options
	)
	flag.BoolVar(&debug, "debug", false, "Enable debug mode")
	flag.BoolVar(&readOptionsFromStdin, "stdin", false, "Read JSON from stdin")
	flag.Parse()
	if readOptionsFromStdin {
		optFromStdin, err := options.FromStdin()
		if err != nil {
			panic(err)
		}
		opt = optFromStdin
	} else {
		optFromEnv, err := options.FromEnv()
		if err != nil {
			panic(err)
		}
		if debug { // Do not call ExecWithPassingOptionsToStdin() if debug is enabled
			opt = optFromEnv
		} else {
			err = optFromEnv.ExecWithPassingOptionsToStdin()
			// if ExecWithPassingOptionsToStdin() returns, it means there was an error
			panic(err)
		}
	}
	bot, err := client.New(opt)
	if err != nil {
		panic(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	defer stop()
	defer bot.Close(ctx)
	if err := bot.OpenGateway(ctx); err != nil {
		panic(err)
	}
	<-ctx.Done()
}

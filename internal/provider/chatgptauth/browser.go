package chatgptauth

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type CommandRunner func(context.Context, string, ...string) error

type OpenCommandBrowser struct {
	Run CommandRunner
}

func (b OpenCommandBrowser) Open(ctx context.Context, rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return fmt.Errorf("ChatGPT OAuth login URL is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	run := b.Run
	if run == nil {
		run = defaultCommandRunner
	}
	if err := run(ctx, "open", rawURL); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if isContextError(err) {
			return err
		}
		return fmt.Errorf("open browser for ChatGPT OAuth login")
	}
	return nil
}

func defaultCommandRunner(ctx context.Context, name string, args ...string) error {
	return exec.CommandContext(ctx, name, args...).Run()
}

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/fxManagerProject/cli-installer/internal/actions"
	"github.com/fxManagerProject/cli-installer/internal/config"
	"github.com/fxManagerProject/cli-installer/internal/platform"
	"github.com/fxManagerProject/cli-installer/internal/theme"
	"github.com/fxManagerProject/cli-installer/internal/ui"
)

func main() {
	platformValue, err := platform.Detect()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}

	params := []config.Param{
		{
			Key:    "action",
			Flag:   "action",
			Usage:  "What do you want to do?",
			Prompt: true,
			Options: []config.Option{
				{Value: "install", Title: "Install", Desc: "Fresh fxServer & fxManager installation"},
				{Value: "update", Title: "Update", Desc: "Update fxManager installation"},
			},
		},
		{
			Key:     "dir",
			Flag:    "dir",
			Usage:   "Directory for the installation or directory where the project is located",
			Default: "./",
		},
		{
			Key:     "os",
			Flag:    "os",
			Usage:   fmt.Sprintf("Operating system (current: %s)", string(platformValue)),
			Default: string(platformValue),
			Action:  "install",
		},
		{
			Key:        "cfxlicense",
			Flag:       "cfxlicense",
			Usage:      "CFX license key used in creating server.cfg (obtained from https://portal.cfx.re/servers/registration-keys)",
			Prompt:     true,
			PromptType: config.PromptInput,
			Action:     "install",
		},
	}

	res, err := config.Parse(params)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}

	// Run UI with actions.Build routing the task creation
	if err := ui.Run(theme.Default(), res, actions.Build); err != nil {
		fmt.Fprintln(os.Stderr, "install failed:", err)
		os.Exit(1)
	}
}

package internal

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"log"
	"os"
	"strings"
)

func Run() {
	log.SetFlags(0)

	app := &cli.App{
		Name:    "shef",
		Usage:   "Shef is a powerful CLI tool for cooking up shell recipes.",
		Version: Version,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Aliases: []string{"d"},
				Usage:   "Enable debug output",
				Value:   false,
			},
			&cli.BoolFlag{
				Name:    "local",
				Aliases: []string{"L"},
				Usage:   "Force local recipes first",
			},
			&cli.BoolFlag{
				Name:    "user",
				Aliases: []string{"U"},
				Usage:   "Force user recipes first",
			},
			&cli.BoolFlag{
				Name:    "public",
				Aliases: []string{"P"},
				Usage:   "Force public recipes first",
			},
			&cli.StringFlag{
				Name:    "category",
				Aliases: []string{"c"},
				Usage:   "Filter by category",
			},
			&cli.PathFlag{
				Name:    "recipe-file",
				Aliases: []string{"r"},
				Usage:   "Path to the recipe file (note: additional recipe flags not supported)",
			},
		},
		Action: func(c *cli.Context) error {
			args := c.Args().Slice()
			if len(args) == 0 && !c.IsSet("recipe-file") {
				err := cli.ShowAppHelp(c)
				if err != nil {
					return err
				}
				return nil
			}

			sourcePriority := getSourcePriority(c)
			return dispatch(c, args, sourcePriority)
		},
		Commands: []*cli.Command{
			{
				Name:    "list",
				Aliases: []string{"ls", "l"},
				Usage:   "List available recipes",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "local",
						Aliases: []string{"l"},
						Usage:   "Filter to local recipes only",
					},
					&cli.BoolFlag{
						Name:    "user",
						Aliases: []string{"u"},
						Usage:   "Filter to user recipes only",
					},
					&cli.BoolFlag{
						Name:    "public",
						Aliases: []string{"p"},
						Usage:   "Filter to public recipes only",
					},
					&cli.BoolFlag{
						Name:    "json",
						Aliases: []string{"j"},
						Usage:   "Output results in JSON format",
					},
					&cli.StringFlag{
						Name:    "category",
						Aliases: []string{"c"},
						Usage:   "Filter by category",
					},
				},
				Action: func(c *cli.Context) error {
					args := c.Args().Slice()
					sourcePriority := getSourcePriority(c)
					return handleListCommand(c, args, sourcePriority)
				},
			},
			{
				Name:    "sync",
				Aliases: []string{"s"},
				Usage:   "Sync public recipes locally",
				Action: func(c *cli.Context) error {
					return syncPublicRecipes()
				},
			},
			{
				Name:    "which",
				Aliases: []string{"w"},
				Usage:   "Show the location of a recipe file",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "local",
						Aliases: []string{"L"},
						Usage:   "Force local recipes first",
					},
					&cli.BoolFlag{
						Name:    "user",
						Aliases: []string{"U"},
						Usage:   "Force user recipes first",
					},
					&cli.BoolFlag{
						Name:    "public",
						Aliases: []string{"P"},
						Usage:   "Force public recipes first",
					},
				},
				Action: func(c *cli.Context) error {
					args := c.Args().Slice()
					sourcePriority := getSourcePriority(c)
					return which(args, sourcePriority)
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		errorText := strings.ToLower(err.Error())
		formattedErr := fmt.Sprintf(
			"%s: %s",
			FormatText("Error", ColorRed, StyleBold),
			errorText,
		)
		log.Fatal(formattedErr)
	}
}

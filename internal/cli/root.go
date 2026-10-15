// Package cli implements the cswitch commands.
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dollarkillerx/my-claude/internal/i18n"
	"github.com/dollarkillerx/my-claude/internal/store"
)

// Version is set at build time with -ldflags "-X github.com/dollarkillerx/my-claude/internal/cli.Version=v1.2.3".
var Version = "dev"

func info(msg string) { fmt.Fprintln(os.Stderr, msg) }
func warn(msg string) { fmt.Fprintln(os.Stderr, i18n.T("prefix.warning")+msg) }

// NewRoot builds the command tree. Help text is translated when the tree is
// built, so the language must be set before calling it.
func NewRoot() *cobra.Command {
	var lang string
	var sw switchFlags
	root := &cobra.Command{
		Use:           "cswitch [name]",
		Short:         i18n.T("cmd.root.short"),
		Long:          i18n.T("cmd.root.long"),
		Version:       Version,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		// `cswitch` alone opens the account picker; `cswitch <name>` switches.
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				return switchByName(cmd, args[0], sw)
			}
			return pick(cmd.Context(), sw)
		},
	}
	sw.register(root)
	// cobra only defaults this on its own error path; switchByName uses it too.
	root.SuggestionsMinimumDistance = 2
	// Parsed early by Execute; registered here so it shows up in help and is accepted.
	root.PersistentFlags().StringVar(&lang, "lang", "", i18n.T("flag.lang"))
	root.AddCommand(addCmd(), loginCmd(), listCmd(), currentCmd(), useCmd(), usageCmd(),
		renameCmd(), removeCmd(), langCmd(), syncCmd())
	localizeHelp(root)
	// "cswitch <name>" would run a command of the same name instead of
	// switching, so new profiles must not take command names or aliases.
	for _, c := range root.Commands() {
		store.Reserve(c.Name())
		store.Reserve(c.Aliases...)
	}
	store.Reserve("help", "completion")
	return root
}

// Execute runs the CLI.
func Execute() {
	applyLanguage(os.Args[1:])
	if err := NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("prefix.error")+err.Error())
		os.Exit(1)
	}
}

// applyLanguage picks the UI language: --lang, then CSWITCH_LANG, then the
// language saved with `cswitch lang`, then the system language.
func applyLanguage(args []string) {
	if l := langFromArgs(args); l != "" {
		if !i18n.SetLang(l) {
			fmt.Fprintln(os.Stderr, i18n.T("err.bad_lang", l))
			os.Exit(2)
		}
		return
	}
	if os.Getenv("CSWITCH_LANG") != "" {
		return // honored by i18n's own detection
	}
	if saved := store.LoadSettings().Lang; saved != "" {
		i18n.SetLang(saved)
	}
}

// langFromArgs finds --lang before cobra parses flags, because help text is
// translated while the command tree is built.
func langFromArgs(args []string) string {
	for i, a := range args {
		if a == "--" {
			break
		}
		if v, ok := strings.CutPrefix(a, "--lang="); ok {
			return v
		}
		if a == "--lang" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// localizeHelp translates cobra's built-in help headings and flags.
func localizeHelp(root *cobra.Command) {
	tmpl := root.UsageTemplate()
	for _, h := range []struct{ en, key string }{
		{"Usage:", "help.usage"},
		{"Aliases:", "help.aliases"},
		{"Examples:", "help.examples"},
		{"Available Commands:", "help.commands"},
		{"Additional Commands:", "help.additional_commands"},
		{"Global Flags:", "help.global_flags"},
		{"Flags:", "help.flags"},
		{"Additional help topics:", "help.topics"},
		{`Use "{{.CommandPath}} [command] --help" for more information about a command.`, "help.more"},
	} {
		tmpl = strings.ReplaceAll(tmpl, h.en, i18n.T(h.key))
	}
	root.SetUsageTemplate(tmpl)
	root.SetHelpCommand(&cobra.Command{
		Use:   "help [command]",
		Short: i18n.T("cmd.help.short"),
		Run: func(c *cobra.Command, args []string) {
			cmd, _, err := c.Root().Find(args)
			if cmd == nil || err != nil {
				cmd = c.Root()
			}
			cmd.Help()
		},
	})
	root.CompletionOptions.HiddenDefaultCmd = true
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		c.InitDefaultHelpFlag()
		if f := c.Flags().Lookup("help"); f != nil {
			f.Usage = i18n.T("flag.help", c.Name())
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
	root.InitDefaultVersionFlag()
	if f := root.Flags().Lookup("version"); f != nil {
		f.Usage = i18n.T("flag.version")
	}
}

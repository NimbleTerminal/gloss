package main

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"nimbleterminal.dev/gloss/skills"
)

// gloss has one default verb, view, and a few others. A verb is recognized
// only as the first argument and only by its exact name; any other first
// argument is a file or an option, and the command is an implicit view. A file
// that is named like a verb is opened with the verb spelled out: gloss view skill.
type verb struct {
	Name    string
	Summary string // One line, for help and the reference.
}

var verbs = []verb{
	{"view", "open files in the pager, or export, extract, pick and serve them (the default)"},
	{"skill", "print the skill that teaches agents to use gloss, install it, or print the schema of its JSON"},
	{"help", "say how to use gloss, or one of its commands"},
}

// splitVerb separates the verb from the arguments after it. A first argument
// that names no verb leaves them all to the implicit view.
func splitVerb(args []string) (name string, rest []string) {
	if len(args) > 0 && slices.ContainsFunc(verbs, func(v verb) bool { return v.Name == args[0] }) {
		return args[0], args[1:]
	}
	return "view", args
}

const skillUsage = `Usage: gloss skill show
       gloss skill install [agent] [FOLDER]
       gloss skill install rex [FOLDER]
       gloss skill schema

show prints the skill (SKILL.md) that teaches an agent gloss's headless
surface, so it always matches this binary. Without a command, gloss skill
prints this help.

install agent, the default, writes it as gloss/SKILL.md under the shared
agents folder (~/.agents/skills, which Codex and others read) and the skills
folder of each agent found on this machine, or under FOLDER, and prints the
paths it wrote.

install rex writes gloss.lua, gloss-companion.lua, and grex into the Rex
terminal multiplexer's config folder (~/.config/rex, $XDG_CONFIG_HOME/rex, or
the folder of $REX_CONFIG, with grex also in ~/.local/bin), or under FOLDER:
the actions grex (open gloss in a split), gloss_companion (cmd+shift+g: a gloss
split that follows the block beside it), and gloss_push, plus the grex shell
script (grex [right|left|above|below] [GLOSS_ARGS...]). It adds
require("gloss") to init.lua, once, making the file when there is none, and has
a running Rex reload its config. A folder named agent or rex is ./agent or
./rex.

schema prints the JSON Schema of the objects gloss writes for a program:
the --json results of an export, --text, --grep, and --info, and the objects
of a session started without a terminal (start, --status, --resume --json).`

// parseSkill is the skill verb: show prints it, and done says nothing more is
// to be done; install leaves the folder in opts for run, as an option does.
func parseSkill(args []string, opts options, out io.Writer) (options, bool, error) {
	// Help is help wherever it comes, after the subcommand too; and the skill
	// verb has no other option, so one is never a folder: gloss skill install
	// --help once wrote the skill under a folder named --help (issue 6).
	for _, a := range args {
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprintln(out, skillUsage)
			return opts, true, nil
		case strings.HasPrefix(a, "-"):
			return opts, false, fmt.Errorf("skill: unknown option %s (see gloss skill --help)", a)
		}
	}
	// The skill is long, so bare skill is help; printing it takes asking.
	if len(args) == 0 {
		fmt.Fprintln(out, skillUsage)
		return opts, true, nil
	}
	sub, args := args[0], args[1:]
	switch {
	case sub == "show" && len(args) == 0:
		fmt.Fprint(out, skills.Gloss)
		return opts, true, nil
	case sub == "install":
		return parseSkillInstall(args, opts)
	case sub == "schema" && len(args) == 0:
		return opts, true, writeSchema(out)
	case sub == "show" || sub == "schema":
		return opts, false, fmt.Errorf("skill %s: too many arguments (see gloss skill --help)", sub)
	}
	return opts, false, fmt.Errorf("skill: unknown command %q: show, install, or schema (see gloss skill --help)", sub)
}

// skillKinds are what gloss skill install installs, by name; the first is the
// default. A folder of one of these names is ./agent or ./rex.
var skillKinds = []string{"agent", "rex"}

// parseSkillInstall reads the arguments after install: a kind, a folder, both
// or neither. With no folder, the agent skill goes where the agents found here
// read, and the Rex files where Rex reads its config.
func parseSkillInstall(args []string, opts options) (options, bool, error) {
	opts.SkillKind = skillKinds[0]
	if len(args) > 0 && slices.Contains(skillKinds, args[0]) {
		opts.SkillKind, args = args[0], args[1:]
	}
	switch {
	case len(args) == 0:
		opts.SkillDetect = true
	case len(args) > 1:
		return opts, false, fmt.Errorf("skill install %s: too many arguments (see gloss skill --help)", opts.SkillKind)
	case args[0] == "":
		return opts, false, fmt.Errorf("skill install %s: the folder is empty (see gloss skill --help)", opts.SkillKind)
	default:
		opts.SkillInstall = args[0]
	}
	return opts, false, nil
}

// parseHelp is the help verb: with no argument, the usage of gloss; with a
// verb, that verb's.
func parseHelp(args []string, usage func(), out io.Writer) (bool, error) {
	if len(args) == 0 {
		usage()
		return true, nil
	}
	if len(args) > 1 {
		return false, fmt.Errorf("help takes one command: %s", verbNames())
	}
	switch args[0] {
	case "view":
		usage()
	case "skill":
		fmt.Fprintln(out, skillUsage)
	case "help":
		fmt.Fprintln(out, "Usage: gloss help [command]\n\nCommands: "+verbNames())
	default:
		return false, fmt.Errorf("no help for %q: the commands are %s", args[0], verbNames())
	}
	return true, nil
}

func verbNames() string {
	var names []string
	for _, v := range verbs {
		names = append(names, v.Name)
	}
	return strings.Join(names, ", ")
}

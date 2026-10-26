package main

import "nimbleterminal.dev/gloss/internal/i18n"

// language is the tag gloss's own words follow: --lang when given, else, in
// the viewer, the environment's (GLOSS_LANG, LC_ALL, LC_MESSAGES, LANG). What
// agents read stays English: the headless modes and a detached start or
// server never look at the environment, and a detached page follows the
// browser of the human who opens it. "" is English.
func language(opts options, stdinTTY bool, getenv func(string) string) string {
	switch {
	case opts.Lang != "":
		return opts.Lang
	case opts.Output != "" || opts.OutputDir != "" || opts.Info || opts.Apps || opts.Grep != nil || opts.Text || opts.Print || opts.SkillDetect || opts.SkillInstall != "":
		return ""
	case opts.Detached != "" || (opts.Pick || opts.Serve || opts.PickWeb) && !stdinTTY:
		return ""
	}
	return i18n.FromEnv(getenv)
}

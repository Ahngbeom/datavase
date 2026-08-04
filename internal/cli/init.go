package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/secret"
)

// Wizard writes the first configuration file by asking for what goes in it.
//
// It exists because the alternative is a YAML example printed to stderr, which
// asks someone who has not run datavase yet to know what "env" changes and to
// get a file past a parser that rejects unknown keys. The questions can be
// answered without knowing any of that.
//
// Its dependencies are fields for the same reason App's are: the whole flow —
// the retry after an unreachable server, the file that gets written — is then
// exercisable without a terminal, a keychain or a database.
type Wizard struct {
	// Path is where the configuration will be written. An existing file there
	// is never touched.
	Path string
	Out  io.Writer

	// Ask reads one line, returning def when the answer is empty.
	Ask func(prompt, def string) (string, error)
	// Choose picks one of options, returning def when the answer is empty.
	Choose func(prompt string, options []string, def int) (int, error)
	// ReadPassword reads without echoing.
	ReadPassword func(prompt string) (string, error)

	Secrets secret.Store
	// Probe opens a connection and returns the server version. The wizard uses
	// the same one `dv check` does, so a datasource it accepts is one that
	// really answered.
	Probe func(ctx context.Context, ds *config.DataSource, password string) (string, error)
}

// envChoices are the env answers, safest first: the default has to be the one
// that guards most, because it is what an unanswered question becomes.
var envChoices = []config.Env{config.EnvDev, config.EnvStage, config.EnvProd}

// Indices into envChoices, so a test can name an answer rather than count.
const (
	envChoiceDev = iota
	envChoiceStage
	envChoiceProd
)

// envDescriptions say what the answer changes, in terms of what the user will
// see happen. "production" describes the database; it does not describe what
// datavase will do about it.
//
// These match guard.Evaluate, which keys only off EnvProd — stage and dev are
// the same bargain today, and saying otherwise here would be a promise the
// guard does not keep.
var envDescriptions = map[config.Env]string{
	config.EnvDev:   "statements that change data ask before they run",
	config.EnvStage: "the same, for a shared environment",
	config.EnvProd:  "they are refused until you unlock writes for the session",
}

// presetDescriptions say what choosing a keyboard will feel like. The list of
// presets comes from the keymap package; only the prose lives here, and a test
// checks none is missing.
var presetDescriptions = map[keymap.Preset]string{
	keymap.PresetVim:      "modal — press i to type, Esc to leave insert mode",
	keymap.PresetDataGrip: "an ordinary editor; typing types",
	keymap.PresetVSCode:   "an ordinary editor, with VS Code's keys",
}

// Run asks the questions and writes the file, reporting why it could not.
//
// Every question can be the last one: Ctrl-D and a pipe that runs out arrive
// the same way, and "EOF" is the reader's word for it rather than anything the
// person who pressed the key would recognise. Translating it once here is why
// no individual question has to.
func (w *Wizard) Run(ctx context.Context) error {
	err := w.run(ctx)
	if errors.Is(err, io.EOF) {
		return errors.New("setup was interrupted; nothing was written")
	}
	return err
}

func (w *Wizard) run(ctx context.Context) error {
	// Checked before the first question rather than before the write: someone
	// whose config already exists should not answer eight things to be told so.
	// Anything other than "not there" is treated as "there", because writing
	// over a file we could not read is the one mistake with nothing to undo it.
	if _, err := os.Stat(w.Path); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%s already exists; edit it rather than starting over", w.Path)
	}

	fmt.Fprintf(w.Out, "Setting up datavase. The value in brackets is what Enter takes.\n\n")

	name, err := w.askRequired("a name for this database", "local")
	if err != nil {
		return err
	}

	env, err := w.askEnv()
	if err != nil {
		return err
	}

	ds, password, err := w.askReachable(ctx, name, env)
	if err != nil {
		return err
	}

	preset, err := w.askKeyboard()
	if err != nil {
		return err
	}

	if err := writeConfig(w.Path, ds, preset); err != nil {
		return err
	}
	// After the file, so a keychain that refuses leaves a config behind to fix
	// with `dv auth` rather than nothing at all.
	if err := w.Secrets.Set(ds.Name, password); err != nil {
		return fmt.Errorf("storing the password: %w", err)
	}

	fmt.Fprintf(w.Out, "\nWrote %s and stored the password.\n\nRun: dv\n", w.Path)
	return nil
}

// askReachable collects the connection and does not return until the server
// has answered.
//
// Testing before writing is the point. A wrong port written to the file is one
// the user has to find and hand-edit, having been told only that datavase does
// not work — which is the situation the wizard exists to prevent.
func (w *Wizard) askReachable(ctx context.Context, name string, env config.Env) (config.DataSource, string, error) {
	ds := config.DataSource{
		Name: name,
		Env:  env,
		Host: "127.0.0.1",
		Port: config.DefaultPort,
		User: "root",
	}

	for {
		host, err := w.askRequired("host", ds.Host)
		if err != nil {
			return ds, "", err
		}
		port, err := w.askPort(ds.Port)
		if err != nil {
			return ds, "", err
		}
		user, err := w.askRequired("user", ds.User)
		if err != nil {
			return ds, "", err
		}
		database, err := w.Ask("database (optional)", ds.Database)
		if err != nil {
			return ds, "", err
		}

		ds.Host, ds.Port, ds.User = host, port, strings.TrimSpace(user)
		ds.Database = strings.TrimSpace(database)
		// The same default Load would apply, so what is probed is what the file
		// will mean — a prod datasource is tested over TLS, as it will be used.
		ds.TLS = config.DefaultTLSMode(env)

		password, err := w.ReadPassword(fmt.Sprintf("password for %s@%s: ", ds.User, ds.Host))
		if err != nil {
			return ds, "", err
		}

		version, err := w.Probe(ctx, &ds, password)
		if err == nil {
			fmt.Fprintf(w.Out, "\nConnected — server %s\n", version)
			return ds, password, nil
		}

		// The server's own words. "That did not work" sends people to look at
		// the host when the account was the problem.
		fmt.Fprintf(w.Out, "\nCannot reach it: %v\n\nLet's try again.\n\n", err)
	}
}

// askPort keeps asking until the answer is a port.
//
// Rejecting it here rather than at the end means a typo costs one question
// instead of the whole form.
func (w *Wizard) askPort(def int) (int, error) {
	for {
		answer, err := w.Ask("port", strconv.Itoa(def))
		if err != nil {
			return 0, err
		}

		port, convErr := strconv.Atoi(strings.TrimSpace(answer))
		if convErr == nil && port > 0 && port <= 65535 {
			return port, nil
		}
		fmt.Fprintf(w.Out, "%q is not a port number.\n", answer)
	}
}

// askRequired keeps asking until there is an answer, for the fields config
// validation refuses to be without.
func (w *Wizard) askRequired(prompt, def string) (string, error) {
	for {
		answer, err := w.Ask(prompt, def)
		if err != nil {
			return "", err
		}
		if trimmed := strings.TrimSpace(answer); trimmed != "" {
			return trimmed, nil
		}
		fmt.Fprintf(w.Out, "%s cannot be empty.\n", prompt)
	}
}

func (w *Wizard) askEnv() (config.Env, error) {
	options := make([]string, len(envChoices))
	for i, env := range envChoices {
		options[i] = fmt.Sprintf("%-6s %s", env, envDescriptions[env])
	}

	choice, err := w.Choose("what kind of database is this?", options, envChoiceDev)
	if err != nil {
		return "", err
	}
	if choice < 0 || choice >= len(envChoices) {
		choice = envChoiceDev
	}
	return envChoices[choice], nil
}

// askKeyboard is the question that makes the modal default survivable for
// someone who did not choose it.
//
// The presets come from the keymap package rather than a list here: one that
// this never offers is one nobody finds without reading the source.
func (w *Wizard) askKeyboard() (keymap.Preset, error) {
	presets := keymap.Presets()

	options := make([]string, len(presets))
	def := 0
	for i, p := range presets {
		options[i] = fmt.Sprintf("%-9s %s", p, presetDescriptions[p])
		if p == keymap.DefaultPreset {
			def = i
		}
	}

	choice, err := w.Choose("how do you want to type?", options, def)
	if err != nil {
		return "", err
	}
	if choice < 0 || choice >= len(presets) {
		choice = def
	}
	return presets[choice], nil
}

// writeConfig renders the file.
//
// It is assembled rather than marshalled because the comments are the point:
// what env changes is the one thing in here nobody can work out by looking at
// the value, and yaml.Marshal would drop every line of it.
func writeConfig(path string, ds config.DataSource, preset keymap.Preset) error {
	var b strings.Builder

	b.WriteString("# datavase configuration.\n#\n")
	b.WriteString("# env decides what happens to a statement that changes data:\n")
	for _, env := range envChoices {
		fmt.Fprintf(&b, "#   %-6s %s\n", env, envDescriptions[env])
	}

	b.WriteString("datasources:\n")
	fmt.Fprintf(&b, "  - name: %s\n", ds.Name)
	fmt.Fprintf(&b, "    env: %s\n", ds.Env)
	fmt.Fprintf(&b, "    host: %s\n", ds.Host)
	fmt.Fprintf(&b, "    port: %d\n", ds.Port)
	fmt.Fprintf(&b, "    user: %s\n", ds.User)
	if ds.Database != "" {
		fmt.Fprintf(&b, "    database: %s\n", ds.Database)
	}

	b.WriteString("\n# The keyboard. Change it here, or from the command palette.\n")
	for _, p := range keymap.Presets() {
		fmt.Fprintf(&b, "#   %-9s %s\n", p, presetDescriptions[p])
	}
	b.WriteString("keymap:\n")
	fmt.Fprintf(&b, "  preset: %s\n", preset)

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating the configuration directory: %w", err)
	}
	// Owner only: it holds no password, but it names a host and an account
	// inside someone's network.
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

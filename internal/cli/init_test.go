package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/secret"
)

// script answers the wizard's questions in order and runs out, which is what
// an interrupted session or a closed pipe looks like from inside.
type script struct {
	answers  []string
	choices  []int
	password string

	probe func(ds *config.DataSource, password string) (string, error)

	// asked records every prompt, so a test can prove the wizard came back to
	// a question rather than accepting the answer that failed.
	asked []string
	// offered records the default each Choose was willing to take, so a test
	// can prove a question has none.
	offered []int
}

func (s *script) ask(prompt, def string) (string, error) {
	s.asked = append(s.asked, prompt)
	if len(s.answers) == 0 {
		return "", io.EOF
	}
	answer := s.answers[0]
	s.answers = s.answers[1:]
	if answer == "" {
		return def, nil
	}
	return answer, nil
}

func (s *script) choose(prompt string, _ []string, def int) (int, error) {
	s.asked = append(s.asked, prompt)
	s.offered = append(s.offered, def)

	if len(s.choices) == 0 {
		// A real prompt keeps asking until the answer is valid, so a question
		// with no default and nobody left to answer it is the end of the input
		// rather than a silent fallback to something nobody chose.
		if def < 0 {
			return 0, io.EOF
		}
		return def, nil
	}
	choice := s.choices[0]
	s.choices = s.choices[1:]
	return choice, nil
}

func (s *script) readPassword(string) (string, error) { return s.password, nil }

// newWizard wires a wizard to a script and a config path that does not exist
// yet, which is the state the wizard is for.
func newWizard(t *testing.T, s *script) (*Wizard, string) {
	t.Helper()

	// Nested so that a run also proves the wizard creates the directory: the
	// config directory does not exist on a machine that has never been set up.
	path := filepath.Join(t.TempDir(), "datavase", "config.yaml")

	if s.probe == nil {
		s.probe = func(*config.DataSource, string) (string, error) { return "11.4.2-MariaDB", nil }
	}

	return &Wizard{
		Path:         path,
		Out:          &bytes.Buffer{},
		Ask:          s.ask,
		Choose:       s.choose,
		ReadPassword: s.readPassword,
		Secrets:      secret.NewMemory(),
		Probe: func(_ context.Context, ds *config.DataSource, password string) (string, error) {
			return s.probe(ds, password)
		},
	}, path
}

// answers is one complete pass: name, host, port, user, database.
func answers() []string {
	return []string{"local", "127.0.0.1", "13306", "root", "app_db"}
}

// choices is one complete pass of the numbered questions: the env, which has
// no default and so has to be answered. The keyboard question has one, which
// an unanswered script takes.
func choices() []int {
	return []int{envChoiceDev}
}

// The wizard is the config package's first writer, and Parse rejects unknown
// keys. A file it cannot read back is a machine that cannot be set up at all,
// with the failure landing on the next run rather than on this one.
func TestTheWizardWritesAConfigItCanReadBack(t *testing.T) {
	s := &script{answers: answers(), choices: choices(), password: "secret"}
	w, path := newWizard(t, s)

	if err := w.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("the wizard wrote a config it cannot read: %v", err)
	}

	if len(cfg.DataSources) != 1 {
		t.Fatalf("got %d datasources, want 1", len(cfg.DataSources))
	}
	ds := cfg.DataSources[0]
	if ds.Name != "local" {
		t.Errorf("name = %q, want %q", ds.Name, "local")
	}
	if ds.Host != "127.0.0.1" {
		t.Errorf("host = %q, want %q", ds.Host, "127.0.0.1")
	}
	if ds.Port != 13306 {
		t.Errorf("port = %d, want %d", ds.Port, 13306)
	}
	if ds.User != "root" {
		t.Errorf("user = %q, want %q", ds.User, "root")
	}
	if ds.Database != "app_db" {
		t.Errorf("database = %q, want %q", ds.Database, "app_db")
	}
}

// The file holds no password, so it is not a secret — but it names a host and
// an account inside someone's network, which is nobody else's business.
func TestTheWrittenConfigIsReadableOnlyByItsOwner(t *testing.T) {
	s := &script{answers: answers(), choices: choices(), password: "secret"}
	w, path := newWizard(t, s)

	if err := w.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config mode = %04o, want %04o", perm, 0o600)
	}
}

// Without this the wizard leaves the user one step short: the next thing they
// type is `dv`, and it would answer "no password stored".
func TestTheWizardStoresThePasswordUnderTheDataSourceName(t *testing.T) {
	s := &script{answers: answers(), choices: choices(), password: "secret"}
	w, _ := newWizard(t, s)

	if err := w.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	stored, err := w.Secrets.Get("local")
	if err != nil {
		t.Fatalf("no password was stored: %v", err)
	}
	if stored != "secret" {
		t.Errorf("stored password = %q, want %q", stored, "secret")
	}
}

// A keyboard chosen here and not written is a question asked for nothing, and
// the preset has to be one the keymap package will actually accept.
func TestTheChosenKeyboardReachesTheKeyMap(t *testing.T) {
	// The second Choose is the keyboard; the first is env. Index 1 is the
	// second preset, whatever the presets happen to be.
	presets := keymap.Presets()
	if len(presets) < 2 {
		t.Skip("only one preset to choose from")
	}
	want := presets[1]

	s := &script{answers: answers(), choices: []int{envChoiceDev, 1}, password: "secret"}
	w, path := newWizard(t, s)

	if err := w.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if cfg.Keymap.Preset != string(want) {
		t.Errorf("preset = %q, want %q", cfg.Keymap.Preset, want)
	}
	if _, err := keymap.FromConfig(cfg.Keymap.Preset, cfg.Keymap.Actions); err != nil {
		t.Errorf("the wizard wrote a preset the keymap refuses: %v", err)
	}
}

// The env answer decides whether writes are refused against this datasource,
// so an answer that did not reach the file is a production database configured
// as a development one.
func TestTheChosenEnvReachesTheFile(t *testing.T) {
	s := &script{answers: answers(), choices: []int{envChoiceProd}, password: "secret"}
	w, path := newWizard(t, s)

	if err := w.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if cfg.DataSources[0].Env != config.EnvProd {
		t.Errorf("env = %q, want %q", cfg.DataSources[0].Env, config.EnvProd)
	}
}

// Writing first and testing afterwards leaves a wrong host in a file that the
// user then has to find and edit by hand — which is the very thing the wizard
// exists to spare them.
func TestAnUnreachableServerIsAskedAgainRatherThanWritten(t *testing.T) {
	var attempts int
	s := &script{
		// Two passes: the first host is wrong, the second is not.
		answers:  append([]string{"local", "10.0.0.1", "13306", "root", "app_db"}, "127.0.0.1", "13306", "root", "app_db"),
		choices:  choices(),
		password: "secret",
	}
	s.probe = func(ds *config.DataSource, _ string) (string, error) {
		attempts++
		if ds.Host == "10.0.0.1" {
			return "", errors.New("dial tcp 10.0.0.1:13306: connect: no route to host")
		}
		return "11.4.2-MariaDB", nil
	}

	w, path := newWizard(t, s)
	if err := w.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if attempts != 2 {
		t.Errorf("the server was probed %d times, want 2", attempts)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if got := cfg.DataSources[0].Host; got != "127.0.0.1" {
		t.Errorf("host = %q, want the host that answered", got)
	}
}

// The reason has to reach the user. "It did not work, try again" sends people
// looking at the host when the account was the problem.
func TestAnUnreachableServerSaysWhy(t *testing.T) {
	out := &bytes.Buffer{}
	s := &script{answers: answers(), choices: choices(), password: "secret"}
	s.probe = func(*config.DataSource, string) (string, error) {
		return "", errors.New("Access denied for user 'root'")
	}

	w, _ := newWizard(t, s)
	w.Out = out

	// The script runs out on the second pass, which ends the wizard.
	if err := w.Run(context.Background()); err == nil {
		t.Fatal("Run() = nil, want an error when the input ends")
	}
	if !strings.Contains(out.String(), "Access denied for user 'root'") {
		t.Errorf("output = %q, want it to carry the server's own reason", out)
	}
}

// A wizard that cannot be answered has to stop. Looping on EOF would spin
// forever against a closed pipe, printing the same question.
func TestTheWizardStopsWhenTheInputEnds(t *testing.T) {
	s := &script{password: "secret"}
	w, path := newWizard(t, s)

	err := w.Run(context.Background())
	if err == nil {
		t.Fatal("Run() = nil, want an error when there is nothing to read")
	}
	// "EOF" is the reader's word for a closed input and says nothing to
	// someone who has just pressed Ctrl-D partway through a setup wizard.
	if strings.Contains(err.Error(), "EOF") {
		t.Errorf("error = %q, want words rather than the reader's", err)
	}
	if !strings.Contains(err.Error(), "nothing was written") {
		t.Errorf("error = %q, want it to say the file was not created", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a config was written from an unanswered wizard: %v", err)
	}
}

// Overwriting is the one failure the user cannot undo: their datasources are
// in that file and nowhere else. Refusing and naming the path is the whole of
// what this needs to do.
func TestTheWizardWillNotOverwriteAnExistingConfig(t *testing.T) {
	s := &script{answers: answers(), choices: choices(), password: "secret"}
	w, path := newWizard(t, s)

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(testYAML), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err := w.Run(context.Background())
	if err == nil {
		t.Fatal("Run() = nil, want a refusal")
	}
	if want := errConfigExists(path).Error(); err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}

	// And it says so before asking anything. Refusing at the write alone would
	// be correct and would still make someone answer eight questions to find
	// out, which is why the check before the first one is worth its own line.
	if len(s.asked) != 0 {
		t.Errorf("the wizard asked %v before refusing", s.asked)
	}

	kept, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}
	if string(kept) != testYAML {
		t.Error("the existing config was overwritten")
	}
}

// The wizard cannot work out which kind of database this is, and it is the one
// answer that decides whether the guard does anything at all. A default here is
// a guess that Enter accepts, and the guess that is wrong on a production
// database is the one that leaves it unguarded.
func TestTheEnvQuestionOffersNoDefault(t *testing.T) {
	s := &script{answers: answers(), choices: choices(), password: "secret"}
	w, _ := newWizard(t, s)

	if err := w.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(s.offered) < 2 {
		t.Fatalf("the wizard asked %d numbered questions, want the env and the keyboard", len(s.offered))
	}
	if s.offered[0] >= 0 {
		t.Errorf("the env question offers default %d, want none", s.offered[0])
	}
	// The keyboard keeps its default: it is a preference, and being wrong about
	// it costs keystrokes rather than a guard.
	if s.offered[1] < 0 {
		t.Error("the keyboard question offers no default, want the configured one")
	}
}

// Having no default means the question has to be answered — not that something
// is picked when nobody does.
func TestAnUnansweredEnvStopsTheWizard(t *testing.T) {
	s := &script{answers: answers(), password: "secret"}
	w, path := newWizard(t, s)

	if err := w.Run(context.Background()); err == nil {
		t.Fatal("Run() = nil, want the wizard to stop rather than choose an env")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a config was written with an env nobody chose: %v", err)
	}
}

// Choose is a field, so an implementation that answers out of range is
// somebody else's bug — but it must not become an env nobody chose. Refusing is
// the only outcome that cannot be mistaken for a choice.
//
// The keyboard question resolves the same answer to its default instead, and
// that asymmetry is the point: being wrong about a preference costs keystrokes,
// and being wrong about this costs the guard.
func TestAnOutOfRangeChoiceIsNotResolvedIntoAnEnv(t *testing.T) {
	s := &script{answers: answers(), password: "secret"}
	w, path := newWizard(t, s)
	w.Choose = func(string, []string, int) (int, error) { return len(envChoices), nil }

	if err := w.Run(context.Background()); err == nil {
		t.Fatal("Run() = nil, want a refusal rather than an env nobody chose")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a config was written from an out-of-range choice: %v", err)
	}
}

// A preset added to the keymap and forgotten here is one the wizard offers as
// a bare name with nothing to choose it by — and the config file it writes
// then lists it the same way.
func TestEveryPresetIsDescribed(t *testing.T) {
	for _, p := range keymap.Presets() {
		if presetDescriptions[p] == "" {
			t.Errorf("the keymap offers preset %q but the wizard cannot describe it", p)
		}
	}
}

// The same for env, where the description is also what the written file says
// the value means.
func TestEveryEnvIsOfferedAndDescribed(t *testing.T) {
	offered := make(map[config.Env]bool, len(envChoices))
	for _, env := range envChoices {
		if envDescriptions[env] == "" {
			t.Errorf("the wizard offers env %q with nothing to choose it by", env)
		}
		offered[env] = true
	}

	for _, env := range []config.Env{config.EnvDev, config.EnvStage, config.EnvProd} {
		if !offered[env] {
			t.Errorf("env %q is valid configuration but the wizard never offers it", env)
		}
	}
}

// The wizard asks first and writes last, and the gap between the two is as
// long as someone takes to answer eight questions. A configuration that
// appears in that gap — another terminal, a dotfile sync finishing — is one it
// must not destroy.
//
// The check before the first question is a courtesy, so that nobody answers
// everything to be told the file was already there. The promise is kept at the
// write, which is the only moment that can keep it.
func TestAConfigThatAppearsWhileTheWizardIsAskingIsNotOverwritten(t *testing.T) {
	s := &script{answers: answers(), choices: choices(), password: "secret"}
	w, path := newWizard(t, s)

	// The last thing before the file is written, standing in for anything that
	// could have created it while the questions were being answered.
	s.probe = func(*config.DataSource, string) (string, error) {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		if err := os.WriteFile(path, []byte(testYAML), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		return "11.4.2-MariaDB", nil
	}

	err := w.Run(context.Background())
	if err == nil {
		t.Fatal("Run() = nil, want a refusal")
	}
	// The same words as the check before the questions. To whoever reads it
	// these are one fact, and a raw "writing …: file exists" would be the
	// filesystem's account of it rather than an answer.
	if want := errConfigExists(path).Error(); err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}

	kept, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}
	if string(kept) != testYAML {
		t.Error("the configuration that appeared mid-run was overwritten")
	}

	// A password stored for a datasource that is not in the file is an orphan
	// in the keychain, and nothing later will go looking for it.
	if _, err := w.Secrets.Get("local"); err == nil {
		t.Error("a password was stored for a datasource that was never written")
	}
}

// The file is the one place the env answer is explained after the fact. A
// value with no comment is one nobody can re-decide six months later.
func TestTheWrittenConfigExplainsWhatEnvDoes(t *testing.T) {
	s := &script{answers: answers(), choices: choices(), password: "secret"}
	w, path := newWizard(t, s)

	if err := w.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	for _, want := range []string{"# ", string(config.EnvProd), string(config.EnvDev)} {
		if !strings.Contains(string(written), want) {
			t.Errorf("the written config does not mention %q:\n%s", want, written)
		}
	}
}

// `dv init` is dispatched before the configuration is read, which puts it out
// of App's reach — and out of its tests' reach with it. HandleVersion is a free
// function for the same reason, and this follows it so that the one command a
// new user types is not the one nothing checks.
func TestHandleInitRecognisesTheCommand(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		wanted  bool
		code    int
		usageIn bool
	}{
		{name: "no arguments", args: nil},
		{name: "another command", args: []string{"ls"}},
		{name: "init", args: []string{"init"}, wanted: true},
		// Silently ignoring the extras would run the wizard for someone who
		// asked for something else and got no sign that they had not.
		{name: "init with an extra", args: []string{"init", "local"}, wanted: true, code: exitUsage, usageIn: true},
		{name: "init with several extras", args: []string{"init", "-c", "x"}, wanted: true, code: exitUsage, usageIn: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer

			wanted, code := HandleInit(&out, tc.args)
			if wanted != tc.wanted {
				t.Errorf("wanted = %v, want %v", wanted, tc.wanted)
			}
			if code != tc.code {
				t.Errorf("code = %d, want %d", code, tc.code)
			}
			if got := strings.Contains(out.String(), "usage: dv init"); got != tc.usageIn {
				t.Errorf("usage printed = %v, want %v (output %q)", got, tc.usageIn, out.String())
			}
		})
	}
}

// A machine with no keychain must still finish setup. Returning an error here
// would discard a connection that was just proved to work and send the user to
// `dv auth`, which needs the very keychain that refused.
func TestTheWizardFinishesWhenTheKeychainRefuses(t *testing.T) {
	s := &script{answers: answers(), choices: choices(), password: "secret"}
	w, path := newWizard(t, s)
	w.Secrets = refusingSecrets{err: errors.New("no secret service")}

	if err := w.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if _, err := os.ReadFile(path); err != nil {
		t.Fatalf("the config was not written: %v", err)
	}

	out := w.Out.(*bytes.Buffer).String()
	// Without the variable's name the user is told the setup failed and given
	// nothing that would make it work.
	if want := secret.EnvVarName("local"); !strings.Contains(out, want) {
		t.Errorf("output does not name %s, so there is no way forward:\n%s", want, out)
	}
	if !strings.Contains(out, path) {
		t.Errorf("output does not say the config was written:\n%s", out)
	}
}

// refusingSecrets is a keychain that is not there, as a headless Linux box
// presents one.
type refusingSecrets struct{ err error }

func (r refusingSecrets) Get(string) (string, error) { return "", r.err }
func (r refusingSecrets) Set(string, string) error   { return r.err }
func (r refusingSecrets) Delete(string) error        { return r.err }

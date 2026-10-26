//go:build windows

package main

import (
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

//go:embed ui/index.html ui/app.css ui/app.js ui/fonts/*.woff2
var ui embed.FS

// The pages a link in the window may open, by name: the page can't ask for
// any other address.
var links = map[string]string{
	"terms":    "https://skillissues.dizziedbliss.lol/terms",
	"privacy":  "https://skillissues.dizziedbliss.lol/privacy",
	"site":     "https://skillissues.dizziedbliss.lol",
	"releases": "https://github.com/" + repo + "/releases/latest",
}

// installer is the window's state: what's installed, the newest release,
// and the operation in progress, if any. The page asks through the bound
// functions; long work runs in goroutines and reports back through events.
type installer struct {
	w webview2.WebView

	mu        sync.Mutex
	latest    *Release
	installed *Installed
	cancel    context.CancelFunc
	busy      bool
}

func main() {
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "SkillIssues Setup",
			Width:  520,
			Height: 700,
			IconId: 1, // the icon go-winres embeds
			Center: true,
		},
	})
	if w == nil {
		noWebView()
		return
	}
	defer w.Destroy()
	w.SetSize(520, 700, webview2.HintFixed)

	in := &installer{w: w}
	in.bind()
	page, err := buildPage()
	if err != nil {
		fatal(err)
		return
	}
	w.SetHtml(page)
	w.Run()
}

// bind gives the page its functions. Each returns at once; results that take
// time arrive as events through si.on(...).
func (in *installer) bind() {
	must := func(err error) {
		if err != nil {
			fatal(err)
		}
	}
	must(in.w.Bind("si_state", in.state))
	must(in.w.Bind("si_install", in.install))
	must(in.w.Bind("si_remove", in.remove))
	must(in.w.Bind("si_cancel", in.cancelDownload))
	must(in.w.Bind("si_launch", in.launch))
	must(in.w.Bind("si_open", in.open))
	must(in.w.Bind("si_quit", func() { in.w.Terminate() }))
	must(in.w.Bind("si_defaultDir", func(perMachine bool) string { return defaultDir(perMachine) }))
}

// state looks up what's installed and the newest release, then sends a
// "state" event. Called once when the page loads, and after Try again.
func (in *installer) state() {
	go func() {
		installed, ierr := findInstalled()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		rel, rerr := FetchLatest(ctx, newClient())
		in.mu.Lock()
		in.installed = installed
		if rerr == nil {
			in.latest = &rel
		}
		in.mu.Unlock()
		ev := map[string]any{"installed": installed, "defaultDir": defaultDir(false)}
		if rerr == nil {
			ev["latest"] = rel
		} else {
			ev["error"] = "Couldn't reach GitHub to find the newest version: " + rerr.Error()
		}
		if ierr != nil {
			ev["error"] = ierr.Error()
		}
		in.emit("state", ev)
	}()
}

// installOptions is what the page sends with Install, Update or Repair.
type installOptions struct {
	PerMachine bool   `json:"perMachine"`
	Dir        string `json:"dir"`
}

// install downloads the newest release, checks it, and runs it silently.
func (in *installer) install(opts installOptions) error {
	in.mu.Lock()
	if in.busy {
		in.mu.Unlock()
		return errors.New("setup is already working")
	}
	if in.latest == nil {
		in.mu.Unlock()
		return errors.New("the newest version isn't known yet")
	}
	rel := *in.latest
	ctx, cancel := context.WithCancel(context.Background())
	in.busy, in.cancel = true, cancel
	in.mu.Unlock()

	go func() {
		defer func() {
			in.mu.Lock()
			in.busy, in.cancel = false, nil
			in.mu.Unlock()
			cancel()
		}()
		if err := in.run(ctx, rel, opts); err != nil {
			if errors.Is(err, context.Canceled) {
				in.emit("cancelled", nil)
				return
			}
			in.emit("error", map[string]string{"message": err.Error()})
			return
		}
		installed, _ := findInstalled()
		in.mu.Lock()
		in.installed = installed
		in.mu.Unlock()
		in.emit("done", map[string]any{"installed": installed})
	}()
	return nil
}

func (in *installer) run(ctx context.Context, rel Release, opts installOptions) error {
	dir, err := os.MkdirTemp("", "skillissues-setup-")
	if err != nil {
		return fmt.Errorf("making a folder for the download: %w", err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, rel.Path)
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("saving the download: %w", err)
	}
	in.emit("phase", map[string]string{"phase": "downloading"})
	sum, err := Download(ctx, newClient(), rel.AssetURL(), rel.Size, f, func(p Progress) { in.emit("progress", p) })
	if cerr := f.Close(); err == nil && cerr != nil {
		err = fmt.Errorf("saving the download: %w", cerr)
	}
	if err != nil {
		return err
	}
	in.emit("phase", map[string]string{"phase": "checking"})
	if err := VerifySHA512(sum, rel.SHA512); err != nil {
		return err
	}
	// Past this point the installer runs to the end: nothing cancels it half done.
	in.emit("phase", map[string]string{"phase": "installing"})
	return runInstaller(path, opts.PerMachine, strings.TrimSpace(opts.Dir))
}

// cancelDownload stops a download; installing can't be stopped half way.
func (in *installer) cancelDownload() {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.cancel != nil {
		in.cancel()
	}
}

// remove runs the installed app's uninstaller silently.
func (in *installer) remove() error {
	in.mu.Lock()
	installed := in.installed
	if in.busy || installed == nil {
		in.mu.Unlock()
		return errors.New("nothing to remove")
	}
	in.busy = true
	in.mu.Unlock()
	go func() {
		defer func() {
			in.mu.Lock()
			in.busy = false
			in.mu.Unlock()
		}()
		in.emit("phase", map[string]string{"phase": "removing"})
		if err := runUninstaller(installed); err != nil {
			in.emit("error", map[string]string{"message": err.Error()})
			return
		}
		in.mu.Lock()
		in.installed = nil
		in.mu.Unlock()
		in.emit("removed", nil)
	}()
	return nil
}

func (in *installer) launch() error {
	in.mu.Lock()
	installed := in.installed
	in.mu.Unlock()
	if err := launch(installed); err != nil {
		return err
	}
	in.w.Terminate()
	return nil
}

// open opens one of the named links in the default browser.
func (in *installer) open(name string) error {
	u, ok := links[name]
	if !ok {
		return fmt.Errorf("no link named %q", name)
	}
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
}

// emit sends an event to the page, on the window's thread.
func (in *installer) emit(name string, data any) {
	b, err := json.Marshal(data)
	if err != nil {
		b = []byte("null")
	}
	js := fmt.Sprintf("window.si && window.si.on(%q, %s)", name, b)
	in.w.Dispatch(func() { in.w.Eval(js) })
}

// buildPage inlines the stylesheet, the script and the fonts into one page,
// so it loads with no file or network access at all.
func buildPage() (string, error) {
	read := func(name string) (string, error) {
		b, err := ui.ReadFile("ui/" + name)
		return string(b), err
	}
	html, err := read("index.html")
	if err != nil {
		return "", err
	}
	css, err := read("app.css")
	if err != nil {
		return "", err
	}
	js, err := read("app.js")
	if err != nil {
		return "", err
	}
	fonts, err := ui.ReadDir("ui/fonts")
	if err != nil {
		return "", err
	}
	for _, f := range fonts {
		b, err := ui.ReadFile("ui/fonts/" + f.Name())
		if err != nil {
			return "", err
		}
		css = strings.ReplaceAll(css, "fonts/"+f.Name(), "data:font/woff2;base64,"+base64.StdEncoding.EncodeToString(b))
	}
	html = strings.Replace(html, `<link rel="stylesheet" href="app.css">`, "<style>"+css+"</style>", 1)
	html = strings.Replace(html, `<script src="app.js"></script>`, "<script>"+js+"</script>", 1)
	return html, nil
}

// noWebView handles a Windows without the WebView2 runtime: it offers the
// full installer instead.
func noWebView() {
	title, _ := windows.UTF16PtrFromString("SkillIssues Setup")
	text, _ := windows.UTF16PtrFromString("This setup needs Microsoft Edge WebView2, which this PC doesn't have.\n\nOpen the releases page to download the full installer instead?")
	const yesNo, yes = 0x00000004 | 0x00000040, 6
	if r, _ := windows.MessageBox(0, text, title, yesNo); r == yes {
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", links["releases"]).Start()
	}
}

func fatal(err error) {
	title, _ := windows.UTF16PtrFromString("SkillIssues Setup")
	text, _ := windows.UTF16PtrFromString("Setup couldn't start: " + err.Error())
	_, _ = windows.MessageBox(0, text, title, 0x00000010)
	os.Exit(1)
}

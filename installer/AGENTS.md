# Windows installer (`SkillIssues-Installer.exe`, its own Go module)

A WebView2 window (`github.com/jchv/go-webview2`, no cgo) showing `ui/`, which downloads the newest
desktop release and runs the NSIS installer silently.

## Workflow

```bash
cd installer
gofmt -l . && go vet ./... && GOOS=windows go vet ./...   # both: main_other.go vs main_windows.go
go test ./...                                             # the platform-independent logic
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o SkillIssues-Installer.exe .   # needs winres first, see ci.yml
```

CI's `installer` job checks it; `release.yml` builds it on the Windows runner.

## Rules

- **Download only from GitHub's release hosts**, and **check the SHA-512 before running anything.** The
  newest version comes from `releases/latest/download/latest.yml` (no API, no rate limit).
- Run the NSIS installer silently with `/S`, `/currentuser` or `/allusers`, and `/D=`.
- The page can open only named links.
- `ui/` has the site's look with fonts inlined: it must work offline from the binary.
- No WebView2: a message box offers the full installer instead.
- An existing install (its Uninstall registry entry) offers Update, Repair or Remove.

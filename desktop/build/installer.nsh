; The Windows installer's welcome page: what SkillIssues is, before the license,
; the install mode and folder pages electron-builder adds (package.json, nsis).

!macro customWelcomePage
  ; Dark, like the app: the welcome and finish pages' text area and the header
  ; strip, behind the sidebar and header images (installerSidebar.bmp,
  ; installerHeader.bmp). Set before the first page, which draws the interface.
  !ifndef MUI_BGCOLOR
    !define MUI_BGCOLOR "0A0A0C"
  !endif
  !ifndef MUI_TEXTCOLOR
    !define MUI_TEXTCOLOR "FFFFFF"
  !endif
  !define MUI_WELCOMEPAGE_TITLE "Welcome to SkillIssues"
  !define MUI_WELCOMEPAGE_TEXT "SkillIssues helps you find open-source issues you can solve, and takes you from reading one to a pull request.$\r$\n$\r$\nIt forks the project, clones your fork, makes a branch for the issue and opens it in VS Code. You commit, push and open the pull request from the app. Your GitHub token for pushing never leaves this computer.$\r$\n$\r$\nNext, you'll see the license and terms, then choose where to install it.$\r$\n$\r$\nClick Next to continue."
  !insertmacro MUI_PAGE_WELCOME
!macroend

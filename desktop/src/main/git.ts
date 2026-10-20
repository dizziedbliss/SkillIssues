import { execFile } from "node:child_process";
import { gitEnv, redact } from "./security";

/*
 * git, run as an async child process so the window never blocks. Arguments
 * go to execFile as a list: no shell parses them. A push token never appears
 * on the command line (any local process can read argv); it reaches git
 * through the environment as an http.extraheader for github.com only, and is
 * redacted from anything git prints back.
 */

const TIMEOUT_MS = 5 * 60_000;
const MAX_OUTPUT = 16 * 1024 * 1024;

export class GitError extends Error {}

/** Turns git's stderr into something a person can act on. */
function explain(stderr: string): string {
  if (/could not read Username|Authentication failed|terminal prompts disabled/i.test(stderr)) {
    return "GitHub refused the credentials. Reconnect GitHub for pushing in Settings.";
  }
  if (/Permission to .* denied|The requested URL returned error: 403/i.test(stderr)) {
    return "GitHub refused the push: this token can't write to that repository.";
  }
  if (/Repository not found|returned error: 404/i.test(stderr)) {
    return "GitHub couldn't find that repository. A new fork can take a few seconds to appear; try again.";
  }
  if (/Please tell me who you are|empty ident/i.test(stderr)) {
    return "git needs your name and email before it can commit: run git config --global user.name and user.email.";
  }
  return stderr.trim().split("\n").slice(-3).join("\n") || "git failed.";
}

/** Runs git in cwd and returns its stdout. Throws GitError with a readable, redacted message. */
export function git(cwd: string, args: string[], token: string | null = null): Promise<string> {
  return new Promise((resolve, reject) => {
    execFile(
      "git",
      args,
      { cwd, env: gitEnv(token), timeout: TIMEOUT_MS, maxBuffer: MAX_OUTPUT, windowsHide: true },
      (error, stdout, stderr) => {
        if (error) {
          const code = (error as NodeJS.ErrnoException).code;
          const text =
            code === "ENOENT"
              ? "git isn't installed or isn't on your PATH. Install it from https://git-scm.com and restart the app."
              : error.killed
                ? `git ${args[0]} took longer than ${TIMEOUT_MS / 60_000} minutes and was stopped.`
                : explain(String(stderr));
          reject(new GitError(redact(text, token)));
          return;
        }
        resolve(redact(String(stdout), token).trim());
      },
    );
  });
}

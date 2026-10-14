import { useEffect, useState } from "react";
import { Modal } from "./Modal";

export interface AttachPullRequestDialogProps {
  open: boolean;
  /** owner/name of the challenge repository; the PR must belong to it. */
  repository: string;
  /**
   * Called with the parsed URL and number. Reject with an Error to show its
   * message in the dialog — the server's reason (wrong author, PR not found)
   * is what the user needs to see.
   */
  onAttach: (prUrl: string, prNumber: number) => Promise<void>;
  onClose: () => void;
}

const PR_URL = /^https:\/\/github\.com\/([A-Za-z0-9_.-]+)\/([A-Za-z0-9_.-]+)\/pull\/(\d+)\/?$/;

/**
 * Parses a GitHub pull request URL for `repository`. Returns the canonical
 * URL (no trailing slash, query or fragment) and number, or an error message.
 */
export function parsePullRequestUrl(
  raw: string,
  repository: string,
): { url: string; number: number } | { error: string } {
  const trimmed = raw.trim().replace(/[?#].*$/, "");
  const match = PR_URL.exec(trimmed);
  if (!match) {
    return { error: "Paste the pull request's address, like https://github.com/owner/repo/pull/123." };
  }
  const [, owner, name, num] = match;
  if (`${owner}/${name}`.toLowerCase() !== repository.toLowerCase()) {
    return { error: `That pull request is on ${owner}/${name}; it must be on ${repository}.` };
  }
  return { url: `https://github.com/${owner}/${name}/pull/${num}`, number: Number(num) };
}

// For a PR the user opened on GitHub themselves — typically from the compare
// page the submit flow falls back to when GitHub refuses to open the PR with
// their authorization. Until it is linked, the contribution cannot be
// verified or credited by the merge webhook.
export function AttachPullRequestDialog({ open, repository, onAttach, onClose }: AttachPullRequestDialogProps) {
  const [value, setValue] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (open) {
      setValue("");
      setError(null);
      setBusy(false);
    }
  }, [open]);

  const submit = async () => {
    const parsed = parsePullRequestUrl(value, repository);
    if ("error" in parsed) {
      setError(parsed.error);
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await onAttach(parsed.url, parsed.number);
      onClose();
    } catch (err) {
      setError(err instanceof Error && err.message ? err.message : "Couldn't link the pull request.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal open={open} onClose={busy ? () => {} : onClose} className="ui-confirm-dialog" labelledBy="ui-attach-pr-title">
      <h2 id="ui-attach-pr-title">Link your pull request</h2>
      <p>
        Opened the pull request on GitHub yourself? Paste its address so SkillIssues can track it and award XP when it
        merges. It must be on <strong>{repository}</strong> and opened from your account.
      </p>
      <form
        className="ui-confirm-dialog-type"
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
      >
        <label htmlFor="ui-attach-pr-url">Pull request URL</label>
        <input
          id="ui-attach-pr-url"
          type="url"
          value={value}
          onChange={(e) => setValue(e.target.value)}
          placeholder={`https://github.com/${repository}/pull/123`}
          autoComplete="off"
          disabled={busy}
          aria-invalid={error !== null}
          aria-describedby={error ? "ui-attach-pr-error" : undefined}
        />
        {error && (
          <p id="ui-attach-pr-error" className="ui-attach-pr-error" role="alert">
            {error}
          </p>
        )}
        <div className="ui-confirm-dialog-actions">
          <button type="button" onClick={onClose} disabled={busy}>
            Cancel
          </button>
          <button type="submit" disabled={busy || value.trim() === ""}>
            {busy ? "Linking…" : "Link pull request"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

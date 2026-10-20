import { useState, type FormEvent } from "react";
import { ApiError, type RepositoryLookup } from "@skillissues/client";
import { api } from "../../lib/api";
import s from "./Filler.module.css";

const numbers = new Intl.NumberFormat("en-US");

/** Plain words for repositories.eligibility_reason (backend ingest.Eligible). */
const reasons: Record<string, string> = {
  below_star_floor: "it has fewer stars than projects we list",
  archived: "it is archived on GitHub",
  excluded: "it is on our exclusion list",
  no_primary_language: "GitHub reports no main language for it",
  no_issues: "it has no issues",
  stale: "it hasn’t been pushed to recently",
  not_found: "GitHub no longer has it",
  duplicate: "it is listed under its current name",
};

function explain(r: RepositoryLookup): string {
  const why = (r.reason ?? "")
    .split(";")
    .map((x) => x.trim())
    .filter(Boolean)
    .map((x) => reasons[x] ?? x.replaceAll("_", " "));
  return why.length ? why.join(", and ") : "it doesn’t meet our listing rules right now";
}

type State =
  | { kind: "idle" }
  | { kind: "busy" }
  | { kind: "done"; result: RepositoryLookup }
  | { kind: "error"; message: string };

/** Next to "For maintainers": is my repository listed, and if not, why. */
export function RepoLookup() {
  const [name, setName] = useState("");
  const [state, setState] = useState<State>({ kind: "idle" });

  async function check(e: FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    setState({ kind: "busy" });
    try {
      setState({ kind: "done", result: await api.lookupRepository(name.trim()) });
    } catch (err) {
      const message =
        err instanceof ApiError && err.status === 400
          ? "Enter it as owner/repo, like facebook/react."
          : err instanceof ApiError
            ? err.message
            : "Couldn’t check right now. Try again in a moment.";
      setState({ kind: "error", message });
    }
  }

  return (
    <form className={s.lookup} onSubmit={check}>
      <label className={s.fieldLabel} htmlFor="repo-lookup">
        Is your project listed?
      </label>
      <div className={s.field}>
        <input
          id="repo-lookup"
          name="repository"
          placeholder="owner/repo"
          autoComplete="off"
          spellCheck={false}
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        <button type="submit" disabled={state.kind === "busy"}>
          {state.kind === "busy" ? "Checking" : "Check"}
        </button>
      </div>
      <p className={s.result} role="status" aria-live="polite">
        {state.kind === "done" && <Answer r={state.result} />}
        {state.kind === "error" && state.message}
      </p>
    </form>
  );
}

function Answer({ r }: { r: RepositoryLookup }) {
  if (!r.found) {
    return (
      <>
        <strong>Not listed yet.</strong> We check repositories as they show issue activity on GitHub, so one that
        qualifies is picked up after its next issue update.
      </>
    );
  }
  if (!r.eligible) {
    return (
      <>
        <strong>{r.full_name} isn’t listed</strong> because {explain(r)}.
      </>
    );
  }
  return (
    <>
      <strong>{r.full_name} is listed.</strong> {numbers.format(r.open_issues)} open{" "}
      {r.open_issues === 1 ? "issue is" : "issues are"} on SkillIssues.
    </>
  );
}

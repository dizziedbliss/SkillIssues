import { useState } from "react";
import { Modal } from "./Modal";

export interface ConfirmDialogProps {
  open: boolean;
  title: string;
  description: string;
  confirmLabel?: string;
  cancelLabel?: string;
  // When set, the confirm button stays disabled until the user types this
  // exact text — used for the account-deletion "danger zone" (D3e), which
  // today is a bare window.confirm() a user can blow through with one
  // reflexive click.
  requireTypedConfirmation?: string;
  danger?: boolean;
  busy?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

export function ConfirmDialog({
  open,
  title,
  description,
  confirmLabel = "Confirm",
  cancelLabel = "Cancel",
  requireTypedConfirmation,
  danger,
  busy,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  const [typed, setTyped] = useState("");

  const confirmDisabled =
    busy || (requireTypedConfirmation !== undefined && typed !== requireTypedConfirmation);

  return (
    <Modal
      open={open}
      onClose={onCancel}
      className="ui-confirm-dialog"
      labelledBy="ui-confirm-dialog-title"
    >
      <h2 id="ui-confirm-dialog-title">{title}</h2>
      <p>{description}</p>
      {requireTypedConfirmation !== undefined && (
        <div className="ui-confirm-dialog-type">
          <label>
            Type <strong>{requireTypedConfirmation}</strong> to confirm
          </label>
          <input
            type="text"
            value={typed}
            onChange={(e) => setTyped(e.target.value)}
            autoComplete="off"
            autoFocus
          />
        </div>
      )}
      <div className="ui-confirm-dialog-actions">
        <button type="button" onClick={onCancel} disabled={busy}>
          {cancelLabel}
        </button>
        <button
          type="button"
          className={danger ? "ui-confirm-dialog-danger" : undefined}
          onClick={onConfirm}
          disabled={confirmDisabled}
        >
          {busy ? "Working…" : confirmLabel}
        </button>
      </div>
    </Modal>
  );
}

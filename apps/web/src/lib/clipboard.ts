import { toast } from "sonner";

function legacyCopy(text: string): boolean {
  if (typeof document === "undefined") return false;
  const ta = document.createElement("textarea");
  ta.value = text;
  ta.setAttribute("readonly", "");
  ta.style.position = "fixed";
  ta.style.top = "0";
  ta.style.left = "0";
  ta.style.opacity = "0";
  ta.style.pointerEvents = "none";
  document.body.appendChild(ta);
  const active = document.activeElement as HTMLElement | null;
  try {
    ta.focus();
    ta.select();
    ta.setSelectionRange(0, text.length);
    return document.execCommand("copy");
  } catch {
    return false;
  } finally {
    document.body.removeChild(ta);
    active?.focus?.();
  }
}

/**
 * Copies `text` to the clipboard. Tries the async Clipboard API first and falls back to
 * `document.execCommand("copy")`. Never throws — resolves `true` when the text was copied.
 *
 * Call it from a user gesture (click handler) whenever possible: browsers reject
 * `navigator.clipboard.writeText` when the document is not focused (e.g. right after a
 * native dialog) or outside a user activation.
 */
export async function copyToClipboard(text: string): Promise<boolean> {
  if (typeof window === "undefined" || !text) return false;
  const canUseAsync = !!navigator.clipboard?.writeText && (typeof document.hasFocus !== "function" || document.hasFocus());
  if (canUseAsync) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // fall through to the legacy path
    }
  }
  return legacyCopy(text);
}

/** Copies and reports the outcome with a toast. Returns whether the copy succeeded. */
export async function copyWithToast(text: string, successMessage = "Copiado"): Promise<boolean> {
  const ok = await copyToClipboard(text);
  if (ok) toast.success(successMessage);
  else toast.error("Não foi possível copiar automaticamente. Selecione o texto e copie manualmente.");
  return ok;
}

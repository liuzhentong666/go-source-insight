const TOKEN_KEY = 'gosource.session.token';
const JOB_PREFIX = 'gosource.jobs.';
let token: string | null = null;
let generation = 0;
const listeners = new Set<() => void>();

export function storage(): Storage | undefined {
  try { return globalThis.sessionStorage; } catch { return undefined; }
}
export function readStoredToken(): string | null {
  try {
    globalThis.localStorage?.removeItem('token');
    globalThis.localStorage?.removeItem('user');
  } catch { /* Legacy storage may be blocked separately from session storage. */ }
  try { return storage()?.getItem(TOKEN_KEY) || null; } catch { return null; }
}
export function getSessionToken() { return token; }
export function getSessionGeneration() { return generation; }
export function setSessionToken(value: string | null) {
  token = value;
  try {
    if (value) storage()?.setItem(TOKEN_KEY, value);
    else storage()?.removeItem(TOKEN_KEY);
  } catch { /* Memory sessions still work if browser storage is unavailable. */ }
}
export function onSessionReset(listener: () => void) {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
}
export function resetSession(clearJobs = true) {
  generation++;
  setSessionToken(null);
  if (clearJobs) {
    try {
      const target = storage();
      for (let i = (target?.length || 0) - 1; i >= 0; i--) {
        const key = target?.key(i);
        if (key?.startsWith(JOB_PREFIX)) target?.removeItem(key);
      }
    } catch { /* Storage may be disabled. */ }
  }
  listeners.forEach(listener => listener());
}

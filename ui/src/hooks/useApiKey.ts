import { useState, useCallback } from "react";

const STORAGE_KEY = "evidra_api_key";
const EPHEMERAL_KEY = "evidra_ephemeral";

function readKey(): string | null {
  return localStorage.getItem(STORAGE_KEY) ?? sessionStorage.getItem(STORAGE_KEY);
}

function readEphemeral(): boolean {
  return localStorage.getItem(EPHEMERAL_KEY) === "true";
}

export function useApiKey() {
  const [apiKey, setApiKeyState] = useState<string | null>(readKey);
  const [ephemeral, setEphemeralState] = useState(readEphemeral);

  const setApiKey = useCallback(
    (key: string) => {
      if (ephemeral) {
        sessionStorage.setItem(STORAGE_KEY, key);
        localStorage.removeItem(STORAGE_KEY);
      } else {
        localStorage.setItem(STORAGE_KEY, key);
        sessionStorage.removeItem(STORAGE_KEY);
      }
      setApiKeyState(key);
    },
    [ephemeral],
  );

  const clearApiKey = useCallback(() => {
    localStorage.removeItem(STORAGE_KEY);
    sessionStorage.removeItem(STORAGE_KEY);
    setApiKeyState(null);
  }, []);

  const setEphemeral = useCallback(
    (value: boolean) => {
      setEphemeralState(value);
      localStorage.setItem(EPHEMERAL_KEY, String(value));

      // Migrate existing key to the correct storage
      const currentKey = readKey();
      if (currentKey) {
        if (value) {
          sessionStorage.setItem(STORAGE_KEY, currentKey);
          localStorage.removeItem(STORAGE_KEY);
        } else {
          localStorage.setItem(STORAGE_KEY, currentKey);
          sessionStorage.removeItem(STORAGE_KEY);
        }
      }
    },
    [],
  );

  return { apiKey, ephemeral, setApiKey, clearApiKey, setEphemeral };
}

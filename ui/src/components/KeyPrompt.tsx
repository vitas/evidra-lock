import { useState } from "react";

interface KeyPromptProps {
  onSubmit: (key: string) => void;
  ephemeral: boolean;
  onEphemeralChange: (value: boolean) => void;
}

export function KeyPrompt({ onSubmit, ephemeral, onEphemeralChange }: KeyPromptProps) {
  const [value, setValue] = useState("");

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (value.trim()) {
      onSubmit(value.trim());
    }
  };

  return (
    <form className="key-prompt" onSubmit={handleSubmit}>
      <div className="key-prompt-field">
        <input
          type="text"
          placeholder="Paste your API key"
          value={value}
          onChange={(e) => setValue(e.target.value)}
        />
        <button type="submit">Connect</button>
      </div>
      <label className="key-prompt-ephemeral">
        <input
          type="checkbox"
          checked={ephemeral}
          onChange={(e) => onEphemeralChange(e.target.checked)}
        />
        Forget key on tab close
      </label>
      <p className="key-prompt-warning">
        API key stored in browser storage. Do not use on shared computers.
      </p>
    </form>
  );
}

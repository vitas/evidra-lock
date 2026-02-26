import { useState } from "react";

interface JsonEditorProps {
  value: string;
  onChange: (parsed: Record<string, unknown> | null, valid: boolean) => void;
}

export function JsonEditor({ value, onChange }: JsonEditorProps) {
  const [text, setText] = useState(value);
  const [status, setStatus] = useState<"valid" | "invalid" | null>(null);

  const handleBlur = () => {
    try {
      const parsed = JSON.parse(text);
      setStatus("valid");
      onChange(parsed, true);
    } catch {
      setStatus("invalid");
      onChange(null, false);
    }
  };

  return (
    <div className="json-editor">
      <textarea
        className="json-editor-textarea"
        value={text}
        onChange={(e) => setText(e.target.value)}
        onBlur={handleBlur}
        spellCheck={false}
      />
      {status && (
        <span className={`json-editor-status json-editor-status--${status}`}>
          {status === "valid" ? "Valid JSON" : "Invalid JSON"}
        </span>
      )}
    </div>
  );
}

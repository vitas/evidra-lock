import { CopyButton } from "./CopyButton";

interface CodeBlockProps {
  code: string;
  copyable?: boolean;
}

export function CodeBlock({ code, copyable = true }: CodeBlockProps) {
  return (
    <div className="code-block">
      <pre>
        <code>{code}</code>
      </pre>
      {copyable && (
        <div className="code-block-actions">
          <CopyButton text={code} />
        </div>
      )}
    </div>
  );
}

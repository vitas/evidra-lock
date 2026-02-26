interface InlineErrorProps {
  message: string;
  onRetry?: () => void;
  action?: {
    label: string;
    onClick: () => void;
  };
}

export function InlineError({ message, onRetry, action }: InlineErrorProps) {
  return (
    <div className="inline-error">
      <span className="inline-error-msg">{message}</span>
      <div className="inline-error-actions">
        {action && (
          <button
            type="button"
            className="inline-error-btn"
            onClick={action.onClick}
          >
            {action.label}
          </button>
        )}
        {onRetry && (
          <button type="button" className="inline-error-btn" onClick={onRetry}>
            Retry
          </button>
        )}
      </div>
    </div>
  );
}
